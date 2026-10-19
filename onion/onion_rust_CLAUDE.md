# CLAUDE.md

## 应用架构：洋葱架构（三层）

本项目采用洋葱架构，从内到外分为三层：**领域层（domain）→ 应用层（application）→ 外层（adapter + app）**。依赖只能由外向内。crate 和模块名称可以按业务调整，但必须遵守下面的依赖规则。新增或修改代码前，先确认代码属于哪一层。

```
┌──────────────────── 外层 ─────────────────────┐
│ 入站适配器       出站适配器        app（组装）    │
│   ┌────────────── 应用层 ──────────────┐       │
│   │ 用例编排、出站端口 trait             │       │
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
| 领域层 | 业务概念和业务规则，不依赖异步运行时和 IO | 实体、值对象、领域服务、领域事件、领域错误 |
| 应用层 | 编排用例：调用领域对象和端口；定义出站端口 trait | 应用服务、端口 trait、命令/结果类型、应用错误 |
| 外层·入站适配器 | 接收外部触发，转换为内层类型后调用应用层 | HTTP handler、WS 推送接收、MQ 消费、定时任务、CLI |
| 外层·出站适配器 | 实现应用层的端口 trait，访问外部系统 | 交易所下单客户端、数据库仓储、Redis、Kafka 发布 |
| 外层·组装 | 创建对象、注入依赖、启动和关闭任务，只做接线 | `crates/app/src/main.rs` |

不设单独的 common 层：业务通用类型放在领域层；与业务无关的纯工具函数放在领域层的 `shared` 模块中，不得依赖 IO 相关 crate。

### 本项目的层级

> 新增 crate、拆分适配器时，同步更新本表。下表为示例，按实际项目修改。

| 层 | crate | 可以依赖 |
|---|---|---|
| 领域层 | `crates/domain` | 无（只允许 `rust_decimal`、`thiserror`、`chrono` 等基础库） |
| 应用层 | `crates/application` | domain；`async-trait`、tokio 的同步原语（`sync`） |
| 入站适配器 | `crates/adapter-ws`、`crates/adapter-http` | application、domain |
| 出站适配器 | `crates/adapter-binance`、`crates/adapter-kafka`、`crates/adapter-db` | application、domain |
| 组装 | `crates/app`（bin） | 所有 crate |

### 依赖规则

- **依赖只能由外向内**：domain 不依赖任何项目 crate；application 只依赖 domain；适配器依赖 application 和 domain；app 依赖所有 crate。各 crate 的 `Cargo.toml` 必须符合上表，由编译器保证。
- **适配器之间互不依赖**：入站适配器只调用应用层，不能直接调用出站适配器；出站适配器之间也不互相依赖。
- **领域层保持纯净**：不依赖 tokio、reqwest、sqlx、redis、rdkafka、serde 等；领域类型不 derive `Serialize`/`Deserialize`，序列化格式由适配器决定。
- **应用层不做 IO**：可以使用 `tokio::sync` 中的 channel、锁等同步原语，但不能依赖网络、数据库、文件相关 crate。
- 禁止通过 `static`、`OnceLock`、`lazy_static` 保存服务对象来绕过依赖规则；所有依赖在 app 中通过构造函数传入。

### 端口与跨边界类型

- **出站端口 trait 定义在应用层**的 `port` 模块，由出站适配器 crate 为自己的类型实现。只有领域服务本身需要调用的端口才放在领域层。
- 端口按业务需要设计（`HedgeOrderGateway::submit(&HedgeOrder)`），不能按外部系统的 API 设计。
- **跨边界的类型归内层所有，转换由外层负责**：
  - 入站：适配器把 HTTP 请求、WS 报文、MQ 消息转换为领域类型或应用命令，再调用应用层。
  - 出站：适配器把领域类型转换为交易所请求、数据库行、Kafka 消息，返回时再转换回内层类型。
- 交易所报文、数据库行结构、Kafka 消息体只能出现在适配器 crate 中。
- 端口 trait 需要 `Send + Sync`。默认使用 `#[async_trait]` 加 `Arc<dyn Trait>`；对性能敏感的路径可以改用泛型参数。

