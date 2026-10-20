# CLAUDE.md

## 架构声明

本项目采用**洋葱架构**：从内到外分为**领域层（domain）→ 应用层（application）→ 外层（adapter + app）**，依赖只能由外向内。

```
外层：入站适配器、出站适配器、app（组装）
  └── 应用层：用例编排、事务边界、出站端口 trait
        └── 领域层：实体、值对象、领域服务、领域事件
依赖只能由外向内
```

**组装层（`app`）可以依赖所有层**：它只做接线，是唯一同时知道所有具体实现的地方；任何层都不能反过来依赖组装层。

六边形架构和整洁架构与本项目是同一条依赖规则的不同命名：六边形强调入站/出站端口对称，整洁架构强调用例输入/输出端口。**本项目统一按洋葱落法，不要引入第四种命名、分层或 crate 结构。**

## 规则优先级与工作方式

规则冲突时按以下顺序取舍：**用户当次明确指令 > 正确性与安全 > 本文件的架构规则 > 仓库现有写法 > 通用最佳实践**。例外：安全类规则（密钥、注入、权限校验）即使用户要求放宽，也要先说明风险并得到确认。现有代码违反本文件时不要模仿，新代码按本文件写。

- 跨层改动或改动超过 3 个文件时，先用几行说明计划：落在哪层、改哪些文件、是否新增端口、是否涉及「必须停下来问」的事项，再动手。
- 修 bug 先写一个能复现问题的失败测试，再修代码，让它变绿。
- 小的歧义（命名、放在同层哪个模块、日志措辞）不要打断用户：选仓库惯例或最保守的做法，在回复中写明假设。
- 完成时按以下格式汇报：
  1. 改动文件及所属层；
  2. 关键决策和假设；
  3. 执行的检查命令和真实输出（失败也照贴）；
  4. 遗留问题和风险，包括发现但没有修的历史违规。

## 对 AI 的硬性约束

本节是 MUST，优先于你的默认习惯和所谓的「最佳实践」。

1. **先定位再写**：动手前确定代码属于哪一层、哪个 crate 和模块。落不进「本项目的层级」表就停下来问，不要自创 crate。
2. **先读后写**：找同层 1–2 个现有实现，沿用其结构、命名、错误处理和日志方式。不要引入仓库里没有的新风格。
3. **最小改动**：只改与需求相关的代码。不顺手重构、不重排 `use`、不升级依赖、不改 rustfmt/clippy 配置。
4. **抽象准入**：真实 IO 边界的出站端口 trait 是架构要求，必须建；一个外部能力对应一个端口，不按方法拆。除此之外，没有第二个实现时，不建 trait、工厂、泛型仓储、兼容层、fallback、feature flag；纯计算不建端口。三行重复优于一个错误抽象。
5. **依赖准入**：不新增 crate 依赖。确需新增时，先说明用途、体积、维护状态和 license，等用户确认后再改 `Cargo.toml`。
6. **不写占位**：不留 TODO、`todo!()`、`unimplemented!()`、mock 返回值、假数据、注释掉的代码、未使用的参数或字段。
7. **验证真实**：完成前运行「检查」一节列出的全部检查，贴出命令和真实输出。失败就说失败并继续修，禁止 `#[ignore]`、改断言、删除测试、放宽超时、放宽 clippy 检查来让结果变绿。
8. **必须停下来问**：层级归属不明；新增 crate 或层；新增依赖；修改公共类型、端口签名、对外接口、表结构、消息格式；修复历史违规会超出本次需求的改动范围。

## 分层与依赖

### 各层职责

| 层 | 职责 | 典型内容 |
|---|---|---|
| 领域层 | 业务概念和业务规则，不依赖异步运行时和 IO | 实体、值对象、领域服务、领域事件、领域错误 |
| 应用层 | 编排用例：加载、调用领域对象、调用端口、发布事件、控制事务；定义出站端口 | 应用服务、查询服务、端口 trait、命令/结果类型、应用错误 |
| 外层·入站适配器 | 接收外部触发，转换为内层类型后调用应用层 | HTTP handler、WS 推送接收、MQ 消费者、定时任务 |
| 外层·出站适配器 | 实现应用层端口，访问外部系统 | 第三方服务客户端、数据库仓储与事务、Redis、Kafka 发布 |
| 外层·组装 | 创建对象、注入依赖、加载配置、启停任务，只做接线 | `crates/app/src/main.rs` |

