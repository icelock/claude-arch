# CLAUDE.md

## 架构声明

本项目采用**经典分层架构**：依赖只能自上而下单向流动，层是封闭的，不能跳层。`bootstrap` 是组装根。

```
bootstrap ──> entry ──> service ──> infra ──> common
    └───────────┴──────────┴────────────────> common
```

**这不是 Clean 架构、六边形架构或依赖倒置。** service 直接 import infra 的具体模块。未经用户明确同意，不要：

- 在 service 定义端口 Protocol、再由 infra 实现；
- 把 infra 的类抽成接口层；
- 把分层改造成洋葱或整洁架构。

单测可以用 fake 对象或 monkeypatch 替换 infra 的具体类，不要为了测试在生产代码里引入端口层。

## 对 AI 的硬性约束

本节是 MUST，优先于你的默认习惯和所谓的「最佳实践」。

1. **先定位再写**：动手前确定代码属于哪一层、哪个包。落不进「本项目的层级」表就停下来问，不要自创包。
2. **先读后写**：找同层 1–2 个现有实现，沿用其结构、命名、异常处理和日志方式。不要引入仓库里没有的新风格。
3. **最小改动**：只改与需求相关的代码。不顺手重构、不重排 import、不升级依赖、不改 ruff/format 配置。
4. **抽象准入**：没有第二个实现、不属于真实 IO 边界时，不建 Protocol、ABC、工厂、泛型仓储、兼容层、fallback、feature flag。三行重复优于一个错误抽象。
5. **依赖准入**：不新增第三方库。确需新增时，先说明用途、体积、维护状态，等用户确认后再改 `pyproject.toml`。
6. **不写占位**：不留 TODO、mock 返回值、假数据、注释掉的代码、未使用的参数或字段。
7. **验证真实**：完成前运行本节列出的全部检查，贴出命令和真实输出。失败就说失败并继续修，禁止改断言、加 skip、放宽超时、更新快照或放宽 import-linter 规则来让结果变绿。
8. **必须停下来问**：层级归属不明；新增包或层；新增依赖；修改公共类型、对外接口、表结构、消息格式；需要放宽架构检查；修复历史违规会显著扩大改动范围。

## 分层与依赖

### 层的角色

每一层必须归入以下某种角色。可以按业务拆子包，但不存在不属于任何角色的层。

| 角色 | 职责 | 典型内容 |
|---|---|---|
| 入口 | 接收外部触发，转换后调用业务层 | FastAPI/Flask 路由、WS 推送接收、MQ 消费、定时任务、CLI |
| 业务 | 业务规则与流程编排 | 计算、决策、状态流转、结果处理、事务边界 |
| 基础设施 | 技术细节 | 交易所/第三方客户端、ORM 模型与仓储、Redis、Kafka、文件、缓存加载 |
| 公共 | 各层共用的只读类型与纯工具，不依赖任何层 | dataclass、Enum、常量、异常基类、纯函数工具 |
| 组装 | 创建对象、注入依赖、启停任务，只做接线 | `myapp/bootstrap.py`、`myapp/main.py` |

### 本项目的层级

> 下表为示例，按实际项目修改。新增层、拆分包时，同步更新本表、依赖图和 import-linter 配置。

| 层 | 包 | 可以依赖 | 外部依赖限制 |
|---|---|---|---|
| bootstrap | `myapp.bootstrap`、`myapp.main` | 所有层 | 无 |
| entry | `myapp.entry`（如 `entry.api`、`entry.ws`、`entry.jobs`） | service、common | 不直接用 SQLAlchemy、redis、kafka 客户端 |
| service | `myapp.service` | infra、common | 禁 sqlalchemy、redis、httpx、requests、websockets、aiokafka |
| infra | `myapp.infra` | common | 无 |
| common | `myapp.common` | 无 | 只允许标准库和基础工具库 |

- 单模块多包，下层包不得 import 上层模块，由 import-linter 保证。
- 依赖图不能有环，包括函数内部延迟 import 形成的环。

### 依赖规则

- 上层可以依赖下层，**下层禁止 import 上层任何模块**。Python 不会在编译期阻止，必须靠 import-linter 检查。
- 禁止用以下方式绕过依赖规则：函数内部延迟 import 上层模块；`importlib.import_module` 动态导入；模块级全局变量或注册表保存上层对象。
- 所有依赖在 bootstrap 通过构造函数（`__init__` 参数）传入。FastAPI 的 `Depends` 只在 entry 使用，不要渗透到 service 和 infra。
- 业务层不直接使用中间件或 SDK 的 API（SQLAlchemy session、redis、aiokafka/confluent_kafka、httpx/requests、websockets），必须通过 infra 封装的类访问。
- 基础设施层只处理技术问题（协议、签名、序列化、连接、重连、持久化），不做业务判断。
- 入口层只做参数解析、校验和调用，不写业务逻辑，不直接调用 infra。
- 公共层不依赖网络、数据库、IO 相关库，保持纯数据和纯函数。
- ORM 模型只在 infra 内使用；pydantic 请求/响应模型只在 entry 使用；两者都不得进入 service。