```rust
// ===== crates/domain：业务规则，不依赖 IO =====
pub struct Symbol { pub code: String, pub tick_size: Decimal, pub qty_scale: u32 }

impl Symbol {
    pub fn align_price(&self, price: Decimal, side: Side) -> Decimal { /* 按 tick_size 取整 */ }
}

pub struct HedgeOrder { /* ... */ }

impl HedgeOrder {
    pub fn from_trade(trade: &Trade, symbol: &Symbol) -> Result<Self, DomainError> { /* ... */ }
    pub fn placed(&self, ack: OrderAck) -> HedgeOrderPlaced { /* 生成领域事件 */ }
}

// ===== crates/application/src/port.rs：出站端口 =====
#[async_trait]
pub trait HedgeOrderGateway: Send + Sync {
    async fn submit(&self, order: &HedgeOrder) -> Result<OrderAck, PortError>;
}

#[async_trait]
pub trait HedgeEventPublisher: Send + Sync {
    async fn publish(&self, event: HedgeOrderPlaced) -> Result<(), PortError>;
}

// ===== crates/application：用例编排，不写业务计算 =====
pub struct HedgeService {
    symbols: Arc<dyn SymbolRepository>,
    gateway: Arc<dyn HedgeOrderGateway>,
    publisher: Arc<dyn HedgeEventPublisher>,
}

impl HedgeService {
    pub async fn on_trade(&self, trade: Trade) -> Result<(), AppError> {
        let symbol = self.symbols.get(&trade.symbol).await?;
        let order = HedgeOrder::from_trade(&trade, &symbol)?;
        let ack = self.gateway.submit(&order).await?;
        self.publisher.publish(order.placed(ack)).await?;
        Ok(())
    }

    pub async fn run(self: Arc<Self>, mut trades: mpsc::Receiver<Trade>) {
        while let Some(trade) = trades.recv().await {
            if let Err(e) = self.on_trade(trade).await {
                tracing::error!(error = ?e, "处理成交失败");
            }
        }
    }
}

// ===== crates/adapter-binance：实现端口，在这里做外部格式转换 =====
#[async_trait]
impl HedgeOrderGateway for BinanceOrderGateway {
    async fn submit(&self, order: &HedgeOrder) -> Result<OrderAck, PortError> {
        let resp = self.client.place(to_request(order)).await.map_err(to_port_error)?;
        Ok(to_ack(resp))
    }
}

// ===== crates/app/src/main.rs：组装 =====
let symbols = Arc::new(DbSymbolRepository::new(pool.clone()));
let gateway = Arc::new(BinanceOrderGateway::new(cfg.binance));
let publisher = Arc::new(KafkaEventPublisher::new(cfg.kafka));
let hedge = Arc::new(HedgeService::new(symbols, gateway, publisher));

let (tx, rx) = tokio::sync::mpsc::channel::<Trade>(10_000);
tokio::spawn(TradeWsReceiver::new(cfg.ws, tx).run()); // 入站适配器：报文转为 Trade 后发送
tokio::spawn(hedge.run(rx));
```

### 领域层规则

- 业务规则优先写在实体和值对象的方法中；跨多个实体的规则写成领域服务（普通函数或无状态结构体）。
- 用类型表达约束：用 newtype 区分 `Price`、`Qty`、`AccountId` 等，用枚举表达状态，非法状态尽量不可构造。
- 不直接获取当前时间、随机数、全局 ID：通过参数传入，或在应用层定义 `Clock`、`IdGenerator` 端口。
- 金额、价格、数量使用 `rust_decimal::Decimal`，不使用 `f64`。
- 领域层必须有单元测试，覆盖核心计算和状态流转；这些测试不需要异步运行时。

### 应用层规则

- 只做编排：加载数据、调用领域对象、调用端口、发布事件。不写价格计算、条件分支判断等业务规则。
- 幂等、去重、并发控制属于应用层，具体实现（如 Redis 去重）通过端口交给外层。
- 应用服务的参数和返回值使用领域类型或应用层定义的命令/结果类型。
- 应用层测试使用端口的内存实现或 mock，不连接真实外部系统。

### 外层规则

- 入站适配器只做转换和调用，不写业务逻辑；持续推送通过 channel 交给应用层，不在接收循环中直接处理。
- 出站适配器只处理技术问题（协议、签名、序列化、连接、重连、持久化），不做业务判断。
- 不在 `.await` 期间持有 `std::sync::Mutex`；CPU 密集计算使用 `spawn_blocking`。
- 有界 channel 需要明确满了以后的策略（等待、丢弃、告警），并写在代码注释中。
- 所有后台任务都必须能通过取消信号退出，app 负责等待它们结束。
- **只读查询例外**：报表、列表等不涉及业务规则的只读场景，可以由入站适配器调用外层的查询模块直接返回 DTO，不经过领域层。

### 错误处理

- 领域层定义 `DomainError`，应用层定义 `AppError` 和端口使用的 `PortError`，均使用 `thiserror`。
- 出站适配器把第三方错误（`reqwest::Error`、`sqlx::Error` 等）转换为 `PortError`，不直接暴露给内层。
- 入站适配器把 `AppError` 转换为响应或日志；`anyhow` 只在 app 和入站适配器中使用。

### 新代码放哪里

- 新的业务概念、业务规则 → domain。
- 新的业务流程（收到什么、做什么、通知谁）→ application 的应用服务；需要外部能力时在 `port` 中新增 trait。
- 新的外部系统接入（交易所、存储、消息）→ 新建或扩展出站适配器 crate，实现端口 trait。
- 新的触发方式（新接口、新定时任务、新消费者、新的 WS 推送接收）→ 入站适配器 crate。
- 新增 crate：先更新「本项目的层级」表，确认 `Cargo.toml` 的依赖符合规则，再写代码。

### 检查方式

- `cargo tree -p domain -e normal`：只应出现允许的基础库，不应出现任何项目 crate 和 IO 相关 crate。
- `cargo tree -p application -e normal`：项目 crate 只应出现 domain。
- 适配器 crate 的 `Cargo.toml` 中不应出现其他适配器 crate。
- 提交前运行 `cargo clippy --all-targets` 和 `cargo test`。

### 已知的历史违规

> 记录现存不符合上述规则的代码，避免被模仿；修改相关代码时顺手修正。没有则保持为空。

- （无）
