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

## 本项目事实

> 拷贝模板后先填写本节。AI 依据这里的版本选择语法和 API；仍是 `<填写>` 的项先问用户，不要猜。

- Python：`<填写：3.11 / 3.12 / 3.13>`（3.11 起才有 `asyncio.TaskGroup`、`typing.Self`）
- 包管理：`<填写：uv / poetry / pip-tools>`；依赖声明在 `pyproject.toml`
- Web 框架：`<填写：FastAPI / Django / Flask / 无>`；ORM：`<填写：SQLAlchemy 2.x / Django ORM / 无>`；表结构迁移：`<填写：Alembic / Django migrations / 无>`
- 并发模型：`<填写：asyncio / 同步 + 线程池>`。asyncio 项目中，业务路径上的 IO 一律用异步客户端
- 类型检查：`<填写：mypy / pyright>`；格式化与 lint：`<填写：ruff>`
- 检查命令：`<填写，如 uv run ruff check . && uv run mypy . && uv run pytest>`

## 规则优先级与工作方式

规则冲突时按以下顺序取舍：**用户当次明确指令 > 正确性与安全 > 本文件的架构规则 > 仓库现有写法 > 通用最佳实践**。例外：安全类规则（密钥、注入、权限校验）即使用户要求放宽，也要先说明风险并得到确认。现有代码违反本文件时不要模仿，新代码按本文件写。

- 跨层改动或改动超过 3 个文件时，先用几行说明计划：落在哪层、改哪些文件、是否涉及「必须停下来问」的事项，再动手。
- 修 bug 先写一个能复现问题的失败测试，再修代码，让它变绿。
- 小的歧义（命名、放在同层哪个子包、日志措辞）不要打断用户：选仓库惯例或最保守的做法，在回复中写明假设。
- 完成时按以下格式汇报：
  1. 改动文件及所属层；
  2. 关键决策和假设；
  3. 执行的检查命令和真实输出（失败也照贴）；
  4. 遗留问题和风险，包括发现但没有修的历史违规。

## 对 AI 的硬性约束

本节是 MUST，优先于你的默认习惯和所谓的「最佳实践」。

1. **先定位再写**：动手前确定代码属于哪一层、哪个包。落不进「本项目的层级」表就停下来问，不要自创包。
2. **先读后写**：找同层 1–2 个现有实现，沿用其结构、命名、异常处理和日志方式。不要引入仓库里没有的新风格。
3. **最小改动**：只改与需求相关的代码。不顺手重构、不重排 import、不升级依赖、不改 ruff/format 配置。
4. **抽象准入**：没有第二个实现、不属于真实 IO 边界时，不建 Protocol、ABC、工厂、泛型仓储、兼容层、fallback、feature flag。三行重复优于一个错误抽象。
5. **依赖准入**：不新增第三方库。确需新增时，先说明用途、体积、维护状态，等用户确认后再改 `pyproject.toml`。
6. **不写占位**：不留 TODO、mock 返回值、假数据、注释掉的代码、未使用的参数或字段。
7. **验证真实**：完成前运行「检查」一节列出的全部检查，贴出命令和真实输出。失败就说失败并继续修，禁止改断言、加 skip、放宽超时、更新快照来让结果变绿。
8. **必须停下来问**：层级归属不明；新增包或层；新增依赖；修改公共类型、对外接口、表结构、消息格式；修复历史违规会超出本次需求的改动范围。

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

| 层 | 包 | 可以被谁依赖 | 外部依赖限制 |
|---|---|---|---|
| 组装 | `myapp.bootstrap`、`myapp.main` | 无 | 无 |
| entry | `myapp.entry`：`entry.api`（HTTP 路由）、`entry.consumer`（MQ 消费）、`entry.jobs`（定时任务） | 组装 | 不直接用 sqlalchemy、redis |
| service | `myapp.service` | entry、组装 | 不直接用 sqlalchemy、redis、HTTP/WS 客户端、Kafka 客户端 |
| infra | `myapp.infra`（如 `infra.db`、`infra.payment`、`infra.ws`） | service、组装 | 无 |
| common | `myapp.common` | 所有层 | 只允许标准库和基础工具库 |

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

## 数据与一致性

