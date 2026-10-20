# CLAUDE.md

## 架构声明

本项目采用**经典分层架构**：依赖只能自上而下单向流动，层是封闭的，不能跳层。组装层（`myapp.bootstrap`、`myapp.main`）是组装根。

```
组装 ──> entry ──> service ──> infra ──> common
  └────────┴──────────┴──────────┴─────> common
```

**组装层（`myapp.bootstrap`、`myapp.main`）可以依赖所有层**：它只做接线，是唯一同时知道所有具体实现的地方；任何层都不能反过来依赖组装层。

common 是共享内核，各层依赖 common 不算跳层；除此之外，entry 不能直接依赖 infra。

**这不是 Clean 架构、六边形架构或依赖倒置。** service 直接 import infra 的具体类。未经用户明确同意，不要：

- 在 service 定义端口 Protocol、再由 infra 实现；
- 把 infra 的类抽成接口层；
- 把分层改造成洋葱或整洁架构。

单测可以用 fake 对象或 monkeypatch 替换 infra 的具体类，不要为了测试在生产代码里引入端口层。

## 架构工作方式

- 跨层改动时，先用几行说明计划：改哪些文件、每项落在哪一层、哪个包，是否涉及「必须停下来问」的事项，再动手。

## 架构硬性约束

本节只保留与架构相关的约束；通用约束见根 `CLAUDE.md`。

1. **先定位再写**：动手前确定代码属于哪一层、哪个包。落不进「本项目的层级」表就停下来问，不要自创包。
2. **必须停下来问**：层级归属不明；新增包或层。

## 分层与依赖

### 层的角色

每一层必须归入以下某种角色。可以按业务拆子包，但不存在不属于任何角色的层。

| 角色 | 职责 | 典型内容 |
|---|---|---|
| 组装 | 创建对象、注入依赖、读取配置、启停任务，只做接线 | `myapp/bootstrap.py`、`myapp/main.py` |
| 入口 | 接收外部请求或定时触发，转换和校验后调用业务层 | FastAPI/Flask 路由、MQ 消费者、定时任务、CLI |
| 业务 | 业务规则与流程编排 | 计算、决策、状态流转、结果处理、事务边界 |
| 基础设施 | 技术细节 | 第三方客户端、WebSocket 客户端、ORM 模型与仓储、Redis、Kafka、文件、缓存加载 |
| 公共 | 各层共用的只读类型与纯工具，不依赖任何层 | 跨层 dataclass、Enum、常量、异常基类、纯函数工具 |

### 本项目的层级

> 下表为示例，按实际项目修改。在已有包下新增子包不需要改。

| 层 | 包 | 可以依赖 | 外部依赖限制 |
|---|---|---|---|
| 组装 | `myapp.bootstrap`、`myapp.main` | 所有层 | 无 |
| entry | `myapp.entry`：`entry.api`（HTTP 路由）、`entry.consumer`（MQ 消费）、`entry.jobs`（定时任务） | service、common | 不直接用 sqlalchemy、redis |
| service | `myapp.service` | infra、common | 不直接用 sqlalchemy、redis、HTTP/WS 客户端、Kafka 客户端 |
| infra | `myapp.infra`（如 `infra.db`、`infra.payment`、`infra.ws`） | common | 无 |
| common | `myapp.common` | 无 | 只允许标准库和基础工具库 |

- 所有模块都必须属于上表某一层；`myapp/__init__.py` 只做包声明，不放逻辑。
- 依赖图不能有环，包括函数内部延迟 import 形成的环。

### 依赖规则

- 上层可以依赖下层，**下层禁止 import 上层任何模块**。Python 不会阻止反向 import，写代码时自行保证。
- 禁止用以下方式绕过依赖规则：函数内部延迟 import 上层模块；`importlib.import_module` 动态导入；模块级全局变量或注册表保存上层对象。
- 所有依赖在组装层通过构造函数（`__init__` 参数）传入。FastAPI 的 `Depends` 只在 entry 使用，不要渗透到 service 和 infra。服务实例由组装层在启动时挂到 `app.state` 上（如 `app.state.order_service = OrderService(...)`），entry 的依赖函数从 `request.app.state` 取出；entry 不得 import 组装层。
- 业务层不直接使用中间件或 SDK 的 API（SQLAlchemy session、redis、aiokafka/confluent_kafka、httpx/requests、websockets），必须通过 infra 封装的类访问。
- 基础设施层只处理技术问题（协议、签名、序列化、连接、重连、持久化），不做业务判断。
- 入口层只做参数解析、校验和调用，不写业务逻辑，不直接调用 infra。定时任务放 `entry.jobs`，只负责触发并调用 service。
- 公共层不依赖网络、数据库、IO 相关库，保持纯数据和纯函数。

### 新代码放哪里

