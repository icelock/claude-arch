# CLAUDE.md

## 架构声明

本项目采用**洋葱架构**：从内到外分为**领域层（domain）→ 应用层（application）→ 外层（adapter + bootstrap）**，依赖只能由外向内。

```
外层：adapter.inbound（入站）、adapter.outbound（出站）、bootstrap（组装）
  └── 应用层：用例编排、事务边界、出站端口 Protocol
        └── 领域层：实体、值对象、领域服务、领域事件
依赖只能由外向内
```

**组装层（`bootstrap`、`main`）可以依赖所有层**：它只做接线，是唯一同时知道所有具体实现的地方；任何层都不能反过来依赖组装层。

六边形架构和整洁架构与本项目是同一条依赖规则的不同命名：六边形强调入站/出站端口对称，整洁架构强调用例输入/输出端口。**本项目统一按洋葱落法，不要引入第四种命名、分层或目录结构。**

包名用 `inbound`/`outbound`，不用 `in`/`out`（`in` 是 Python 关键字）。

## 本项目事实

> 拷贝模板后先填写本节。AI 依据这里的版本选择语法和 API；仍是 `<填写>` 的项先问用户，不要猜。

- Python：`<填写：3.11 / 3.12 / 3.13>`（3.11 起才有 `asyncio.TaskGroup`、`typing.Self`）
- 包管理：`<填写：uv / poetry / pip-tools>`；依赖声明在 `pyproject.toml`
- Web 框架：`<填写：FastAPI / Flask / 无>`；ORM：`<填写：SQLAlchemy 2.x / 无>`；表结构迁移：`<填写：Alembic / 无>`
- 并发模型：`<填写：asyncio / 同步 + 线程池>`。端口 Protocol 的同步/异步与此一致，不要混用
- 类型检查：`<填写：mypy / pyright>`；格式化与 lint：`<填写：ruff>`
- 检查命令：`<填写，如 uv run ruff check . && uv run mypy . && uv run pytest>`

## 规则优先级与工作方式

规则冲突时按以下顺序取舍：**用户当次明确指令 > 正确性与安全 > 本文件的架构规则 > 仓库现有写法 > 通用最佳实践**。例外：安全类规则（密钥、注入、权限校验）即使用户要求放宽，也要先说明风险并得到确认。现有代码违反本文件时不要模仿，新代码按本文件写。

- 跨层改动或改动超过 3 个文件时，先用几行说明计划：落在哪层、改哪些文件、是否新增端口、是否涉及「必须停下来问」的事项，再动手。
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
4. **抽象准入**：真实 IO 边界的出站端口是架构要求，必须建；一个外部能力对应一个端口，不按方法拆。除此之外，没有第二个实现时，不建 Protocol、ABC、工厂、泛型仓储、兼容层、fallback、feature flag；纯计算不建端口。三行重复优于一个错误抽象。
5. **依赖准入**：不新增第三方库。确需新增时，先说明用途、体积、维护状态，等用户确认后再改 `pyproject.toml`。
6. **不写占位**：不留 TODO、mock 返回值、假数据、注释掉的代码、未使用的参数或字段。
7. **验证真实**：完成前运行「检查」一节列出的全部检查，贴出命令和真实输出。失败就说失败并继续修，禁止改断言、加 skip、放宽超时、更新快照来让结果变绿。
8. **必须停下来问**：层级归属不明；新增包或层；新增依赖；修改公共类型、端口签名、对外接口、表结构、消息格式；修复历史违规会超出本次需求的改动范围。

## 分层与依赖

### 各层职责