### 新代码放哪里

- 新的外部系统接入 → infra，协议和报文转换在该层完成。
- 新的业务规则或流程 → service；计算尽量写成不依赖 IO 和事件循环的纯函数。
- 新的触发方式 → entry，只做转换和调用。
- 多层共用的只读类型 → common；只在一层内使用的类型留在该层，不要提前上移。
- 交易所报文、ORM 模型、SDK 类型不得离开 infra；返回上层前转换为业务类型。

## 跨边界数据与下层通知上层

### 类型归属

- ORM 模型、交易所原始报文 dict、SDK 对象只在 infra 内使用；返回前转换为 dataclass 或 service 需要的类型。
- pydantic 模型只在 entry 使用，不传进 service；service 的参数和返回值使用领域类型或 dataclass。
- common 只放跨层共用的只读类型、Enum、常量、异常基类、纯函数工具；禁止放 pydantic 模型、ORM 模型、请求/响应 DTO。

### 方式一：回调 Protocol 或 Callable（单次结果、请求-回报型）

1. `typing.Protocol` 定义在下层，与调用它的类放在一起；签名只用下层或 common 的类型。
2. 上层的类实现对应方法，不需要继承 Protocol。
3. 组装处把实现通过构造函数传给下层。

```python
# myapp/infra/order/client.py：下层定义 Protocol
class OrderResultListener(Protocol):
    def on_order_result(self, event: OrderResultEvent) -> None: ...

class OrderClient:
    def __init__(self, listener: OrderResultListener) -> None:
        self._listener = listener

# myapp/service/hedge.py：上层实现（service -> infra，方向合法）
class HedgeService:
    def on_order_result(self, event: OrderResultEvent) -> None: ...
```

只有一个方法时可用 `Callable[[OrderResultEvent], None]`；异步回调用 `async def`，Protocol 中同样声明为 `async def`。只有一个调用方且无需解耦时，直接调用下层。

### 方式二：队列（持续推送、流式数据）

1. 事件类型（`@dataclass(frozen=True)`）定义在下层或 common。
2. 下层持有队列，只负责 `put_nowait`；上层在自己的任务中消费。
3. 队列在 bootstrap 创建（如 `asyncio.Queue(maxsize=10_000)`），两端分别注入。
4. 有界队列必须明确满了的策略（等待、丢弃、告警）；订单/回报/成交类不能丢，满了应拒绝受理或熔断。

### 跨边界约束

- Protocol、Callable、事件类型的签名只能使用下层或 common 的类型，不能出现上层类型。
- 事件携带转换后的类型，不传交易所原始报文 dict 或 SDK 对象；参数多时封装成事件 dataclass。
- 回调运行在 WS 接收循环中时，耗时处理必须交给其他任务，不能阻塞接收。
- 队列满、丢弃、延迟、重连次数必须有指标或限频日志。

## 数据、时间与一致性

- 价格、数量、金额统一使用 `decimal.Decimal`，禁止 `float`；同一模块不得混用两种表示。
- 从字符串或整数构造：`Decimal("0.1")`，禁止 `Decimal(0.1)`；序列化为字符串，不用 float。
- 舍入必须显式：`quantize(Decimal("0.01"), rounding=ROUND_HALF_UP)`；不要修改全局 decimal context，也不要依赖默认舍入。
- 入库前统一 scale；同一字段在不同路径使用不同 scale 视为错误。
- 下游写入使用确定性幂等键，数据库加唯一约束；重复消息不能产生重复副作用。
- 消息流必须处理重复、乱序、gap、断线重连：gap 时作废本地缓存并重拉快照，不能跳过继续用；重连后必须重订阅。
- 业务代码不直接调用 `datetime.now()`/`datetime.utcnow()`：注入时钟或把 `now` 作为参数；时间带 UTC 时区；延迟和超时用 `time.monotonic()`。

## 并发与性能

- asyncio 代码中禁止阻塞调用：`time.sleep`、`requests`、同步数据库驱动、大量 CPU 计算；用 `asyncio.sleep`、`httpx.AsyncClient`、`asyncio.to_thread` 或进程池。
- 后台任务必须保存引用或用 `asyncio.TaskGroup()` 管理；禁止 `create_task` 后丢弃引用；所有任务必须能取消并正确退出。
- 禁止吞掉 `asyncio.CancelledError`；清理逻辑放 `finally`，取消要继续向上传播。
- 优先使用有界 `asyncio.Queue(maxsize=...)`；行情类可以丢弃并置 gap 标记，订单、回报、成交类不能丢。
- 线程场景用有界 `ThreadPoolExecutor`，不要创建无界线程；线程池必须有名字和关闭流程。
- async 代码用 `asyncio.Lock`，不要在事件循环里用阻塞的 `threading.Lock`；锁内不做 IO。
- 热路径禁止逐笔日志、`json.dumps`、f-string 拼接、反射/inspect、无界重试。
- 多线程共享状态用锁保护或改为消息传递；禁止模块级可变字典作为缓存。

