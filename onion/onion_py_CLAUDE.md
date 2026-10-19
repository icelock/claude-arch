# CLAUDE.md

## 应用架构：洋葱架构（三层）

本项目采用洋葱架构，从内到外分为三层：**领域层（domain）→ 应用层（application）→ 外层（adapter + bootstrap）**。依赖只能由外向内。包名可以按业务调整，但必须遵守下面的依赖规则。新增或修改代码前，先确认代码属于哪一层。

```
┌──────────────────── 外层 ─────────────────────┐
│ adapter.inbound   adapter.outbound   bootstrap │
│   ┌────────────── 应用层 ──────────────┐       │
│   │ 用例编排、出站端口 Protocol          │       │
│   │   ┌────────── 领域层 ──────────┐   │       │
│   │   │ 实体、值对象、领域服务、领域事件 │   │       │
│   │   └────────────────────────────┘   │       │
│   └────────────────────────────────────┘       │
└────────────────────────────────────────────────┘
                 依赖只能由外向内
```

### 各层职责

| 层 | 职责 | 典型内容 |
|---|---|---|
| 领域层 | 业务概念和业务规则，不依赖框架和 IO | 实体、值对象、领域服务、领域事件、领域异常 |
| 应用层 | 编排用例：调用领域对象和端口，控制事务边界；定义出站端口 | 应用服务、端口 Protocol、命令/结果类型、应用异常 |
| 外层·入站适配器 | 接收外部触发，转换为内层类型后调用应用层 | FastAPI 路由、WS 推送接收、MQ 消费、定时任务、CLI |
| 外层·出站适配器 | 实现应用层的端口，访问外部系统 | 交易所下单客户端、ORM 仓储、Redis、Kafka 发布 |
| 外层·组装 | 创建对象、注入依赖、启动和关闭任务，只做接线 | `myapp/bootstrap.py`、`myapp/main.py` |

不设单独的 common 层：业务通用类型放在领域层；与业务无关的纯工具函数放在 `myapp.domain.shared`，不得依赖 IO 相关的库。

包名使用 `inbound` / `outbound`，不使用 `in` / `out`（`in` 是 Python 关键字）。

### 本项目的层级

> 新增包、拆分适配器时，同步更新本表和 import-linter 配置。下表为示例，按实际项目修改。

| 层 | 包 | 可以依赖 |
|---|---|---|
| 领域层 | `myapp.domain` | 无（只允许标准库） |
| 应用层 | `myapp.application`（端口定义在 `myapp.application.ports`） | domain |
| 入站适配器 | `myapp.adapter.inbound`（如 `inbound.api`、`inbound.ws`、`inbound.jobs`） | application、domain |
| 出站适配器 | `myapp.adapter.outbound`（如 `outbound.binance`、`outbound.kafka`、`outbound.db`） | application、domain |
| 组装 | `myapp.bootstrap`、`myapp.main` | 所有包 |

### 依赖规则

- **依赖只能由外向内**：domain 不 import 任何项目包；application 只 import domain；适配器 import application 和 domain；bootstrap import 所有包。Python 不会阻止反向 import，必须用 import-linter 检查（见「检查方式」）。
- **适配器之间互不依赖**：`adapter.inbound` 不 import `adapter.outbound`；不同出站适配器之间也不互相 import。
- **领域层保持纯净**：只使用标准库（`dataclasses`、`decimal`、`enum` 等），不 import pydantic、sqlalchemy、redis、httpx、fastapi 等。
- **应用层不做 IO**：只依赖 domain 和标准库（包括 `asyncio`、`typing`）。
- 禁止用以下方式绕过依赖规则：
  - 在函数内部延迟 import 其他层的模块来规避循环 import。
  - 用模块级全局变量或注册表保存服务对象。
  - `importlib.import_module` 动态导入。
- 所有依赖在 bootstrap 中通过构造函数传入。FastAPI 的 `Depends` 只在入站适配器中使用。
- 跨层的公开函数和类必须写完整的类型注解。

### 端口与跨边界类型

