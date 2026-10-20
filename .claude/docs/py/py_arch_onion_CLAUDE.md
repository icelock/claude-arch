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
7. **验证真实**：完成前运行对应语言检查文件（`py_check_CLAUDE.md`）列出的全部检查，贴出命令和真实输出。失败就说失败并继续修，禁止改断言、加 skip、放宽超时、更新快照来让结果变绿。
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

## 架构自查

- 代码在正确的层和包，依赖方向由外向内；适配器之间互不依赖。
- 内层不使用标准库 IO（`open`、`socket`、`sqlite3`）。
- 端口按业务设计而不是按外部 API 设计；新增端口对应真实 IO 边界。
- 应用层不写业务规则；一个事务只修改一个聚合。

## 已知的历史违规

> 记录现存不符合上述规则的代码，避免被模仿。只在本次需求本来就要改到的代码里顺带修正；超出这个范围的不动，在汇报中列出。没有则保持为空。

- （无）
