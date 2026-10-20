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