- **出站端口定义在应用层**的 `ports` 模块，使用 `typing.Protocol`。出站适配器实现对应方法，不需要继承 Protocol。只有领域服务本身需要调用的端口才放在领域层。
- 端口按业务需要设计（`submit(order: HedgeOrder)`），不能按外部系统的 API 设计。
- **跨边界的类型归内层所有，转换由外层负责**：
  - 入站：适配器把 HTTP 请求（pydantic 模型）、WS 报文、MQ 消息转换为领域类型或应用命令，再调用应用层。
  - 出站：适配器把领域类型转换为交易所请求、ORM 模型、Kafka 消息，返回时再转换回内层类型。
- pydantic 模型、ORM 模型、交易所报文 dict、Kafka 消息体只能出现在适配器中。
- 领域类型使用 `@dataclass(frozen=True)` 或普通类，不使用 pydantic。

```python
# ===== myapp/domain：业务规则，不依赖框架 =====
@dataclass(frozen=True)
class Symbol:
    code: str
    tick_size: Decimal
    qty_scale: int

    def align_price(self, price: Decimal, side: Side) -> Decimal: ...  # 按 tick_size 取整


@dataclass(frozen=True)
class HedgeOrder:
    ...

    @classmethod
    def from_trade(cls, trade: Trade, symbol: Symbol) -> "HedgeOrder": ...

    def placed(self, ack: OrderAck) -> HedgeOrderPlaced: ...  # 生成领域事件


# ===== myapp/application/ports.py：出站端口 =====
class SymbolRepository(Protocol):
    async def get(self, code: str) -> Symbol: ...


class HedgeOrderGateway(Protocol):
    async def submit(self, order: HedgeOrder) -> OrderAck: ...


class HedgeEventPublisher(Protocol):
    async def publish(self, event: HedgeOrderPlaced) -> None: ...


# ===== myapp/application/hedge_service.py：用例编排，不写业务计算 =====
class HedgeService:
    def __init__(self, symbols: SymbolRepository, gateway: HedgeOrderGateway,
                 publisher: HedgeEventPublisher) -> None:
        self._symbols = symbols
        self._gateway = gateway
        self._publisher = publisher

    async def on_trade(self, trade: Trade) -> None:
        symbol = await self._symbols.get(trade.symbol)
        order = HedgeOrder.from_trade(trade, symbol)
        ack = await self._gateway.submit(order)
        await self._publisher.publish(order.placed(ack))

    async def consume(self, trades: asyncio.Queue[Trade]) -> None:
        while True:
            trade = await trades.get()
            try:
                await self.on_trade(trade)
            except Exception:
                logger.exception("处理成交失败 trade_id=%s", trade.trade_id)


# ===== myapp/adapter/outbound/binance.py：实现端口，在这里做外部格式转换 =====
class BinanceOrderGateway:
    async def submit(self, order: HedgeOrder) -> OrderAck:
        try:
            resp = await self._client.place(to_request(order))
        except httpx.HTTPError as e:
            raise GatewayError("binance place order failed") from e
        return to_ack(resp)


# ===== myapp/bootstrap.py：组装 =====
async def main() -> None:
    symbols = DbSymbolRepository(session_factory)
    gateway = BinanceOrderGateway(config.binance)
    publisher = KafkaEventPublisher(config.kafka)
    hedge = HedgeService(symbols, gateway, publisher)

    trades: asyncio.Queue[Trade] = asyncio.Queue(maxsize=10_000)
    ws_receiver = TradeWsReceiver(config.ws, trades)  # 入站适配器：报文转为 Trade 后放入队列

    async with asyncio.TaskGroup() as tg:
        tg.create_task(ws_receiver.run())
        tg.create_task(hedge.consume(trades))
```

### 领域层规则