不设单独的 common 层：业务通用类型放领域层；与业务无关的纯函数放 `domain::shared`，不得依赖 IO 相关 crate。

### 本项目的层级

> 下表为示例，按实际项目修改。新增 crate、拆分适配器时，同步更新本表和「检查」一节的脚本。

| 层 | crate / 模块 | 可以依赖 | 外部依赖限制 |
|---|---|---|---|
| 领域层 | `crates/domain`（`model`、`service`、`shared`） | 无 | 只允许 `rust_decimal`、`thiserror`、`chrono` |
| 应用层 | `crates/application`（`service`、`query`、`port`） | domain | 可用 `async-trait`、`tokio::sync`；禁网络/DB/文件 crate |
| 入站适配器 | `crates/adapter-http`、`adapter-ws`、`adapter-mq-consumer`、`adapter-job` | application、domain | 不依赖出站适配器 crate |
| 出站适配器 | `crates/adapter-payment`、`adapter-db`、`adapter-mq-publisher` | application、domain | 不依赖入站适配器 crate |
| 组装 | `crates/app`（bin） | 所有 crate | 无 |

### 依赖规则

- **依赖只能由外向内**：domain 不依赖任何项目 crate；application 只依赖 domain；适配器依赖 application 和 domain；app 依赖所有 crate。各 crate 的 `Cargo.toml` 必须符合上表，由编译器保证。
- **适配器之间互不依赖**：入站适配器只调用应用层，不能直接调用出站适配器或 sqlx；出站适配器之间也不互相依赖。只读查询走应用层的查询服务。
- **领域层保持纯净**：不依赖 tokio、reqwest、sqlx、redis、rdkafka、serde；领域类型不 derive `Serialize`/`Deserialize`，序列化格式由适配器决定。
- **应用层不做 IO**：可以使用 `tokio::sync` 的 channel、锁等同步原语，但不能依赖网络、数据库、文件相关 crate。
- 禁止通过 `static`、`OnceLock`、`LazyLock`、`lazy_static`、`thread_local!` 保存服务对象或做服务定位；所有依赖在 app 通过构造函数传入。
- 依赖图不能有环；`Arc` 持有关系也不能成环。

### 新代码放哪里

- 新的业务概念、业务规则 → domain：规则优先写在实体和值对象中，跨实体的规则写领域服务。
- 新的业务流程 → application 的应用服务；需要外部能力时在 `port` 新增 trait。
- 新的外部系统接入 → 新建或扩展出站适配器 crate，实现端口 trait。
- 新的触发方式 → 入站适配器 crate，只做转换和调用。
- 新增 crate：先更新层级表和检查脚本，确认 `Cargo.toml` 依赖符合规则，再写代码。

## 端口与跨边界类型

- **出站端口 trait 定义在应用层**的 `port` 模块，由出站适配器 crate 为自己的类型实现。只有领域服务本身需要调用的端口才放领域层。
- 入站端口默认不定义，入站适配器直接调用应用服务；只有需要多种实现时才定义。
- 端口按业务需要设计（`PaymentGateway::charge(&Order)`），不按外部系统 API 设计（禁止 `charge(StripeChargeRequest)`）。
- **查询侧**：只读查询放 `application::query` 的查询服务，查询端口放 `port`，由 `adapter-db` 实现；可以直接返回只读视图结构，不必经过领域模型。入站适配器不得直接查库。
- **跨边界的类型归内层所有，转换由外层负责**：入站适配器把 HTTP/WS/MQ 报文转换为领域类型或应用命令；出站适配器把领域类型转换为第三方请求、数据库行、Kafka 消息，返回时再转回内层类型。第三方报文、数据库行结构、Kafka 消息体只能出现在适配器 crate 中。
- 端口 trait 需要 `Send + Sync`，默认用 `#[async_trait]` + `Arc<dyn Trait>`。

