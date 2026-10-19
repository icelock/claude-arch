# CLAUDE.md

## 架构声明

本项目采用**洋葱架构**：从内到外分为**领域层（domain）→ 应用层（application）→ 外层（adapter + app）**，依赖只能由外向内。

```
外层：入站适配器、出站适配器、app（组装）
  └── 应用层：用例编排、事务边界、出站端口 trait
        └── 领域层：实体、值对象、领域服务、领域事件
依赖只能由外向内
```

六边形架构和整洁架构与本项目是同一条依赖规则的不同命名：六边形强调入站/出站端口对称，整洁架构强调用例输入/输出端口。**本项目统一按洋葱落法，不要引入第四种命名、分层或 crate 结构。**

## 对 AI 的硬性约束

本节是 MUST，优先于你的默认习惯和所谓的「最佳实践」。

1. **先定位再写**：动手前确定代码属于哪一层、哪个 crate 或模块。落不进「本项目的层级」表就停下来问，不要自创 crate。
2. **先读后写**：找同层 1–2 个现有实现，沿用其结构、命名、错误处理和日志方式。不要引入仓库里没有的新风格。
3. **最小改动**：只改与需求相关的代码。不顺手重构、不重排 `use`、不升级依赖、不改 rustfmt/clippy 配置。
4. **抽象准入**：没有第二个实现、不属于真实 IO 边界时，不建端口 trait、工厂、泛型仓储、兼容层、fallback、feature flag。三行重复优于一个错误抽象。
5. **依赖准入**：不新增 crate。确需新增时，先说明用途、体积、维护状态和 license，等用户确认后再改 `Cargo.toml`。
6. **不写占位**：不留 TODO、`todo!()`、`unimplemented!()`、mock 返回值、假数据、注释掉的代码、未使用的参数。
7. **验证真实**：完成前运行本节列出的全部检查，贴出命令和真实输出。失败就说失败并继续修，禁止 `#[ignore]`、放宽 clippy、改断言或删除测试来让结果变绿。
8. **必须停下来问**：层级归属不明；新增 crate 或模块；新增依赖；修改公共类型、端口签名、对外接口、消息格式、表结构；需要放宽架构检查；修复历史违规会显著扩大改动范围。

## 分层与依赖

### 各层职责

| 层 | 职责 | 典型内容 |
|---|---|---|
| 领域层 | 业务概念和业务规则，不依赖异步运行时和 IO | 实体、值对象、领域服务、领域事件、领域错误 |
| 应用层 | 编排用例：调用领域对象和端口，控制事务；定义出站端口 | 应用服务、端口 trait、命令/结果类型、应用错误 |
| 外层·入站适配器 | 接收外部触发，转换为内层类型后调用应用层 | HTTP handler、WS 推送接收、MQ 消费、定时任务 |
| 外层·出站适配器 | 实现应用层端口，访问外部系统 | 交易所下单客户端、数据库仓储、Redis、Kafka 发布 |
| 外层·组装 | 创建对象、注入依赖、启停任务，只做接线 | `crates/app/src/main.rs` |

不设单独的 common 层：业务通用类型放领域层；与业务无关的纯函数放 `domain::shared`，不得依赖 IO 相关 crate。

### 本项目的层级

> 下表为示例，按实际项目修改。新增 crate、拆分适配器时，同步更新本表。

| 层 | crate | 可以依赖 | 外部依赖限制 |
|---|---|---|---|
| 领域层 | `crates/domain` | 无 | 只允许 `rust_decimal`、`thiserror`、`chrono` |
| 应用层 | `crates/application` | domain | 可用 `async-trait`、`tokio::sync`；禁网络/DB/文件 crate |
| 入站适配器 | `crates/adapter-ws`、`crates/adapter-http` | application、domain | 不依赖出站适配器 crate |
| 出站适配器 | `crates/adapter-binance`、`crates/adapter-db` | application、domain | 不依赖入站适配器 crate |
| 组装 | `crates/app`（bin） | 所有 crate | 无 |

### 依赖规则

- **依赖只能由外向内**：domain 不依赖任何项目 crate；application 只依赖 domain；适配器依赖 application 和 domain；app 依赖所有 crate。各 crate 的 `Cargo.toml` 必须符合上表，由编译器保证。
- **适配器之间互不依赖**：入站适配器只调用应用层，不能直接调用出站适配器或查询模块；出站适配器之间也不互相依赖。只读查询走应用层的查询服务。
- **领域层保持纯净**：不依赖 tokio、reqwest、sqlx、redis、rdkafka、serde；领域类型不 derive `Serialize`/`Deserialize`，序列化格式由适配器决定。
- **应用层不做 IO**：可以使用 `tokio::sync` 的 channel、锁等同步原语，但不能依赖网络、数据库、文件相关 crate。
- 禁止通过 `static`、`OnceLock`、`lazy_static`、`thread_local!` 保存服务对象或做服务定位；所有依赖在 app 通过构造函数传入。
- 所有跨边界调用必须有明确的超时；禁止无超时地调用外部系统。