- 金额、价格、数量统一使用 `decimal.Decimal`，禁止 `float`；序列化为字符串，不转成 float。
- 从字符串或整数构造：`Decimal("0.1")`，禁止 `Decimal(0.1)`。
- 舍入必须显式：`amount.quantize(Decimal("0.01"), rounding=ROUND_HALF_UP)`；不要修改全局 decimal context，也不要依赖默认舍入。入库前统一 scale。
- 下游写入使用确定性幂等键，数据库加唯一约束；重复消息不能产生重复副作用。
- 业务代码不直接调用 `datetime.now()`/`datetime.utcnow()`/`time.time()`：注入时钟（`Callable[[], datetime]`）或把 `now` 作为参数；时间一律带 UTC 时区；测量延迟和超时用 `time.monotonic()`。
- 契约演进：表结构变更走迁移脚本并保持向后兼容；消息和接口只加可选字段，不改语义、不删字段。这类改动仍需先问（硬性约束第 8 条）。

## 数据访问

- 禁止在循环里逐条 `await` 查库或调用远程接口（N+1），改为按 ID 集合批量查询；SQLAlchemy 关联用 `selectinload` 等显式加载，不依赖懒加载。
- 列表查询必须分页并设置上限；不允许无条件的全表查询。
- 批量写入用批量 API（如 `session.execute(insert(Model), rows)`），不要循环单条写。
- 只查需要的列；大字段按需单独查询。
- 事务尽量小：只包住必须原子完成的写操作，查询、计算、远程调用放在事务外。
- 一个 `AsyncSession` 只在一个任务内使用，不跨任务共享；提交后不访问未加载的懒加载属性。

## 并发

- asyncio 代码中禁止阻塞调用：`time.sleep`、`requests`、同步数据库驱动、大量 CPU 计算；用 `asyncio.sleep`、异步客户端、`asyncio.to_thread` 或进程池。
- 后台任务必须保存引用或用 `asyncio.TaskGroup` 管理；禁止 `create_task` 后丢弃引用；所有任务必须能取消并正确退出。
- 禁止吞掉 `asyncio.CancelledError`；清理逻辑放 `finally`，取消要继续向上传播。
- 有界队列必须明确队列满时的策略（等待、丢弃、拒绝受理、熔断），写在代码注释中；不能丢的数据，队列满时只能拒绝受理或熔断，不能丢弃。
- 线程场景用有界、有名字的 `ThreadPoolExecutor`，并有关闭流程；不要创建无界线程。
- async 代码用 `asyncio.Lock`，不要在事件循环里用阻塞的 `threading.Lock`；锁内不做 IO、不调用回调。
- 多线程共享状态用锁保护或改为消息传递；禁止模块级可变字典作为缓存。

## 错误处理与事务

- 每层定义自己的异常类；第三方异常（`httpx.HTTPError`、`SQLAlchemyError`、`RedisError`）在 infra 内转换，用 `raise XxxError(...) from e` 保留异常链。
- 业务异常定义在 service 或 common；entry 统一转换为响应（如 FastAPI 的 exception handler）。
- 禁止裸 `except:`、`except Exception: pass`、捕获后只记日志继续执行；失败必须明确采取以下一种处理：重试（有上限、退避、抖动）、死信、熔断、返回错误、停机。
- 事务边界在 service：通过 infra 提供的 Unit of Work（`async with uow:`）控制，正常退出提交、异常退出回滚。entry、infra、组装层不开事务。
- 事务不嵌套；确需在一个用例中分多段提交时，在 service 里顺序开启多个 `async with uow:`。
- 事务内不做远程调用（第三方 HTTP、MQ 同步发送）；数据库与消息的一致性用 outbox 或提交后发送，不做双写。

## 日志、安全与配置

- `logger = logging.getLogger(__name__)`；用 `%s` 惰性参数：`logger.info("order %s paid", order_no)`，不用 f-string 预先格式化。记录异常用 `logger.exception(...)` 或 `exc_info=True`，保留堆栈。
- 链路标识（如 `trace_id`）用 `contextvars` 传递，并通过 logging Filter 注入日志。asyncio 任务和 `asyncio.to_thread` 会自动复制上下文；直接向 `ThreadPoolExecutor` 提交时用 `contextvars.copy_context().run`。
- 日志级别：error 只用于需要人处理的问题，可预期的业务失败用 warning 或 info；循环内和高频路径的日志必须限频。
- 队列积压、丢弃、重试、重连次数必须有指标或限频日志。
- 密钥、token、密码不硬编码，不写进日志和异常信息；从配置中心或环境变量读取。
- SQL 一律参数化：SQLAlchemy 用绑定参数，`text()` 里禁止 f-string 或 `%` 拼接用户输入。
- 入参校验在 entry 完成（pydantic 模型），service 只校验业务规则。
- 配置在组装层集中读取并校验（如 pydantic-settings），以普通参数传给各层；service 和 infra 不直接读 `os.environ` 或全局 settings 对象。URL、超时、并发数、队列长度不硬编码。
- 所有外部调用必须设置超时：`requests` 默认没有超时，必须传 `timeout`；httpx 显式传 `httpx.Timeout`；数据库连接池设置获取连接的超时。