```rust
// crates/domain：业务规则，不依赖 IO 和异步运行时
pub struct Order { id: OrderId, status: OrderStatus, amount: Money /* , ... */ }

impl Order {
    pub fn place(cart: &Cart, now: DateTime<Utc>) -> Result<Self, DomainError> { /* 计算金额、校验约束 */ }
    pub fn start_payment(&mut self, now: DateTime<Utc>) -> Result<(), DomainError> { /* 只有「待支付」可以发起 */ }
    pub fn mark_paid(&mut self, receipt: PaymentReceipt, now: DateTime<Utc>) -> Result<OrderEvent, DomainError> { /* 只有「支付中」可以确认；生成领域事件 */ }
}

// crates/application/src/port.rs：出站端口，签名只用内层类型
#[async_trait]
pub trait PaymentGateway: Send + Sync {
    /// 实现方必须以订单号作为幂等键，重复调用不会重复扣款。
    async fn charge(&self, order: &Order) -> Result<PaymentReceipt, PortError>;
}

#[async_trait]
pub trait UnitOfWork: Send + Sync {
    async fn begin(&self) -> Result<Box<dyn Tx>, PortError>;
}

/// 一个数据库事务：commit 前被 drop 即回滚。方法按聚合分组，一个事务只改一个聚合。
#[async_trait]
pub trait Tx: Send {
    async fn order(&mut self, id: &OrderId) -> Result<Order, PortError>;
    async fn save_order(&mut self, order: &Order) -> Result<(), PortError>;
    async fn append_outbox(&mut self, event: &OrderEvent) -> Result<(), PortError>;
    async fn commit(self: Box<Self>) -> Result<(), PortError>;
}

pub trait Clock: Send + Sync { fn now(&self) -> DateTime<Utc>; }

// crates/application/src/service：编排支付流程，只编排，不写业务计算；远程调用在事务外
pub struct PayOrderService {
    uow: Arc<dyn UnitOfWork>,
    gateway: Arc<dyn PaymentGateway>,
    clock: Arc<dyn Clock>,
}

impl PayOrderService {
    pub async fn pay(&self, id: OrderId) -> Result<(), AppError> {
        // 事务 1：待支付 → 支付中，提交后才调用外部
        let mut tx = self.uow.begin().await?;
        let mut order = tx.order(&id).await?;
        order.start_payment(self.clock.now())?;
        tx.save_order(&order).await?;
        tx.commit().await?;

        // 事务外：不持有 tx；以订单号为幂等键，超时可安全重试
        let receipt = self.gateway.charge(&order).await?;

        // 事务 2：支付中 → 已支付，领域事件写入 outbox；任何一步出错时 tx 被 drop，自动回滚
        let mut tx = self.uow.begin().await?;
        let mut order = tx.order(&id).await?;
        let event = order.mark_paid(receipt, self.clock.now())?;
        tx.save_order(&order).await?;
        tx.append_outbox(&event).await?;
        tx.commit().await?;
        Ok(())
    }   // charge 失败或进程崩溃时订单停在「支付中」，由对账任务按订单号查询支付结果补偿
}

// crates/adapter-db：实现事务端口；PgTx 内部持有 sqlx::Transaction，未提交即 drop 时由 sqlx 回滚
#[async_trait]
impl UnitOfWork for PgUnitOfWork {
    async fn begin(&self) -> Result<Box<dyn Tx>, PortError> {
        let tx = self.pool.begin().await.map_err(to_port_error)?;
        Ok(Box::new(PgTx { tx }))
    }
}

// crates/adapter-payment：实现端口，在这里转换外部格式（请求带 idempotency key = 订单号）
#[async_trait]
impl PaymentGateway for HttpPaymentGateway {
    async fn charge(&self, order: &Order) -> Result<PaymentReceipt, PortError> {
        let resp = self.client.charge(to_request(order)).await.map_err(to_port_error)?;
        Ok(to_receipt(resp))
    }
}

// crates/adapter-http：转换后调用应用层
async fn pay(State(svc): State<Arc<PayOrderService>>, Path(id): Path<i64>) -> Result<StatusCode, ApiError> {
    svc.pay(OrderId::new(id)?).await?;
    Ok(StatusCode::NO_CONTENT)
}
```

### 层级内部规则