- 业务规则优先写在实体和值对象的方法中；跨多个实体的规则写成领域服务（普通函数或无状态类）。
- 用类型表达约束：用 `Enum` 表达状态，用 `NewType` 或值对象区分 `Price`、`Qty`、`AccountId`；在构造时校验，避免创建非法对象。
- 不直接调用 `datetime.now()`、`random`、全局 ID 生成器：通过参数传入，或在应用层定义 `Clock`、`IdGenerator` 端口。
- 金额、价格、数量使用 `decimal.Decimal`，不使用 `float`。
- 领域层必须有 pytest 单元测试，覆盖核心计算和状态流转；这些测试不需要事件循环和外部服务。

### 应用层规则

- 只做编排：加载数据、调用领域对象、调用端口、发布事件、控制事务。不写价格计算、条件分支判断等业务规则。
- 事务边界由应用层通过端口（如 `UnitOfWork` Protocol）控制，具体实现在出站适配器。
- 幂等、去重、并发控制属于应用层，具体实现（如 Redis 去重）通过端口交给外层。
- 应用层测试使用端口的内存实现或 fake，不连接真实外部系统。

### 外层规则

- 入站适配器只做转换和调用，不写业务逻辑；持续推送通过队列交给应用层，不在接收循环中直接处理。
- 出站适配器只处理技术问题（协议、签名、序列化、连接、重连、持久化），不做业务判断。
- asyncio 代码中禁止调用阻塞操作（`time.sleep`、`requests`、同步数据库驱动、大量 CPU 计算）；必要时使用 `asyncio.to_thread` 或进程池。
- 有界队列需要明确满了以后的策略（等待、`put_nowait` 失败后丢弃、告警），并写在代码注释中。
- 所有后台任务都必须能被取消并正确退出，bootstrap 负责等待它们结束。
- **只读查询例外**：报表、列表等不涉及业务规则的只读场景，可以由入站适配器调用外层的查询模块直接返回 DTO，不经过领域层。

### 异常处理

- 领域异常定义在领域层，应用异常和端口异常定义在应用层。
- 出站适配器把第三方异常（`httpx.HTTPError`、`sqlalchemy.exc.SQLAlchemyError`、`redis.RedisError` 等）转换为端口约定的异常，使用 `raise ... from e` 保留异常链。
- 入站适配器统一把内层异常转换为响应。
- 不要使用裸 `except:` 或吞掉异常；必须捕获时记录日志并带上 trace_id 等上下文。

### 新代码放哪里

- 新的业务概念、业务规则 → domain。
- 新的业务流程（收到什么、做什么、通知谁）→ application 的应用服务；需要外部能力时在 `ports` 中新增 Protocol。
- 新的外部系统接入（交易所、存储、消息）→ `adapter.outbound` 下新建或扩展模块，实现端口。
- 新的触发方式（新接口、新定时任务、新消费者、新的 WS 推送接收）→ `adapter.inbound`。
- 新增包：先更新「本项目的层级」表和 import-linter 配置，再写代码。

### 检查方式

- 使用 import-linter 检查依赖（包名按「本项目的层级」表修改）：

```toml
# pyproject.toml
[tool.importlinter]
root_package = "myapp"
include_external_packages = true

[[tool.importlinter.contracts]]
name = "洋葱依赖：只能由外向内"
type = "layers"
layers = [
    "myapp.bootstrap | myapp.main",
    "myapp.adapter",
    "myapp.application",
    "myapp.domain",
]

[[tool.importlinter.contracts]]
name = "入站与出站适配器互相独立"
type = "independence"
modules = [
    "myapp.adapter.inbound",
    "myapp.adapter.outbound",
]

[[tool.importlinter.contracts]]
name = "内层不依赖框架和中间件"
type = "forbidden"
source_modules = ["myapp.domain", "myapp.application"]
forbidden_modules = [
    "pydantic", "fastapi", "sqlalchemy", "redis",
    "httpx", "requests", "websockets", "aiokafka",
]
```

- 运行 `lint-imports` 检查依赖；函数内部的延迟 import 同样会被检查到。
- 提交前运行 `ruff check`、`mypy`（或 `pyright`）、`lint-imports` 和 `pytest`。

### 已知的历史违规

> 记录现存不符合上述规则的代码，避免被模仿；修改相关代码时顺手修正。没有则保持为空。

- （无）
