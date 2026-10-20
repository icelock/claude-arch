# CLAUDE.md

本文件是 Python 语言级通用规则，供 `py_arch_layer_CLAUDE.md`（经典分层）和 `py_arch_onion_CLAUDE.md`（洋葱）共用。架构规则（分层、依赖方向、类型归属、端口）以对应的架构文件为准；两者冲突时以架构文件为准。

## 数据与一致性

- 金额、价格、数量统一使用 `decimal.Decimal`，禁止 `float`；序列化为字符串，不转成 float。
- 从字符串或整数构造：`Decimal("0.1")`，禁止 `Decimal(0.1)`。
- 舍入必须显式：`amount.quantize(Decimal("0.01"), rounding=ROUND_HALF_UP)`；不要修改全局 decimal context，也不要依赖默认舍入。入库前统一 scale。
- 下游写入使用确定性幂等键，数据库加唯一约束；重复消息不能产生重复副作用。
- 业务代码和纯函数不直接调用 `datetime.now()`/`datetime.utcnow()`/`time.time()`：把 `now` 作为参数传入，或由业务编排层注入时钟（`Callable[[], datetime]`）；领域层只接受传入的 `now`。时间一律带 UTC 时区；测量延迟和超时用 `time.monotonic()`。
- 契约演进：表结构变更走迁移脚本并保持向后兼容；消息和接口只加可选字段，不改语义、不删字段。这类改动仍需先问（硬性约束第 8 条）。

## 数据访问

- 禁止在循环里逐条 `await` 查库或调用远程接口（N+1），改为按 ID 集合批量查询；ORM 关联用 `selectinload` 等显式加载，不依赖懒加载；需要时由下层提供批量方法（洋葱架构在端口上提供）。
- 列表查询必须分页并设置上限；不允许无条件的全表查询。
- 批量写入用批量 API（如 `session.execute(insert(Model), rows)`），不要循环单条写。
- 只查需要的列；大字段按需单独查询。
- 事务尽量小：只包住必须原子完成的写操作，查询、计算、远程调用放在事务外。
- 一个 `AsyncSession` 只在一个任务内使用，不跨任务共享；提交后不访问未加载的懒加载属性。

## 并发

- asyncio 代码中禁止阻塞调用：`time.sleep`、`requests`、同步数据库驱动、大量 CPU 计算；用 `asyncio.sleep`、异步客户端、`asyncio.to_thread` 或进程池。
- 后台任务必须保存引用或用 `asyncio.TaskGroup` 管理；禁止 `create_task` 后丢弃引用；所有任务必须能取消并正确退出。
- 禁止吞掉 `asyncio.CancelledError`；清理逻辑放 `finally`，取消要继续向上传播。
- 有界队列（`asyncio.Queue(maxsize=...)`）必须明确队列满时的策略（等待、丢弃、拒绝受理、熔断），写在代码注释中；不能丢的数据，队列满时只能拒绝受理或熔断，不能丢弃。
- 线程场景用有界、有名字的 `ThreadPoolExecutor`，并有关闭流程；不要创建无界线程。
- async 代码用 `asyncio.Lock`，不要在事件循环里用阻塞的 `threading.Lock`；锁内不做 IO、不调用回调。
- 多线程共享状态用锁保护或改为消息传递；禁止模块级可变字典作为缓存。

## 错误处理与事务

- 业务异常定义在业务层：经典分层在 service 或 common；洋葱在 domain（领域异常）和 application（应用异常、端口异常）。入口层统一转换为响应（如 FastAPI 的 exception handler）。
- 基础设施层（infra / 出站适配器）把第三方异常（`httpx.HTTPError`、`SQLAlchemyError`、`RedisError`）转换为本层/端口约定的异常，用 `raise XxxError(...) from e` 保留异常链。
- 禁止裸 `except:`、`except Exception: pass`、捕获后只记日志继续执行；失败必须明确采取以下一种处理：重试（有上限、退避、抖动）、死信、熔断、返回错误、停机。
- 事务边界在业务编排层（经典分层为 service，洋葱为 application）：经典分层通过 infra 提供的 Unit of Work（`async with uow:`）控制，洋葱通过 `UnitOfWork` 端口控制。正常退出提交、异常退出回滚；入口层、基础设施层和组装层不开事务，出站适配器的仓储方法不自行提交。
- 事务不嵌套；确需在一个用例中分多段提交时，在业务编排层里顺序开启多个 `async with uow:`。
- 事务内不做远程调用（第三方 HTTP、MQ 同步发送）；数据库与消息的一致性用 outbox 或提交后发送，不做双写。