- 领域层：用 newtype 区分 ID、金额、数量；不直接取当前时间、随机数、全局 ID，这些都通过参数传入；必须有纯单元测试。
- 聚合：一个事务只修改一个聚合；聚合之间按 ID 引用，不持有对方对象；跨聚合的一致性用领域事件实现最终一致。仓储和事务方法按聚合组织。
- 应用层：只做加载、调用领域对象、调用端口、发布事件、控制事务；不写金额计算，也不写承载业务规则的条件分支。幂等、去重、并发控制属于应用层，具体实现通过端口交给外层。
- 外层：入站只转换和调用；出站只处理协议、签名、序列化、连接、重连、持久化，不做业务判断。回调或读取任务中不做阻塞操作，耗时处理投递到业务任务。

## 数据与一致性

- 金额、价格、数量统一使用 `rust_decimal::Decimal`，禁止 `f64`/`f32`；同一模块不得混用两种表示。
- 用 `checked_add`/`checked_sub`/`checked_mul`/`checked_div`，溢出返回错误；禁止用 `as` 做金额截断。
- 舍入必须显式：`round_dp_with_strategy`，策略按业务选择；禁止 `Decimal::from_f64` 后忽略 `None`；从字符串或整数构造，序列化为字符串。
- 下游写入使用确定性幂等键，数据库加唯一约束；重复消息不能产生重复副作用。
- 领域层不调用 `Utc::now()`/`SystemTime::now()`，`now` 作为参数传入；应用层通过 `Clock` 端口取时间。测量延迟和超时用 `Instant`（单调时钟）。
- 契约演进：表结构变更走迁移脚本并保持向后兼容；消息和接口只加可选字段，不改语义、不删字段。这类改动仍需先问（硬性约束第 8 条）。

## 数据访问

- 禁止在循环里逐条查库或调用远程接口（N+1），改为按 ID 集合批量查询；需要时在端口上提供批量方法。
- 列表查询必须分页并设置上限；不允许无条件的全表查询。
- 批量写入用批量语句（如 sqlx `QueryBuilder::push_values`），不要循环单条写。
- 只查需要的列，不写 `select *`；大字段按需单独查询。
- 事务尽量小：只包住必须原子完成的写操作；查询、计算、远程调用放在事务外。

## 并发

- async 代码中禁止阻塞：`std::thread::sleep`、`block_on`、同步文件/数据库 IO、长时间 CPU 计算；用 `tokio::time::sleep`、异步客户端、`spawn_blocking`。
- 禁止在 `.await` 期间持有 `std::sync::MutexGuard`；需要跨 await 时用 `tokio::sync::Mutex`，或改为消息传递。
- 每个 `tokio::spawn` 必须保留 `JoinHandle` 或加入 `JoinSet`，关停时等待；禁止 detach 后不管。
- 任务必须能被取消：使用 `CancellationToken`、`select!` 或 drop 语义；取消时清理资源并向上传播。
- `select!` 的分支必须考虑取消安全：被取消时不能丢失已读取的消息或半写入的状态。
- 优先使用有界 `mpsc::channel`，不用 `unbounded_channel`；必须明确满了的策略（阻塞、丢弃、拒绝受理、熔断），写在代码注释中；处理 `try_send` 的 `Full` 错误，不得静默丢弃。
- 锁内不做 IO、不 `.await`、不调用可能重入的回调；锁范围尽量短。

## 错误处理与事务

- 领域层用 `thiserror` 定义领域错误，应用层定义应用错误和端口错误；出站适配器把第三方错误（`sqlx::Error`、`reqwest::Error`）转换为端口错误；入站适配器统一把内层错误转换为响应。`anyhow` 只在 app 和入站适配器边缘使用。
- 业务路径禁止 `unwrap()`/`expect()`/`panic!`；禁止用 `unwrap_or_default()` 吞掉错误。
- 失败不能只打日志后继续，必须明确采取以下一种处理：重试（有上限、退避、抖动）、死信、熔断、返回错误、停机。
- 事务边界在应用层，通过 `UnitOfWork` 端口控制；适配器和 app 不自行决定事务范围。
- 事务内不做远程调用：持有 `Tx` 时不能 `.await` 其他端口的远程调用；数据库与消息的一致性用 outbox 或提交后发送，不做双写。
- 只有启动阶段遇到无法恢复的配置错误才允许 `panic!`/`expect()`，错误信息必须包含配置项名称。

## 日志、安全与配置