| 层 | 职责 | 典型内容 |
|---|---|---|
| 领域层 | 业务概念和业务规则，不依赖框架和 IO | 实体、值对象、领域服务、领域事件、领域异常 |
| 应用层 | 编排用例：加载、调用领域对象、调用端口、发布事件、控制事务；定义出站端口 | 应用服务、端口 Protocol、命令/结果类型、应用异常 |
| 外层·入站适配器 | 接收外部触发，转换为内层类型后调用应用层 | FastAPI 路由、WS 推送接收、MQ 消费者、定时任务、CLI |
| 外层·出站适配器 | 实现应用层端口，访问外部系统 | 第三方服务客户端、ORM 仓储与 Unit of Work、Redis、Kafka 发布 |
| 外层·组装 | 创建对象、注入依赖、读取配置、启停任务，只做接线 | `myapp/bootstrap.py`、`myapp/main.py` |

不设单独的 common 层：业务通用类型放领域层；与业务无关的纯函数放 `myapp.domain.shared`，不得依赖 IO。

### 本项目的层级

> 下表为示例，按实际项目修改。新增包、拆分适配器时同步更新本表。

| 层 | 包 | 可以依赖 | 外部依赖限制 |
|---|---|---|---|
| 领域层 | `myapp.domain`（含 `domain.shared`） | 无 | 只允许标准库 |
| 应用层 | `myapp.application`：`application.services`、`application.queries`、`application.ports` | domain | 只允许 domain 和标准库 |
| 入站适配器 | `myapp.adapter.inbound`：`inbound.api`、`inbound.ws`、`inbound.consumer`、`inbound.jobs` | application、domain | 不依赖出站适配器 |
| 出站适配器 | `myapp.adapter.outbound`：`outbound.payment`、`outbound.db`、`outbound.mq` | application、domain | 不依赖入站适配器，彼此不依赖 |
| 组装 | `myapp.bootstrap`、`myapp.main` | 所有包 | 无 |

### 依赖规则

- **依赖只能由外向内**：domain 不 import 任何项目包；application 只 import domain；适配器 import application 和 domain；bootstrap import 所有包。
- **适配器之间互不依赖**：入站适配器只调用应用层，不能直接调用出站适配器或 ORM 查询；不同出站适配器之间也不互相 import。只读查询走应用层的查询服务。
- **领域层不依赖任何框架和 IO**：只用标准库（`dataclasses`、`decimal`、`enum`、`datetime`），不 import pydantic、sqlalchemy、redis、httpx、fastapi。
- **应用层不做 IO**：只依赖 domain 和标准库（包括 `asyncio`、`typing`），外部能力全部通过端口获得。
- 所有依赖在 bootstrap 通过构造函数传入；禁止函数内部延迟 import、`importlib.import_module`、模块级全局变量或注册表保存服务对象。FastAPI 的 `Depends` 只在入站适配器使用。服务实例由 bootstrap 在启动时挂到 `app.state` 上，入站适配器的依赖函数从 `request.app.state` 取出；入站适配器不得 import bootstrap。
- 依赖图不能有环，包括函数内部延迟 import 形成的环。

### 新代码放哪里

- 新的业务概念、业务规则 → 领域层：规则优先写在实体和值对象中，跨实体的规则写领域服务。
- 新的业务流程 → `application.services`；需要外部能力时在 `application.ports` 新增 Protocol。
- 新的外部系统接入 → `adapter.outbound`，实现已有或新增的端口。
- 新的触发方式 → `adapter.inbound`，只做转换和调用。
- 新增包或适配器：先更新层级表，再写代码。

## 端口与跨边界类型

