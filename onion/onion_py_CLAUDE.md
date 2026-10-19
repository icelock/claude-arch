# CLAUDE.md

## 架构声明

本项目采用**洋葱架构**：从内到外分为**领域层（domain）→ 应用层（application）→ 外层（adapter + bootstrap）**，依赖只能由外向内。

```
外层：adapter.inbound（入站）、adapter.outbound（出站）、bootstrap（组装）
  └── 应用层：用例编排、事务边界、出站端口 Protocol
        └── 领域层：实体、值对象、领域服务、领域事件
依赖只能由外向内
```

六边形架构和整洁架构与本项目是同一条依赖规则的不同命名：六边形强调入站/出站端口对称，整洁架构强调用例输入/输出端口。**本项目统一按洋葱落法，不要引入第四种命名、分层或目录结构。**

包名用 `inbound`/`outbound`，不用 `in`/`out`（`in` 是 Python 关键字）。

## 对 AI 的硬性约束

本节是 MUST，优先于你的默认习惯和所谓的「最佳实践」。

1. **先定位再写**：动手前确定代码属于哪一层、哪个包。落不进「本项目的层级」表就停下来问，不要自创包。
2. **先读后写**：找同层 1–2 个现有实现，沿用其结构、命名、异常处理和日志方式。不要引入仓库里没有的新风格。
3. **最小改动**：只改与需求相关的代码。不顺手重构、不重排 import、不升级依赖、不改 ruff/format 配置。
4. **抽象准入**：没有第二个实现、不属于真实 IO 边界时，不建 Protocol、ABC、工厂、泛型仓储、兼容层、fallback、feature flag。三行重复优于一个错误抽象。
5. **依赖准入**：不新增第三方库。确需新增时，先说明用途、体积、维护状态，等用户确认后再改 `pyproject.toml`。
6. **不写占位**：不留 TODO、mock 返回值、假数据、注释掉的代码、未使用的参数或字段。
7. **验证真实**：完成前运行本节列出的全部检查，贴出命令和真实输出。失败就说失败并继续修，禁止改断言、加 skip、放宽超时、更新快照或放宽 import-linter 规则来让结果变绿。
8. **必须停下来问**：层级归属不明；新增包或层；新增依赖；修改公共类型、端口签名、对外接口、表结构、消息格式；需要放宽架构检查；修复历史违规会显著扩大改动范围。

## 分层与依赖

### 各层职责

| 层 | 职责 | 典型内容 |
|---|---|---|
| 领域层 | 业务概念和业务规则，不依赖框架和 IO | 实体、值对象、领域服务、领域事件、领域异常 |
| 应用层 | 编排用例：调用领域对象和端口，控制事务；定义出站端口 | 应用服务、端口 Protocol、命令/结果类型、应用异常 |
| 外层·入站适配器 | 接收外部触发，转换为内层类型后调用应用层 | FastAPI 路由、WS 推送接收、MQ 消费、定时任务、CLI |
| 外层·出站适配器 | 实现应用层端口，访问外部系统 | 交易所下单客户端、ORM 仓储、Redis、Kafka 发布 |
| 外层·组装 | 创建对象、注入依赖、启停任务，只做接线 | `myapp/bootstrap.py`、`myapp/main.py` |

不设单独的 common 层：业务通用类型放领域层；与业务无关的纯函数放 `myapp.domain.shared`，不得依赖 IO。

### 本项目的层级

> 下表为示例，按实际项目修改。新增包、拆分适配器时，同步更新本表和 import-linter 配置。

| 层 | 包 | 可以依赖 | 外部依赖限制 |
|---|---|---|---|
| 领域层 | `myapp.domain` | 无 | 只允许标准库 |
| 应用层 | `myapp.application`（端口在 `application.ports`） | domain | 只允许 domain、标准库、`asyncio`、`typing` |
| 入站适配器 | `myapp.adapter.inbound`（如 `inbound.api`、`inbound.ws`） | application、domain | 不依赖 `adapter.outbound` |
| 出站适配器 | `myapp.adapter.outbound`（如 `outbound.binance`、`outbound.db`） | application、domain | 不依赖 `adapter.inbound` |
| 组装 | `myapp.bootstrap`、`myapp.main` | 所有包 | 无 |

### 依赖规则

- **依赖只能由外向内**：domain 不 import 任何项目包；application 只 import domain；适配器 import application 和 domain；bootstrap import 所有包。
- **适配器之间互不依赖**：`adapter.inbound` 不 import `adapter.outbound`，不同出站适配器之间也不互相 import。只读查询也走应用层的查询服务，不得由入站适配器直连出站查询。
- **内层保持纯净**：领域层只用标准库（`dataclasses`、`decimal`、`enum`），不 import pydantic、sqlalchemy、redis、httpx、fastapi；应用层只依赖 domain 和标准库（包括 `asyncio`、`typing`），不做 IO。
- 所有依赖在 bootstrap 通过构造函数传入；禁止函数内部延迟 import、`importlib.import_module`、模块级全局变量或注册表保存服务对象。FastAPI 的 `Depends` 只在入站适配器使用。
- 跨层的公开函数和类必须写完整类型注解。