### 新代码放哪里

- 新的业务概念、业务规则 → domain。
- 新的业务流程 → application 的应用服务；需要外部能力时在 `port` 新增 trait。
- 新的外部系统接入 → 新建或扩展出站适配器 crate，实现端口 trait。
- 新的触发方式 → 入站适配器 crate，只做转换和调用。
- 新增 crate：先更新层级表，确认 `Cargo.toml` 依赖符合规则，再写代码。

## 端口与跨边界类型

- **出站端口 trait 定义在应用层**的 `port` 模块，由出站适配器 crate 为自己的类型实现。只有领域服务本身需要调用的端口才放领域层。
- 入站端口默认不定义，入站适配器直接调用应用服务；只有需要多种实现时才定义。
- 端口按业务需要设计（`HedgeOrderGateway::submit(&HedgeOrder)`），不按外部系统 API 设计。
- **跨边界的类型归内层所有，转换由外层负责**：入站适配器把 HTTP/WS/MQ 报文转换为领域类型或应用命令；出站适配器把领域类型转换为交易所请求、数据库行、Kafka 消息，返回时再转回内层类型。
- 交易所报文、数据库行结构、Kafka 消息体只能出现在适配器 crate 中。
- 端口 trait 需要 `Send + Sync`。边界默认用 `#[async_trait]` + `Arc<dyn Trait>`；热路径改用泛型参数，避免动态分发。

```rust
// crates/domain：业务规则，不依赖 IO
pub struct HedgeOrder { /* ... */ }

impl HedgeOrder {
    pub fn from_trade(trade: &Trade, symbol: &Symbol) -> Result<Self, DomainError> { /* ... */ }
    pub fn placed(&self, ack: OrderAck) -> HedgeOrderPlaced { /* ... */ }
}

// crates/application/src/port.rs：出站端口
#[async_trait]
pub trait HedgeOrderGateway: Send + Sync {
    async fn submit(&self, order: &HedgeOrder) -> Result<OrderAck, PortError>;
}

// crates/application：只编排，不写业务计算
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
}

// crates/adapter-binance：实现端口，在这里转换外部格式
#[async_trait]
impl HedgeOrderGateway for BinanceOrderGateway {
    async fn submit(&self, order: &HedgeOrder) -> Result<OrderAck, PortError> {
        let resp = self.client.place(to_request(order)).await.map_err(to_port_error)?;
        Ok(to_ack(resp))
    }
}
```

### 层级内部规则

- 领域层：规则优先写在实体和值对象中；跨实体规则写领域服务。用 newtype 区分价格、数量、账户 ID；不直接取时间、随机数、全局 ID，`now` 通过参数传入；金额用 `Decimal`；必须有纯单元测试。
- 应用层：只做加载、调用领域对象、调用端口、发布事件、控制事务；不写价格计算和条件分支业务规则。幂等、去重、并发控制属于应用层，具体实现通过端口交给外层。
- 外层：入站只转换和调用；出站只处理协议、签名、序列化、连接、重连、持久化，不做业务判断；回调线程不做阻塞操作。

## 数据、时间与一致性

- 价格、数量、金额统一使用 `rust_decimal::Decimal`，禁止 `f64`/`f32`；同一模块不得混用两种表示。
- 用 `checked_add`/`checked_sub`/`checked_mul`/`checked_div`，溢出返回错误；禁止用 `as` 做金额截断。
- 舍入必须显式：`round_dp_with_strategy`，策略按业务选择；禁止 `Decimal::from_f64` 后忽略 `None`；从字符串或整数构造，序列化为字符串。
- 下游写入使用确定性幂等键并加唯一约束；消息流必须处理重复、乱序、gap、断线重连：gap 时作废缓存并重拉快照，重连后重订阅。
- 领域层不调用 `Utc::now()`/`SystemTime::now()`：`now` 作为参数传入；应用层需要时可注入 Clock，但时钟属于应用层。延迟和超时用 `Instant`（单调时钟）。

## 并发与性能

- async 代码中禁止阻塞：`std::thread::sleep`、`block_on`、同步文件/数据库 IO、长时间 CPU 计算；用 `tokio::time::sleep`、异步客户端、`spawn_blocking`。
- 禁止在 `.await` 期间持有 `std::sync::MutexGuard`；需要跨 await 时用 `tokio::sync::Mutex`，或改为消息传递。
- 每个 `tokio::spawn` 必须保留 `JoinHandle` 或加入 `JoinSet`，关停时等待；禁止 detach 后不管。
- 任务必须能被取消：使用 `CancellationToken`、`select!` 或 drop 语义；取消时清理资源并向上传播。
- 优先使用有界 `mpsc::channel`；处理 `try_send` 的 `Full` 错误，不得静默丢弃。行情类可丢弃并置 gap 标记，订单/回报/成交类不能丢。
- `select!` 的分支必须考虑取消安全：被取消时不能丢失已读取的消息或半写入的状态。
- 锁内不做 IO、不 `.await`、不调用可能重入的回调；锁范围尽量短。
- 热路径禁止逐笔日志、`serde_json`、`format!`、`Arc<dyn Trait>` 动态分发、无界重试和不必要的 `clone`。