- **出站端口定义在应用层**的 `application.ports`，使用 `typing.Protocol`；出站适配器实现对应方法，不需要继承。只有领域服务本身需要调用的端口才放领域层。
- 入站端口默认不定义，入站适配器直接调用应用服务；只有需要多种实现时才定义。
- 端口按业务需要设计（`charge(order: Order)`），不按外部系统 API 设计（禁止 `charge(req: StripeChargeRequest)` 或 `charge(payload: dict)`）。
- **事务通过 Unit of Work 端口表达**：`UnitOfWork` Protocol 定义在 `application.ports`，聚合仓储作为它的属性；出站适配器（如 `outbound.db`）用 SQLAlchemy session 实现。
- **查询侧**：只读查询放 `application.queries`，查询端口放 `application.ports`，由 `outbound.db` 实现；可以直接返回只读视图（frozen dataclass），不必经过领域模型。入站适配器不得直接执行 ORM 查询。
- **跨边界的类型归内层所有，转换由外层负责**：入站适配器把 HTTP/WS/MQ 报文转换为应用命令或领域类型；出站适配器把领域类型转换为第三方请求、ORM 模型、Kafka 消息，返回时再转回内层类型。
- pydantic 模型、ORM 模型、第三方报文 dict、Kafka 消息体只能出现在适配器中；领域类型用 dataclass 或普通类，不用 pydantic。

```python
# myapp/domain/order.py：业务规则，只用标准库
@dataclass(frozen=True, slots=True)
class Money:
    amount: Decimal
    currency: str  # 加减、比较、舍入规则写在这里

class Order:
    @classmethod
    def place(cls, cart: Cart, now: datetime) -> "Order": ...                    # 计算金额、校验约束
    def start_payment(self, now: datetime) -> None: ...                          # 只有「待支付」可以发起，否则抛领域异常
    def mark_paid(self, receipt: PaymentReceipt, now: datetime) -> OrderPaid: ...  # 只有「支付中」可以确认；生成领域事件

# myapp/application/ports.py：出站端口，签名只用内层类型
class PaymentGateway(Protocol):
    async def charge(self, order: Order) -> PaymentReceipt:
        """实现方必须以订单号作为幂等键，重复调用不会重复扣款。"""

class UnitOfWork(Protocol):
    orders: OrderRepository
    outbox: OutboxWriter  # 与订单写入同库同事务
    async def __aenter__(self) -> Self: ...
    async def __aexit__(self, *exc_info: object) -> None: ...  # 正常退出提交，异常退出回滚

# myapp/application/services/pay_order.py：只编排，不写业务计算；远程调用在事务外
class PayOrder:
    def __init__(self, uow: Callable[[], UnitOfWork], gateway: PaymentGateway, clock: Callable[[], datetime]) -> None:
        self._uow, self._gateway, self._clock = uow, gateway, clock

    async def __call__(self, order_id: OrderId) -> None:
        async with self._uow() as uow:                    # 事务 1：待支付 → 支付中，提交后才调用外部
            order = await uow.orders.get(order_id)
            order.start_payment(self._clock())
            await uow.orders.save(order)
        receipt = await self._gateway.charge(order)       # 事务外；超时可安全重试
        async with self._uow() as uow:                    # 事务 2：支付中 → 已支付，领域事件写入 outbox
            order = await uow.orders.get(order_id)
            await uow.outbox.add(order.mark_paid(receipt, self._clock()))
            await uow.orders.save(order)
    # charge 失败或进程崩溃时订单停在「支付中」，由对账任务按订单号查询支付结果补偿

# myapp/adapter/outbound/payment.py：实现端口，在这里转换外部格式
class HttpPaymentGateway:
    def __init__(self, http: httpx.AsyncClient) -> None:
        self._http = http

    async def charge(self, order: Order) -> PaymentReceipt:
        resp = await self._http.post("/charges", json=to_request(order), headers={"Idempotency-Key": str(order.id)})
        resp.raise_for_status()
        return to_receipt(resp.json())

# myapp/adapter/inbound/api/orders.py：转换后调用应用层
def get_pay_order(request: Request) -> PayOrder:
    return request.app.state.pay_order  # bootstrap 启动时挂上；这里不 import bootstrap

@router.post("/orders/{order_id}/pay")
async def pay(order_id: int, pay_order: PayOrder = Depends(get_pay_order)) -> None:
    await pay_order(OrderId(order_id))
```

### 层级内部规则

