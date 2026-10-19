# CLAUDE.md

## 应用架构：分层架构

本项目采用分层架构，依赖只能自上而下单向流动。层级和包名根据业务需要定义，但必须遵守下面的依赖规则。新增或修改代码前，先确认代码属于哪一层。

### 层的角色

每一层都必须归入以下某种角色。可以按业务拆出更多层或子包，但不能出现不属于任何角色的层。

| 角色 | 职责 | 典型内容 |
|---|---|---|
| 入口 | 接收外部触发，转换后调用业务层 | FastAPI/Flask 路由、WS 推送接收、MQ 消费、定时任务、CLI |
| 业务 | 业务规则与流程编排 | 计算、决策、状态流转、结果处理、事务边界 |
| 基础设施 | 技术细节 | 交易所/第三方客户端、ORM 模型与仓储、Redis、Kafka、文件、缓存加载 |
| 公共 | 各层共用的类型与工具，不依赖任何层 | 通用 dataclass、Enum、常量、异常基类、工具函数 |
| 组装 | 创建对象、注入依赖、启动和关闭任务，只做接线 | `main.py`、`bootstrap.py` |

### 本项目的层级

> 新增层、拆分包时，同步更新本表、依赖图和 import-linter 配置。下表为示例，按实际项目修改。

| 层 | 包 | 角色 | 可以依赖 |
|---|---|---|---|
| bootstrap | `myapp.bootstrap`、`myapp.main` | 组装 | 所有层 |
| entry | `myapp.entry`（如 `entry.api`、`entry.ws`、`entry.jobs`） | 入口 | service、common |
| service | `myapp.service` | 业务 | infra、common |
| infra | `myapp.infra` | 基础设施 | common |
| common | `myapp.common` | 公共 | 无（只允许标准库和基础工具库） |

```
bootstrap ──> entry ──> service ──> infra ──> common
    └───────────┴──────────┴────────────────> common
```

### 依赖规则

- 上层可以依赖下层，**下层禁止 import 上层的任何模块**。Python 不会在编译期阻止，必须用 import-linter 检查（见「检查方式」）。
- 依赖图中不能有环。新增层时，先确定它在图中的位置，再写代码。
- 禁止用以下方式绕过依赖规则：
  - 在函数内部延迟 import 上层模块（`def f(): from myapp.service import x`）来规避循环 import。
  - 用模块级全局变量或注册表保存上层对象，再由下层取出调用。
  - `importlib.import_module` 动态导入上层模块。
- 所有依赖在组装处通过构造函数（`__init__` 参数）传入。FastAPI 的 `Depends` 只在入口层使用，不要渗透到业务层和基础设施层。
- 业务层不直接使用中间件或 SDK 的 API（`sqlalchemy` session、`redis`、`aiokafka`/`confluent_kafka`、`httpx`/`requests`、`websockets` 等），必须通过基础设施层封装的类访问。
- 基础设施层只处理技术问题（协议、签名、序列化、连接、重连、持久化），不做业务判断。
- 入口层只做参数解析、校验和调用，不写业务逻辑，不直接调用基础设施层。
- 公共层不依赖网络、数据库、IO 相关的库，保持纯数据和纯函数。
- ORM 模型只在基础设施层内使用，返回给上层前转换为 dataclass 或 pydantic 模型；pydantic 的请求/响应模型只在入口层使用。
- 跨层的公开函数和类必须写完整的类型注解。

### 下层通知上层：下层定义 Protocol 或队列，上层实现/消费

当下层需要把结果交给业务层（下单回报、WS 推送、MQ 消息等）时，**不能直接调用上层**。按以下两种方式之一处理。

#### 方式一：回调 Protocol 或 Callable（单次结果、请求-回报型）

1. `typing.Protocol` 定义在下层，与调用它的类放在一起。
2. 上层的类实现对应方法。Protocol 是结构化类型，上层不需要继承它。
3. 组装处把实现通过构造函数传给下层。

```python
# myapp/infra/order/client.py：下层定义 Protocol，签名只用 infra 或 common 中的类型
class OrderResultListener(Protocol):
    def on_order_result(self, event: OrderResultEvent) -> None: ...


class OrderClient:
    def __init__(self, listener: OrderResultListener) -> None:
        self._listener = listener


# myapp/service/hedge.py：上层实现下层 Protocol（service -> infra，方向合法）
class HedgeService:
    def on_order_result(self, event: OrderResultEvent) -> None: ...


# myapp/bootstrap.py：组装
hedge = HedgeService(...)
client = OrderClient(listener=hedge)
```