## 错误处理与事务

- 领域层用 `thiserror` 定义 `DomainError`，应用层定义 `AppError`；`anyhow` 只在 app 和入口边缘使用，不进入 domain/service 公开 API。
- 出站调用把第三方错误（`sqlx::Error`、`reqwest::Error`）转换为本层错误，用 `?` + `From` 或显式 `map_err`。
- 业务路径禁止 `unwrap()`/`expect()`/`panic!`；禁止用 `unwrap_or_default()` 吞掉错误。
- 失败必须明确重试（有上限、退避、抖动）、死信、熔断、返回错误或停机中的一种，不能只打日志继续。
- 事务边界在应用层，通过出站适配器的 repository/UoW 控制；入站适配器不开事务，出站适配器不自开事务。
- 事务内不做远程调用（交易所、HTTP、MQ 同步发送）；数据库与消息的一致性用 outbox 或提交后发送，不做双写。
- 只有启动阶段遇到无法恢复的配置错误才允许 `panic!`/`expect()`，错误信息必须包含配置项名称。

## 测试与验证

- 领域计算、状态流转必须有普通 `#[test]` 单元测试，不启动 tokio runtime，覆盖正常、边界、非法输入。
- 应用层测试用泛型参数、函数类型或 fake 替换 IO；不连接真实数据库、Redis、交易所；不要 mock 被测逻辑本身。
- async 测试用 `#[tokio::test]`；时间相关逻辑用 `tokio::time::pause/advance`，不要真实 sleep。
- 适配器用集成测试或契约测试验证协议、序列化、重连；不要 mock 掉被测的转换逻辑。
- 测试必须确定性：不依赖当前时间、随机数、真实网络、执行顺序；`now` 通过参数注入。
- 禁止为了变绿加 `#[ignore]`、修改断言、删除用例、放宽 clippy 或超时；测试只加严不放宽。覆盖率不是目标，但核心业务分支必须有测试。

## AI 常见劣化模式

| 劣化写法 | 正确做法 |
|---|---|
| domain 依赖 tokio/reqwest/sqlx/serde | 领域层保持纯净，序列化交给适配器 |
| 入站适配器直接调用出站适配器或查询模块 | 走应用服务 + 端口 |
| 把类型挪到 `domain::shared` 让各层都能用 | 类型归真正拥有它的层，边界做转换 |
| 单实现也建 trait + factory + `dyn` | 直接用具体类型，出现第二实现再抽象 |
| 加 `if use_new { ... } else { ... }` 兼容层 | 删旧路径，保持单一路径 |
| 加 `#[ignore]`、改断言让测试变绿 | 修代码；测试只加严 |
| 只跑单个测试就宣称全部通过 | 跑完整检查并贴输出 |
| 业务路径 `unwrap()`/`unwrap_or_default()` | 返回 `Result`，错误向上传递 |
| `f64` 金额、`as` 截断、`from_f64` 忽略 None | `rust_decimal` + checked 运算 + 显式舍入 |
| 持有 `std::sync::MutexGuard` 跨 `.await` | `tokio::sync::Mutex` 或消息传递 |

## 检查方式与完成定义

### 检查

- `cargo fmt --check`
- `cargo clippy --all-targets -- -D warnings`
- `cargo test`
- `cargo tree -p domain -e normal`：只应出现允许的基础库，不应出现任何项目 crate 和 IO crate。
- `cargo tree -p application -e normal`：项目 crate 只应出现 domain。
- 适配器 crate 的 `Cargo.toml` 中不应出现其他适配器 crate。
- 新增 crate 时，建议用编译期边界或 CI 脚本校验 `Cargo.toml`，不要只靠人工 review。

### 完成定义（DoD）

- [ ] 代码在正确的层和 crate，依赖方向正确，`cargo tree` 检查通过。
- [ ] `cargo fmt`、`clippy -D warnings`、`cargo test` 全部通过，并贴出真实输出。
- [ ] 新增业务分支有单元测试，覆盖失败路径。
- [ ] 没有 TODO、`todo!()`、注释掉的代码、调试日志、未使用的参数。
- [ ] 错误处理、日志上下文、超时和取消符合本文件。
- [ ] 没有新增依赖、trait、工厂或兼容层；如确有必要，已在回复中说明理由。

## 已知的历史违规

> 记录现存不符合上述规则的代码，避免被模仿；修改相关代码时顺手修正。没有则保持为空。

- （无）