### 新代码放哪里

- 新的业务概念、业务规则 → domain。
- 新的业务流程 → application 的应用服务；需要外部能力时在 `ports` 新增 Protocol。
- 新的外部系统接入 → `adapter.outbound`，实现端口。
- 新的触发方式 → `adapter.inbound`，只做转换和调用；新增包时先更新层级表和 import-linter 配置，再写代码。

## 端口与跨边界类型

- **出站端口定义在应用层**的 `ports` 模块，使用 `typing.Protocol`。出站适配器实现对应方法，不需要继承。只有领域服务本身需要调用的端口才放领域层。
- 入站端口默认不定义，入站适配器直接调用应用服务；只有需要多种实现时才定义。端口按业务需要设计（`submit(order: HedgeOrder)`），不按外部系统 API 设计。
- **跨边界的类型归内层所有，转换由外层负责**：入站适配器把 HTTP/WS/MQ 报文转换为应用命令或领域类型；出站适配器把领域类型转换为交易所请求、ORM 模型、Kafka 消息，返回时再转回内层类型。
- pydantic 模型、ORM 模型、交易所报文 dict、Kafka 消息体只能出现在适配器中；领域类型使用 `@dataclass(frozen=True)` 或普通类，不使用 pydantic。

```python
# myapp/domain：业务规则，不依赖框架
@dataclass(frozen=True)
class Symbol:
    code: str
    tick_size: Decimal
    qty_scale: int

    def align_price(self, price: Decimal, side: Side) -> Decimal: ...

@dataclass(frozen=True)
class HedgeOrder:
    @classmethod
    def from_trade(cls, trade: Trade, symbol: Symbol) -> "HedgeOrder": ...
    def placed(self, ack: OrderAck) -> HedgeOrderPlaced: ...

# myapp/application/ports.py：出站端口
class HedgeOrderGateway(Protocol):
    async def submit(self, order: HedgeOrder) -> OrderAck: ...

# myapp/application/hedge_service.py：只编排，不写业务计算
class HedgeService:
    def __init__(self, symbols: SymbolRepository, gateway: HedgeOrderGateway, publisher: HedgeEventPublisher) -> None:
        self._symbols, self._gateway, self._publisher = symbols, gateway, publisher

    async def on_trade(self, trade: Trade) -> None:
        symbol = await self._symbols.get(trade.symbol)
        order = HedgeOrder.from_trade(trade, symbol)
        ack = await self._gateway.submit(order)
        await self._publisher.publish(order.placed(ack))

# myapp/adapter/outbound/binance.py：实现端口，在这里转换外部格式
class BinanceOrderGateway:
    async def submit(self, order: HedgeOrder) -> OrderAck:
        return to_ack(await self._client.place(to_request(order)))

# myapp/adapter/inbound/ws.py：转换后调用应用层
class TradeWsReceiver:
    def __init__(self, service: HedgeService) -> None: self._service = service

    async def on_message(self, raw: str) -> None:
        await self._service.on_trade(to_trade(parse(raw)))
```

### 层级内部规则

- 领域层：规则优先写在实体和值对象中；跨实体规则写领域服务。不直接取时间、随机数、全局 ID，`now` 通过参数传入；金额用 `Decimal`；必须有纯 pytest 单元测试。
- 应用层：只做加载、调用领域对象、调用端口、发布事件、控制事务；不写价格计算和条件分支业务规则。幂等、去重、并发控制属于应用层，具体实现通过端口交给外层。
- 外层：入站只转换和调用；出站只处理协议、签名、序列化、连接、重连、持久化，不做业务判断。回调运行在接收循环中时，耗时处理必须交给其他任务。

## 数据、时间与一致性

- 价格、数量、金额统一使用 `decimal.Decimal`，禁止 `float`；同一模块不得混用两种表示。
- 从字符串或整数构造：`Decimal("0.1")`，禁止 `Decimal(0.1)`；序列化为字符串，不用 float。
- 舍入必须显式：`quantize(Decimal("0.01"), rounding=ROUND_HALF_UP)`；不要修改全局 decimal context。入库前统一 scale，同一字段在不同路径使用不同 scale 视为错误。
- 下游写入使用确定性幂等键并加唯一约束；消息流必须处理重复、乱序、gap、断线重连：gap 时作废缓存并重拉快照，重连后重订阅。
- 领域层不调用 `datetime.now()`/`datetime.utcnow()`：`now` 作为参数传入；应用层需要时可注入 Clock，但时钟属于应用层。时间带 UTC 时区，延迟用 `time.monotonic()`。