- 领域层：不直接取当前时间、随机数、全局 ID，这些都通过参数传入；必须有纯单元测试。
- 聚合：一个 `async with uow:` 只修改一个聚合；聚合之间按 ID 引用，不持有对方对象；跨聚合的一致性用领域事件实现最终一致。仓储按聚合定义，一个聚合一个。
- 应用层：只做加载、调用领域对象、调用端口、发布事件、控制事务；不写金额计算，也不写承载业务规则的条件分支。幂等、去重、并发控制属于应用层，具体实现通过端口交给外层。
- 外层：入站只转换和调用；出站只处理协议、签名、序列化、连接、重连、持久化，不做业务判断。回调运行在 WS/MQ 接收循环中时，耗时处理必须交给其他任务。

## 数据与一致性

- 金额、价格、数量统一使用 `decimal.Decimal`，禁止 `float`；从字符串或整数构造（`Decimal("0.1")`，禁止 `Decimal(0.1)`）；序列化为字符串，不转成 float。
- 舍入必须显式：`amount.quantize(Decimal("0.01"), rounding=ROUND_HALF_UP)`；不要修改全局 decimal context，也不要依赖默认舍入。入库前统一 scale。
- 下游写入使用确定性幂等键，数据库加唯一约束；重复消息不能产生重复副作用。
- 领域层不调用 `datetime.now()`/`datetime.utcnow()`/`time.time()`，`now` 作为参数传入；应用层注入时钟（`Callable[[], datetime]`）取时间。时间一律带 UTC 时区；测量延迟和超时用 `time.monotonic()`。
- 契约演进：表结构变更走迁移脚本并保持向后兼容；消息和接口只加可选字段，不改语义、不删字段。这类改动仍需先问（硬性约束第 8 条）。

## 数据访问

- 禁止在循环里逐条 `await` 查库或调用远程接口（N+1），改为按 ID 集合批量查询；需要时在端口上提供批量方法。
- 列表查询必须分页并设置上限；不允许无条件的全表查询。
- 批量写入用批量 API（如 `session.execute(insert(Model), rows)`），不要循环单条写。
- 只查需要的列；ORM 关联显式加载（如 `selectinload`），不依赖懒加载。
- 事务尽量小：只包住必须原子完成的写操作，查询、计算、远程调用放在事务外。

## 并发

- asyncio 代码中禁止阻塞调用：`time.sleep`、`requests`、同步数据库驱动、大量 CPU 计算；用 `asyncio.sleep`、异步客户端、`asyncio.to_thread` 或进程池。
- 后台任务必须保存引用或用 `asyncio.TaskGroup` 管理；禁止 `create_task` 后丢弃引用；所有任务必须能取消并正确退出。
- 禁止吞掉 `asyncio.CancelledError`；清理逻辑放 `finally`，取消要继续向上传播。
- 有界队列（`asyncio.Queue(maxsize=...)`）必须明确队列满时的策略（等待、丢弃、拒绝受理、熔断），写在代码注释中；不能丢的数据，队列满时只能拒绝受理或熔断，不能丢弃。
- 线程场景用有界、有名字的 `ThreadPoolExecutor`，并有关闭流程。
- async 代码用 `asyncio.Lock`，不要在事件循环里用阻塞的 `threading.Lock`；锁内不做 IO、不调用回调。
- 一个 `AsyncSession` 只在一个任务内使用，不跨任务共享；禁止模块级可变字典作为缓存。

## 错误处理与事务

- 领域异常定义在领域层，应用异常和端口异常定义在应用层；出站适配器把第三方异常转换为端口约定的异常，用 `raise ... from e` 保留异常链；入站适配器统一把内层异常转换为响应。
- 禁止裸 `except:`、`except Exception: pass`、捕获后只记日志继续执行；失败必须明确采取以下一种处理：重试（有上限、退避、抖动）、死信、熔断、返回错误、停机。
- 事务边界在应用层：只通过 `UnitOfWork` 端口开启，正常退出提交、异常退出回滚。适配器和组装层不开事务，出站适配器的仓储方法不自行提交。
- 事务不嵌套；确需在一个用例中分多段提交时，像上例一样顺序开启多个 `async with uow:`。
- 事务内不做远程调用（第三方 HTTP、MQ 同步发送）；数据库与消息的一致性用 outbox 或提交后发送，不做双写。