## 日志、安全与配置

- `logger = logging.getLogger(__name__)`；用 `%s` 惰性参数：`logger.info("order %s paid", order_no)`，不用 f-string 预先格式化。记录异常用 `logger.exception(...)` 或 `exc_info=True`，保留堆栈。
- 领域层和纯计算代码不打日志，用返回值或领域异常表达结果；日志放在业务编排层（service / application）和外层。
- 链路标识（如 `trace_id`）用 `contextvars` 传递，并通过 logging Filter 注入日志。asyncio 任务和 `asyncio.to_thread` 会自动复制上下文；直接向 `ThreadPoolExecutor` 提交时用 `contextvars.copy_context().run`。
- 日志级别：error 只用于需要人处理的问题，可预期的业务失败用 warning 或 info；循环内和高频路径的日志必须限频。
- 队列积压、丢弃、重试、重连次数必须有指标或限频日志。
- 密钥、token、密码不硬编码，不写进日志和异常信息；从配置中心或环境变量读取。
- SQL 一律参数化：SQLAlchemy 用绑定参数，`text()` 里禁止 f-string 或 `%` 拼接用户输入。
- 入参格式校验在入口层完成（经典分层为 entry，洋葱为入站适配器；pydantic 模型）；业务规则和不变量由业务层/领域对象在构造和状态流转时校验。
- 配置在组装层集中读取并校验（如 pydantic-settings），只在外层使用；内层需要的参数通过普通参数传入，不直接读 `os.environ` 或全局 settings 对象。URL、超时、并发数、队列长度不硬编码。
- 所有外部调用必须设置超时：`requests` 默认没有超时，必须传 `timeout`；httpx 显式传 `httpx.Timeout`；数据库连接池设置获取连接的超时。

## Python 编码细则

- 公开函数和类必须有完整类型注解；`Any`、`cast` 要注释原因；`# type: ignore` 必须带错误码和原因，如 `# type: ignore[attr-defined]  # 第三方库缺少类型`。
- 值对象和跨层数据用 `@dataclass(frozen=True, slots=True)`，在 `__post_init__` 中校验不变量；pydantic 只用于边界（entry 的请求响应、配置）。实体的状态变化通过表达业务意图的方法完成，外部不直接给属性赋值。
- 禁止可变默认参数（`def f(items=[])`）、`from x import *`、在 `__init__.py` 中写业务逻辑。
- 模块导入时不做 IO、不建连接、不读配置；这些放在组装层的启动流程中。
- 返回集合时返回空集合，不返回 None；返回 `X | None` 的函数，调用方必须处理 None。
- 注释写「为什么」，不写「做了什么」；公开函数的非显然前置条件、是否可在多个任务中并发调用写在 docstring 里。

## 测试与验证

- 业务计算、状态流转必须有 pytest 单元测试，不启动外部服务，覆盖正常、边界、非法输入。
- 业务编排层测试使用 fake 或内存实现替换下层（经典分层替换 infra，洋葱替换端口，包括内存 `UnitOfWork`），不连接真实数据库、Redis、第三方服务；不要 mock 被测对象自身。
- 基础设施层（infra / 出站适配器）用集成测试或契约测试验证协议、序列化、重连；不要 mock 掉被测的转换逻辑。
- 测试必须确定性：不依赖当前时间、随机数、真实网络、执行顺序；注入固定时钟。禁止 `time.sleep`/`asyncio.sleep` 等待异步结果，用 `asyncio.wait_for`、`asyncio.Event` 或带上限的轮询。
- 禁止为了变绿修改断言、删除用例、加 `pytest.mark.skip`/`xfail`、放宽超时；测试只加严不放宽。
- 覆盖率不是目标，但核心业务分支必须有测试；新增分支不得没有测试。