## 并发与性能

- asyncio 代码中禁止阻塞调用：`time.sleep`、`requests`、同步数据库驱动、大量 CPU 计算；用 `asyncio.sleep`、`httpx.AsyncClient`、`asyncio.to_thread` 或进程池。
- 后台任务必须保存引用或用 `asyncio.TaskGroup()` 管理；禁止 `create_task` 后丢弃引用；所有任务必须能取消并正确退出。
- 禁止吞掉 `asyncio.CancelledError`；清理逻辑放 `finally`，取消要继续向上传播。
- 优先使用有界 `asyncio.Queue(maxsize=...)`；行情类可以丢弃并置 gap 标记，订单、回报、成交类不能丢。
- 线程场景用有界 `ThreadPoolExecutor`，线程池必须有名字和关闭流程。
- async 代码用 `asyncio.Lock`，不要在事件循环里用阻塞的 `threading.Lock`；锁内不做 IO。
- 热路径禁止逐笔日志、`json.dumps`、f-string 拼接、反射/inspect、无界重试。
- 多线程共享状态用锁保护或改为消息传递；禁止模块级可变字典作为缓存。

## 错误处理与事务

- 领域异常定义在领域层，应用异常和端口异常定义在应用层；出站适配器把第三方异常转换为端口约定的异常，使用 `raise ... from e` 保留异常链；入站适配器统一把内层异常转换为响应。
- 禁止裸 `except:`、`except Exception: pass`、捕获后只 `logger.exception` 继续执行。
- 失败必须明确重试（有上限、退避、抖动）、进入死信、熔断、返回错误或停机中的一种，不能只记日志继续。
- 事务边界在应用层，通过端口（如 `UnitOfWork` Protocol）控制；入站适配器不管理事务，出站适配器不自开事务。
- 事务内不做远程调用（交易所、HTTP、MQ 同步发送）；数据库与消息的一致性用 outbox 或提交后发送，不做双写。
- 捕获异常时保留 trace_id/order_id 等上下文；不要用 `str(e)` 替代异常链。

## 测试与验证

- 领域计算、状态流转必须有 pytest 单元测试，不启动事件循环和外部服务，覆盖正常、边界、非法输入。
- 应用层测试使用端口的内存实现或 fake，不连接真实数据库、Redis、交易所；不要 mock 被测对象自身。
- 适配器用集成测试或契约测试验证协议、序列化、重连；不要 mock 掉被测的转换逻辑。
- 测试必须确定性：用固定时钟或 `freezegun`；禁止 `time.sleep`/`asyncio.sleep` 等待异步结果，用 `asyncio.wait_for` 或事件。
- 禁止为了变绿修改断言、删除用例、加 `pytest.mark.skip`、放宽超时；测试只加严不放宽。覆盖率不是目标，但核心业务分支必须有测试。

## AI 常见劣化模式

| 劣化写法 | 正确做法 |
|---|---|
| domain import pydantic/sqlalchemy/fastapi | 领域层只用标准库 |
| 入站适配器直接调用出站适配器或查询模块 | 走应用服务 + 端口 |
| 把类型挪到 `domain.shared` 让各层都能用 | 类型归真正拥有它的层，边界做转换 |
| 单实现也建 Protocol + Factory + ABC | 直接用具体类，出现第二实现再抽象 |
| 加 `if settings.use_new: ... else: ...` 兼容层 | 删旧路径，保持单一路径 |
| 改断言、加 skip、放宽超时让测试变绿 | 修代码；测试只加严 |
| 只跑单个测试就宣称全部通过 | 跑完整检查并贴输出 |
| `except Exception: logger.exception(...)` 后继续 | 转换抛出，或明确重试/死信/熔断 |
| `float` 金额、`Decimal(0.1)`、依赖默认舍入 | `Decimal(str)`、显式 `quantize` + rounding |
| async 里 `time.sleep`、`requests`、同步 DB | `asyncio.sleep`、异步客户端、`to_thread` |

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
include_external_packages = true

[[tool.importlinter.contracts]]
name = "洋葱依赖：只能由外向内"
type = "layers"
layers = ["myapp.bootstrap | myapp.main", "myapp.adapter", "myapp.application", "myapp.domain"]

[[tool.importlinter.contracts]]
name = "内层不依赖框架和中间件"
type = "forbidden"
source_modules = ["myapp.domain", "myapp.application"]
forbidden_modules = ["pydantic", "fastapi", "sqlalchemy", "redis", "httpx", "requests", "websockets", "aiokafka"]
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