- 新的外部系统接入 → infra，协议和报文转换在该层完成。
- 新的业务规则或流程 → service；计算尽量写成不依赖 IO 和事件循环的纯函数。
- 新的 HTTP 接口、消息消费或定时任务 → entry（`entry.api`、`entry.consumer`、`entry.jobs`），只做转换和调用。
- 多层共用的只读数据 → common；只在一层内使用的类型留在该层，不要提前上移。

## 跨边界数据与下层通知上层

### 类型归属

- common 里跨层传递的数据用 `@dataclass(frozen=True, slots=True)`，不可变。
- ORM 模型、外部系统原始报文 dict、SDK 对象只在 infra 内使用，不得进入 common；返回上层前转换为 dataclass 或 service 需要的类型。
- pydantic 请求/响应模型只在 entry 使用，不传进 service；service 的参数和返回值使用 dataclass 或普通类型。

### 方式一：回调 Protocol 或 Callable（单次结果、请求-回报型）

1. `typing.Protocol` 定义在下层，与调用它的类放在一起；签名只用下层或 common 的类型。
2. 上层的类实现对应方法，不需要继承 Protocol。
3. 组装层先创建回调的实现，再通过构造函数交给下层。

```python
# myapp/infra/payment/client.py：下层定义回调 Protocol
class PaymentResultListener(Protocol):
    async def on_payment_result(self, event: PaymentResultEvent) -> None: ...

class PaymentClient:
    def __init__(self, http: httpx.AsyncClient, listener: PaymentResultListener) -> None:
        self._http, self._listener = http, listener

    async def pay(self, order_no: str, amount: Decimal) -> None: ...  # 结果异步到达后调用 listener

# myapp/service/order.py：发起支付，只持有 PaymentClient
class OrderService:
    def __init__(self, payment: PaymentClient) -> None:
        self._payment = payment

# myapp/service/payment_result.py：单独的对象实现回调（service -> infra，方向合法），不持有 PaymentClient
class PaymentResultHandler:
    def __init__(self, orders: OrderRepository) -> None:
        self._orders = orders

    async def on_payment_result(self, event: PaymentResultEvent) -> None: ...  # 更新订单状态

# myapp/bootstrap.py：先建回调实现，再建持有它的客户端
handler = PaymentResultHandler(orders)
payment = PaymentClient(http, listener=handler)
order_service = OrderService(payment)
```

- **实现回调的对象不能再持有那个下层客户端**：上例若让 `OrderService` 同时实现 `on_payment_result`，构造 `PaymentClient` 需要 `OrderService`，构造 `OrderService` 又需要 `PaymentClient`，只能先构造一个，再用 `client.set_listener(service)` 事后回填。事后 setter 会让对象存在半初始化状态，这本身就是坏味道。包依赖是合法的，但装配时会成环。
- 只有一个方法时可用 `Callable[[PaymentResultEvent], Awaitable[None]]`。只有一个调用方且无需解耦时，直接调用下层，不要为形式加 Protocol。

### 方式二：队列（持续推送、流式数据）

- 事件类型（`@dataclass(frozen=True)`）定义在下层或 common。
- 下层持有队列，只负责 `put` / `put_nowait`；上层在自己的任务中消费。队列在组装层创建（如 `asyncio.Queue(maxsize=10_000)`），两端分别注入。

### 跨边界约束

- Protocol、Callable、事件类型的签名只能使用下层或 common 的类型，不能出现上层类型。
- 事件携带转换后的类型，不传原始报文 dict 或 SDK 对象；参数多时封装成事件 dataclass。
- 回调运行在 WS/MQ 接收循环中时，耗时处理必须交给其他任务，不能阻塞接收。

## AI 常见劣化模式

| 劣化写法 | 正确做法 |
|---|---|
| service 直接 import sqlalchemy/redis/httpx | 通过 infra 封装访问 |
| 把类型挪到 common 让两层都能用 | 类型归真正拥有它的层，边界做转换 |
| 单实现也建 Protocol + Factory + ABC，或加 `if settings.use_new` 兼容层 | 直接用具体类，保持单一路径 |
| 同一个对象既实现下层回调，又持有该下层客户端，靠 `set_listener` 事后回填 | 回调拆到单独的对象 |
| 循环里逐条 `await` 查库、靠懒加载取关联、列表不分页 | 批量查询、显式加载，分页并设上限 |
| `except Exception: logger.exception(...)` 后继续 | 转换后抛出，或明确重试、死信、熔断 |
| async 里 `time.sleep`、`requests`、同步 DB 驱动 | `asyncio.sleep`、异步客户端、`to_thread` |
| `create_task` 不保存引用、吞掉 `CancelledError` | `TaskGroup` 管理，取消向上传播 |
| 路由函数里写业务逻辑，`Depends` 渗进 service | 入口只转换和调用，依赖在组装层注入 |
| 改断言、加 skip，或只跑单个测试就宣称通过 | 修代码，跑完整检查并贴输出 |

## 架构自查

- 代码在正确的层和包，依赖方向符合本文件；entry 不直接使用 SQLAlchemy、Redis、HTTP/WS 客户端。
- common 只用标准库和基础工具库。