## 日志、安全与配置

- `logger = logging.getLogger(__name__)`；用 `%s` 惰性参数：`logger.info("order %s paid", order_no)`，不用 f-string 预先格式化。记录异常用 `logger.exception(...)` 或 `exc_info=True`，保留堆栈。
- 领域层不打日志，用返回值或领域异常表达结果；日志放在应用层和适配器。
- 链路标识（如 `trace_id`）用 `contextvars` 传递，并通过 logging Filter 注入日志；直接向 `ThreadPoolExecutor` 提交时用 `contextvars.copy_context().run`。
- 日志级别：error 只用于需要人处理的问题，可预期的业务失败用 warning 或 info；循环内和高频路径的日志必须限频。
- 队列积压、丢弃、重试、重连次数必须有指标或限频日志。
- 密钥、token、密码不硬编码，不写进日志和异常信息；从配置中心或环境变量读取。
- SQL 一律参数化：SQLAlchemy 用绑定参数，`text()` 里禁止 f-string 或 `%` 拼接用户输入。
- 格式校验在入站适配器完成（pydantic 模型）；业务不变量由领域对象在构造和状态流转时校验。
- 配置在 bootstrap 集中读取并校验（如 pydantic-settings），只在外层使用；内层需要的参数通过构造函数传入，不读 `os.environ`。URL、超时、并发数、队列长度不硬编码。
- 所有外部调用必须设置超时：`requests` 默认没有超时，必须传 `timeout`；httpx 显式传 `httpx.Timeout`；数据库连接池设置获取连接的超时。

## Python 编码细则

- 公开函数和类必须有完整类型注解；`Any`、`cast` 要注释原因；`# type: ignore` 必须带错误码和原因。
- 值对象用 `@dataclass(frozen=True, slots=True)`，在 `__post_init__` 中校验不变量；实体的状态变化通过表达业务意图的方法完成，外部不直接给属性赋值。
- 禁止可变默认参数、`from x import *`、在 `__init__.py` 中写业务逻辑。
- 模块导入时不做 IO、不建连接、不读配置；这些放在 bootstrap 的启动流程中。
- 返回集合时返回空集合，不返回 None；返回 `X | None` 的函数，调用方必须处理 None。
- 注释写「为什么」，不写「做了什么」；公开函数的非显然前置条件、是否可在多个任务中并发调用写在 docstring 里。

## 测试与验证

- 领域计算、状态流转必须有 pytest 单元测试，不依赖 IO 和事件循环，覆盖正常、边界、非法输入。
- 应用层测试使用端口的内存实现或 fake（包括内存 `UnitOfWork`），不连接真实数据库、Redis、第三方服务；不要 mock 被测对象自身。
- 适配器用集成测试或契约测试验证协议、序列化、重连；不要 mock 掉被测的转换逻辑。
- 测试必须确定性：注入固定时钟；禁止 `time.sleep`/`asyncio.sleep` 等待异步结果，用 `asyncio.wait_for`、`asyncio.Event` 或带上限的轮询。
- 禁止为了变绿修改断言、删除用例、加 `pytest.mark.skip`/`xfail`、放宽超时；测试只加严不放宽。
- 覆盖率不是目标，但核心业务分支必须有测试；新增分支不得没有测试。

## AI 常见劣化模式