- 日志用 `tracing` 结构化字段：`info!(order_id = %id, "order paid")`，不用 `format!` 拼接消息；错误作为字段记录：`error!(error = ?e, order_id = %id, "charge failed")`。
- 领域层不打日志，用返回值或领域错误表达结果；日志放在应用层和适配器。
- 链路上下文用 span 传递（`#[instrument(skip(self), fields(order_id = %id))]`）；`tokio::spawn` 新任务时用 `.instrument(span)` 把 span 带过去。
- 日志级别：error 只用于需要人处理的问题，可预期的业务失败用 warn 或 info；循环内和高频路径的日志必须限频。
- 通道积压、丢弃、重试、重连次数必须有指标或限频日志。
- 密钥、token、密码不硬编码，不写进日志和错误信息；含密钥的配置结构手写 `Debug` 并脱敏，不要 derive。
- SQL 一律参数化（`query!`/`bind`），禁止用 `format!` 拼接 SQL。
- 格式校验在入站适配器完成，把外部输入解析成领域 newtype；业务不变量由领域对象在构造和状态流转时校验。
- 配置在 app 反序列化为类型化配置并在启动时校验，只在外层使用；内层需要的参数通过构造函数传入，不读配置。URL、超时、并发数、通道容量不硬编码。
- 所有外部调用（HTTP、DB、Redis、MQ）必须有明确的超时：客户端级超时（如 reqwest `timeout`、连接池 `acquire_timeout`）或 `tokio::time::timeout`。

## Rust 编码细则

- 实体字段私有，状态变化通过表达业务意图的方法完成（`start_payment`、`mark_paid`），不提供通用 setter，不 `pub` 字段。
- 值对象在构造函数中校验不变量，构造失败返回 `Result`；校验通过后的值不可变。
- 业务枚举的 `match` 不写 `_ =>` 兜底，新增变体时让编译器提示所有需要处理的地方。
- 可见性最小化：默认私有，crate 内共享用 `pub(crate)`，只有跨 crate 的 API 才 `pub`。
- 不为了通过借用检查随手 `clone()` 或包 `Arc<Mutex<_>>`；先调整所有权和借用，确需共享再用 `Arc`。
- 禁止 `unsafe`，确需使用时先问，并写 `// SAFETY:` 注释说明成立条件。
- 注释写「为什么」，不写「做了什么」；公开 API 的非显然前置条件、`Send`/`Sync` 要求、可能的 panic 写在文档注释里。

## 测试与验证

- 领域计算、状态流转必须有普通 `#[test]` 单元测试，不启动 tokio runtime，覆盖正常、边界、非法输入。
- 应用层测试用端口的内存 fake 替换 IO（包括 `UnitOfWork` 和 `Clock`）；不连接真实数据库、Redis、第三方服务；不要 mock 被测逻辑本身。
- async 测试用 `#[tokio::test]`；时间相关逻辑用 `tokio::time::pause`/`advance`，不要真实 sleep。
- 适配器用集成测试或契约测试验证协议、序列化、重连、事务回滚；不要 mock 掉被测的转换逻辑。
- 测试必须确定性：不依赖当前时间、随机数、真实网络、执行顺序；时间通过 fake `Clock` 注入。
- 禁止为了变绿加 `#[ignore]`、修改断言、删除用例、放宽 clippy 或超时；测试只加严不放宽。
- 覆盖率不是目标，但核心业务分支必须有测试；新增分支不得没有测试。

## AI 常见劣化模式

| 劣化写法 | 正确做法 |
|---|---|
| domain 依赖 tokio/sqlx/reqwest/serde | 领域层保持纯净，序列化交给适配器 |
| 入站适配器直接调用出站适配器或直接查库 | 走应用服务 + 端口；查询走 `application::query` |
| 端口签名里出现第三方请求类型 | 端口只用内层类型，转换放在适配器 |
| 单实现也建工厂，为纯计算建端口，或加 `if use_new` 兼容层 | 只为 IO 边界建端口，其余用具体类型，保持单一路径 |
| 应用服务里写金额计算和业务 if-else | 规则下沉到实体、值对象或领域服务 |
| 实体字段 `pub`，在应用层直接改状态 | 私有字段 + 表达业务意图的方法 |
| 一个事务里修改多个聚合 | 按 ID 引用，跨聚合用领域事件 |
| 持有 `Tx` 时 `.await` 远程调用，或先调外部、后落库且没有幂等键 | 先落库「处理中」，以业务 ID 为幂等键调用外部，再落库结果 |
| 业务路径 `unwrap()`，或只打日志后继续 | 返回 `Result`，或明确重试、死信、熔断 |
| 加 `#[ignore]`、改断言，或只跑单个测试就宣称通过 | 修代码，跑完整检查并贴输出 |