只有一个方法时，也可以使用 `Callable[[OrderResultEvent], None]`。异步回调使用 `async def`，Protocol 中同样声明为 `async def`。

#### 方式二：队列（持续推送、流式数据，优先使用）

1. 事件类型（`@dataclass(frozen=True)`）定义在下层或 common。
2. 下层持有队列，只负责 `put`。
3. 上层在自己的任务或线程中消费。
4. 队列在组装处创建，分别传给下层和上层。

```python
# myapp/bootstrap.py：组装（asyncio 场景）
async def main() -> None:
    events: asyncio.Queue[TradeEvent] = asyncio.Queue(maxsize=10_000)
    ws_client = TradeWsClient(config.ws, events)
    hedge = HedgeService(...)

    async with asyncio.TaskGroup() as tg:
        tg.create_task(ws_client.run())
        tg.create_task(hedge.consume(events))
```

多线程场景使用 `queue.Queue`，用法相同。

#### 约束

- Protocol、Callable 和事件类型的签名只能使用下层或 common 中的类型，不能出现上层类型。
- 事件携带的是转换后的类型，不能直接传递交易所原始报文 dict 或 SDK 对象。
- 参数较多时封装成事件 dataclass，不要堆叠参数。
- 回调运行在 WS 接收循环中时，耗时处理必须交给其他任务，不能阻塞接收。
- asyncio 代码中禁止调用阻塞操作（`time.sleep`、`requests`、同步数据库驱动、大量 CPU 计算）；必要时使用 `asyncio.to_thread` 或进程池。
- 有界队列需要明确满了以后的策略（等待、`put_nowait` 失败后丢弃、告警），并写在代码注释中。
- 所有后台任务都必须能被取消并正确退出，组装处负责等待它们结束。
- 只有一个调用方且无需解耦时，上层直接调用下层即可，不要为了形式增加 Protocol。

### 异常与事务

- 每层定义自己的异常类，下层的第三方异常（`httpx.HTTPError`、`sqlalchemy.exc.SQLAlchemyError`、`redis.RedisError` 等）在本层内转换，使用 `raise XxxError(...) from e` 保留异常链，不直接抛给上层。
- 业务异常定义在业务层或 common，入口层统一转换为响应。
- 事务边界（session 的开启和提交）由业务层通过基础设施层提供的接口（如 unit of work）控制，入口层不管理事务。
- 不要使用裸 `except:` 或吞掉异常；必须捕获时记录日志并带上 trace_id 等上下文。

### 新代码放哪里

- 新的外部系统接入（交易所、第三方 API、存储）→ 基础设施层，协议和报文转换在该层内完成。
- 新的业务规则或流程 → 业务层；计算逻辑尽量写成不依赖 IO 和事件循环的纯函数，并配 pytest 单元测试。
- 新的触发方式（新接口、新定时任务、新消费者、新的 WS 推送接收）→ 入口层，只做转换和调用。
- 多个层都用到的类型 → common；只在一层内使用的类型留在该层，不要提前放到 common。
- 交易所报文模型与业务类型分开定义和存放。
- 金额、价格、数量使用 `decimal.Decimal`，不使用 `float`。
- 不属于现有任何层的代码：先在「本项目的层级」表中新增层、明确角色和依赖，再写代码。

### 检查方式

- 使用 import-linter 检查层间依赖（包名按「本项目的层级」表修改）：

```toml
# pyproject.toml
[tool.importlinter]
root_package = "myapp"

[[tool.importlinter.contracts]]
name = "分层依赖：下层不能依赖上层"
type = "layers"
layers = [
    "myapp.bootstrap",
    "myapp.entry",
    "myapp.service",
    "myapp.infra",
    "myapp.common",
]

[[tool.importlinter.contracts]]
name = "入口层不能直接依赖基础设施层"
type = "forbidden"
source_modules = ["myapp.entry"]
forbidden_modules = ["myapp.infra"]
```

- 运行 `lint-imports` 检查依赖；函数内部的延迟 import 同样会被检查到。
- 提交前运行 `ruff check`、`mypy`（或 `pyright`）、`lint-imports` 和 `pytest`。

### 已知的历史违规

> 记录现存不符合上述规则的代码，避免被模仿；修改相关代码时顺手修正。没有则保持为空。

- （无）