| 劣化写法 | 正确做法 |
|---|---|
| domain import pydantic/sqlalchemy/fastapi，领域对象用 pydantic 模型 | 领域层只用标准库和 dataclass |
| 入站适配器直接调用出站适配器或执行 ORM 查询 | 走应用服务 + 端口；查询走 `application.queries` |
| 端口签名里出现第三方请求类型或 `dict` | 端口只用内层类型，转换放在适配器 |
| 单实现也建工厂、ABC，为纯计算建 Protocol，或加 `if settings.use_new` 兼容层 | 只为 IO 边界建端口，其余用具体类，保持单一路径 |
| 应用服务里写金额计算和业务 if-else | 规则下沉到实体、值对象或领域服务 |
| 外部直接给实体属性赋值改状态 | 用表达业务意图的方法 |
| 一个 `async with uow:` 里修改多个聚合 | 按 ID 引用，跨聚合用领域事件 |
| 远程调用放在 `async with uow:` 内，或先调外部、后落库且没有幂等键 | 先落库「处理中」，以业务 ID 为幂等键调用外部，再落库结果 |
| `except Exception: logger.exception(...)` 后继续 | 转换后抛出，或明确重试、死信、熔断 |
| async 里 `time.sleep`、`requests`、同步 DB 驱动 | `asyncio.sleep`、异步客户端、`to_thread` |
| 改断言、加 skip，或只跑单个测试就宣称通过 | 修代码，跑完整检查并贴输出 |

## 检查方式与完成定义

### 检查

- `ruff check .` 和 `ruff format --check .`
- `mypy .`（或 `pyright`）：公开函数和类必须有完整类型注解。
- `pytest`：全量，不只跑改动目录。
- 以下规则靠自查和 review：内层使用标准库 IO（`open`、`socket`、`sqlite3`）、端口按业务设计而不是按外部 API 设计、应用层不写业务规则、一个事务只改一个聚合、金额构造与舍入、事务内远程调用、async 中的阻塞调用、日志中的敏感信息、队列满时的策略。

### 完成定义（DoD）

- [ ] 代码在正确的层和包，依赖方向符合本文件。
- [ ] `ruff`、`mypy`、`pytest` 全部通过，并贴出真实输出。
- [ ] 新增业务分支有单元测试，覆盖失败路径；修 bug 有复现测试。
- [ ] 没有 TODO、注释掉的代码、调试日志、未使用的参数。
- [ ] 错误处理、日志、超时与取消、配置方式符合本文件。
- [ ] 没有新增依赖、工厂或兼容层；新增端口对应真实 IO 边界；如有例外，已在回复中说明理由。
- [ ] 已按「工作方式」的格式汇报。

## 已知的历史违规

> 记录现存不符合上述规则的代码，避免被模仿。只在本次需求本来就要改到的代码里顺带修正；超出这个范围的不动，在汇报中列出。没有则保持为空。

- （无）

## 附录：实时交易系统（可选）

> 非交易类项目删除本节。本节规则补充正文，冲突时以本节为准。

- 交易所报文、签名、限频（rate limit）、错误码只在出站适配器处理；端口按业务设计（`submit(order: HedgeOrder)`），禁止 `submit(req: BinanceOrderRequest)`。
- 下单使用确定性的 `client_order_id` 作为幂等键，超时或网络错误后先按它查询订单状态再决定是否重下，禁止盲目重试。
- 行情、订单推送必须处理重复、乱序、gap、断线重连：出现 gap 时作废本地缓存并重拉快照，不能跳过继续用；重连后必须重新订阅。
- 背压分级：行情类可以丢弃并置 gap 标记；订单、回报、成交类不能丢，队列满时拒绝受理或熔断。
- WS 接收循环只做解析和投递，不查库、不发 HTTP、不 `await` 慢操作。
- 热路径禁止逐笔日志、`json.dumps`、f-string 拼接、反射/inspect、无界重试和不必要的对象分配。
- 价格和数量按交易对的 tickSize/stepSize 取整，取整规则写在领域值对象中（如 `Symbol.align_price(price, side)`），舍入方向由买卖方向决定，不依赖默认值。