## 错误处理与事务

- 每层定义自己的异常类；第三方异常（`httpx.HTTPError`、`SQLAlchemyError`、`RedisError`）在本层内转换，使用 `raise XxxError(...) from e` 保留异常链。
- 业务异常定义在 service 或 common；entry 统一转换为响应。
- 禁止裸 `except:`、`except Exception: pass`、捕获后只 `logger.exception` 继续执行。
- 失败必须明确重试（有上限、退避、抖动）、进入死信、熔断、返回错误或停机中的一种，不能只记日志继续。
- 事务边界在 service，通过 infra 提供的 UoW/session 接口控制；entry 不管理事务，infra 不自开事务。
- 事务内不做远程调用（交易所、HTTP、MQ 同步发送）；数据库与消息的一致性用 outbox 或提交后发送，不做双写。
- 捕获异常时保留 trace_id/order_id 等上下文；不要用 `str(e)` 替代异常链。

## 测试与验证

- 领域计算、状态流转必须有 pytest 单元测试，不启动事件循环和外部服务，覆盖正常、边界、非法输入。
- service 测试使用 fake 或内存实现，不连接真实数据库、Redis、交易所；不要 mock 被测对象自身。
- infra 用集成测试或契约测试验证协议、序列化、重连；不要 mock 掉被测的转换逻辑。
- 测试必须确定性：不依赖当前时间、随机数、真实网络、执行顺序；用固定时钟或 `freezegun`。
- 禁止 `time.sleep`/`asyncio.sleep` 等待异步结果，用 `asyncio.wait_for`、事件或条件轮询。
- 禁止为了变绿修改断言、删除用例、加 `pytest.mark.skip`、放宽超时；测试只加严不放宽。
- 覆盖率不是目标，但核心业务分支必须有测试；新增分支不得无测试。

## AI 常见劣化模式

| 劣化写法 | 正确做法 |
|---|---|
| service 直接 import sqlalchemy/redis/httpx | 通过 infra 封装访问 |
| 把类型挪到 common 让两层都能用 | 类型归真正拥有它的层，边界做转换 |
| 单实现也建 Protocol + Factory + ABC | 直接用具体类，出现第二实现再抽象 |
| 加 `if settings.use_new: ... else: ...` 兼容层 | 删旧路径，保持单一路径 |
| 改断言、加 skip、放宽超时让测试变绿 | 修代码；测试只加严 |
| 只跑单个测试就宣称全部通过 | 跑完整检查并贴输出 |
| `except Exception: logger.exception(...)` 后继续 | 转换抛出，或明确重试/死信/熔断 |
| `float` 金额、`Decimal(0.1)`、依赖默认舍入 | `Decimal(str)`、显式 `quantize` + rounding |
| async 里 `time.sleep`、`requests`、同步 DB | `asyncio.sleep`、异步客户端、`to_thread` |
| `create_task` 不保存引用、无界 Queue | `TaskGroup` + 有界队列 + 取消处理 |

## 检查方式与完成定义

### 检查

- `ruff check .` 和 `ruff format --check .`
- `mypy .`（或 `pyright`）：公开函数和类必须有完整类型注解。
- `lint-imports`：层间依赖和外部库限制，函数内部延迟 import 同样会被检查。
- `pytest`：全量，不只跑改动目录。

```toml
# pyproject.toml
[tool.importlinter]
root_package = "myapp"

[[tool.importlinter.contracts]]
name = "分层依赖"
type = "layers"
layers = ["myapp.bootstrap | myapp.main", "myapp.entry", "myapp.service", "myapp.infra", "myapp.common"]

[[tool.importlinter.contracts]]
name = "入口不直连基础设施"
type = "forbidden"
source_modules = ["myapp.entry"]
forbidden_modules = ["myapp.infra"]
```

### 完成定义（DoD）

- [ ] 代码在正确的层和包，依赖方向正确，`lint-imports` 通过。
- [ ] `ruff`、`mypy`、`pytest` 全部通过，并贴出真实输出。
- [ ] 新增业务分支有单元测试，覆盖失败路径。
- [ ] 没有 TODO、注释掉的代码、调试日志、未使用的参数。
- [ ] 错误处理、日志上下文、超时和取消符合本文件。
- [ ] 没有新增依赖、Protocol、工厂或兼容层；如确有必要，已在回复中说明理由。

## 已知的历史违规

> 记录现存不符合上述规则的代码，避免被模仿；修改相关代码时顺手修正。没有则保持为空。

- （无）