## 检查方式与完成定义

### 检查

- `cargo fmt --check`
- `cargo clippy --workspace --all-targets -- -D warnings`
- `cargo test --workspace`
- 在 workspace 的 `Cargo.toml` 中用 lint 把文字约束变成编译检查（各 crate 写 `[lints] workspace = true`）：

```toml
[workspace.lints.clippy]
unwrap_used = "deny"
expect_used = "deny"
panic = "deny"
todo = "deny"
unimplemented = "deny"
dbg_macro = "deny"
print_stdout = "deny"
print_stderr = "deny"
await_holding_lock = "deny"
float_arithmetic = "warn"   # 金额必须用 Decimal；确需浮点处局部 allow 并注释原因
```

- 根目录 `clippy.toml` 设置 `allow-unwrap-in-tests = true`、`allow-expect-in-tests = true`；`crates/domain/clippy.toml` 和 `crates/application/clippy.toml` 再用 `disallowed-methods` 禁止 `chrono::Utc::now`、`std::time::SystemTime::now`、`std::time::Instant::now`（领域层），时间只能通过参数或 `Clock` 端口获得。clippy 只读取离 crate 最近的一个 `clippy.toml`，不会合并，所以子目录的文件要重复根目录的设置。
- 以下规则靠自查和 review：端口按业务设计而不是按外部 API 设计、应用层不写业务规则、一个事务只改一个聚合、金额舍入策略、持有 `Tx` 时的远程调用、锁内 IO、日志中的敏感信息、通道满时的策略。

### 完成定义（DoD）

- [ ] 代码在正确的层和 crate，依赖方向符合本文件。
- [ ] `cargo fmt`、`clippy -D warnings`、`cargo test --workspace` 全部通过，并贴出真实输出。
- [ ] 新增业务分支有单元测试，覆盖失败路径；修 bug 有复现测试。
- [ ] 没有 TODO、`todo!()`、注释掉的代码、调试日志、未使用的参数。
- [ ] 错误处理、日志、超时与取消、配置方式符合本文件。
- [ ] 没有新增依赖、工厂或兼容层；新增端口对应真实 IO 边界；如有例外，已在回复中说明理由。
- [ ] 已按「工作方式」的格式汇报。

## 已知的历史违规

> 记录现存不符合上述规则的代码，避免被模仿。只在本次需求本来就要改到的代码里顺带修正；超出这个范围的不动，在汇报中列出。没有则保持为空。

- （无）

## 附录：实时交易系统（可选）

> 非交易类项目删除本节。本节规则补充正文，冲突时以本节为准。

- 交易所报文、签名、限频（rate limit）、错误码只在出站适配器处理；端口按业务设计（`submit(&HedgeOrder)`），禁止 `submit(BinanceOrderRequest)`。
- 下单使用确定性的 `client_order_id` 作为幂等键，超时或网络错误后先按它查询订单状态再决定是否重下，禁止盲目重试。
- 行情、订单推送必须处理重复、乱序、gap、断线重连：出现 gap 时作废本地缓存并重拉快照，不能跳过继续用；重连后必须重新订阅。
- 背压分级：行情类可以丢弃并置 gap 标记；订单、回报、成交类不能丢，通道满时拒绝受理或熔断。
- WS 读取任务只做解析和投递，不查库、不发 HTTP、不 sleep。
- 热路径禁止逐笔日志、`serde_json`、`format!`、`Arc<dyn Trait>` 动态分发（热路径改用泛型参数）、无界重试和不必要的 `clone`。
- 价格和数量按交易对的 tickSize/stepSize 取整，取整规则写在领域值对象中（如 `Symbol::align_price(price, side)`），舍入方向由买卖方向决定，不依赖默认值。