## Python 编码细则

- 公开函数和类必须有完整类型注解；`Any`、`cast` 要注释原因；`# type: ignore` 必须带错误码和原因，如 `# type: ignore[attr-defined]  # 第三方库缺少类型`。
- 值对象和跨层数据用 `@dataclass(frozen=True, slots=True)`；pydantic 只用于边界（entry 的请求响应、配置）。
- 禁止可变默认参数（`def f(items=[])`）、`from x import *`、在 `__init__.py` 中写业务逻辑。
- 模块导入时不做 IO、不建连接、不读配置；这些放在组装层的启动流程中。
- 返回集合时返回空集合，不返回 None；返回 `X | None` 的函数，调用方必须处理 None。
- 注释写「为什么」，不写「做了什么」；公开函数的非显然前置条件、是否可在多个任务中并发调用写在 docstring 里。

## 测试与验证

- 业务计算、状态流转必须有 pytest 单元测试，不启动外部服务，覆盖正常、边界、非法输入。
- service 测试使用 fake 或内存实现替换 infra，不连接真实数据库、Redis、第三方服务；不要 mock 被测对象自身。
- infra 用集成测试或契约测试验证协议、序列化、重连；不要 mock 掉被测的转换逻辑。
- 测试必须确定性：不依赖当前时间、随机数、真实网络、执行顺序；注入固定时钟。禁止 `time.sleep`/`asyncio.sleep` 等待异步结果，用 `asyncio.wait_for`、`asyncio.Event` 或带上限的轮询。
- 禁止为了变绿修改断言、删除用例、加 `pytest.mark.skip`/`xfail`、放宽超时；测试只加严不放宽。
- 覆盖率不是目标，但核心业务分支必须有测试；新增分支不得没有测试。

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

## 检查方式与完成定义

### 检查

- `ruff check .` 和 `ruff format --check .`
- `mypy .`（或 `pyright`）：公开函数和类必须有完整类型注解。
- `pytest`：全量，不只跑改动目录。
- 以下规则靠自查和 review：common 只用标准库和基础工具库、金额构造与舍入、事务内远程调用、async 中的阻塞调用、锁内 IO、日志中的敏感信息、队列满时的策略。

### 完成定义（DoD）

- [ ] 代码在正确的层和包，依赖方向符合本文件。
- [ ] `ruff`、`mypy`、`pytest` 全部通过，并贴出真实输出。
- [ ] 新增业务分支有单元测试，覆盖失败路径；修 bug 有复现测试。
- [ ] 没有 TODO、注释掉的代码、调试日志、未使用的参数。
- [ ] 错误处理、日志、超时与取消、配置方式符合本文件。
- [ ] 没有新增依赖、Protocol、工厂或兼容层；如确有必要，已在回复中说明理由。
- [ ] 已按「工作方式」的格式汇报。

## 已知的历史违规

> 记录现存不符合上述规则的代码，避免被模仿。只在本次需求本来就要改到的代码里顺带修正；超出这个范围的不动，在汇报中列出。没有则保持为空。

- （无）

## 附录：实时交易系统（可选）

> 非交易类项目删除本节。本节规则补充正文，冲突时以本节为准。

- 交易所报文、签名、限频（rate limit）、错误码只在 infra 处理；返回上层前转换为业务类型，不把交易所字段名带进 service。
- 下单使用确定性的 `client_order_id` 作为幂等键，超时或网络错误后先按它查询订单状态再决定是否重下，禁止盲目重试。
- 行情、订单推送必须处理重复、乱序、gap、断线重连：出现 gap 时作废本地缓存并重拉快照，不能跳过继续用；重连后必须重新订阅。
- 背压分级：行情类可以丢弃并置 gap 标记；订单、回报、成交类不能丢，队列满时拒绝受理或熔断。
- WS 接收循环只做解析和投递，不查库、不发 HTTP、不 `await` 慢操作。
- 热路径禁止逐笔日志、`json.dumps`、f-string 拼接、反射/inspect、无界重试和不必要的对象分配。
- 价格和数量按交易对的 tickSize/stepSize 用 `quantize` 显式取整，舍入方向由业务决定（买卖方向不同），写在代码里，不依赖默认值。
