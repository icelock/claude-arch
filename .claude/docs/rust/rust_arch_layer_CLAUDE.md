# CLAUDE.md

## 架构声明

本项目采用**经典分层架构**：依赖只能自上而下单向流动，层是封闭的，不能跳层。组装层（`src/main.rs`、`src/bootstrap`）是组装根。

```
组装 ──> entry ──> service ──> infra ──> common
  └────────┴──────────┴──────────┴─────> common
```

**组装层（`src/main.rs`、`src/bootstrap`）可以依赖所有层**：它只做接线，是唯一同时知道所有具体实现的地方；任何层都不能反过来依赖组装层。

common 是共享内核，各层依赖 common 不算跳层；除此之外，entry 不能直接依赖 infra。

**这不是 Clean 架构、六边形架构或依赖倒置。** service 直接依赖 infra 的具体类型。未经用户明确同意，不要：

- 在 service 定义端口 trait、再由 infra 实现；
- 把 infra 类型抽象成 `dyn Trait` 层；
- 把分层改造成洋葱或整洁架构。

唯一例外是单元测试：service 可以在自己的模块内定义不超过 3 个方法的窄 trait，并在 service 内为 infra 的具体类型实现它（方向仍是 service → infra），测试时换成 fake；生产装配仍传具体类型。也可以用函数类型或 `#[cfg(test)]` 的 fake。禁止系统性地为每个 infra 类型建 trait。

## 架构工作方式

- 跨层改动时，先用几行说明计划：改哪些文件、每项落在哪一层、哪个 crate 或模块，是否涉及「必须停下来问」的事项，再动手。

## 架构硬性约束

本节只保留与架构相关的约束；通用约束见根 `CLAUDE.md`。

1. **先定位再写**：动手前确定代码属于哪一层、哪个 crate 或模块。落不进「本项目的层级」表就停下来问，不要自创 crate 或顶层模块。
2. **必须停下来问**：层级归属不明；新增 crate 或层。

## 分层与依赖

### 层的角色

每一层必须归入以下某种角色。可以按业务拆子模块，但不存在不属于任何角色的层。

| 角色 | 职责 | 典型内容 |
|---|---|---|
| 组装 | 创建对象、注入依赖、加载配置、启停任务，只做接线 | `src/main.rs`、`src/bootstrap` |
| 入口 | 接收外部请求或定时触发，转换和校验后调用业务层 | HTTP handler、MQ 消费者、定时任务、CLI |
| 业务 | 业务规则与流程编排 | 计算、决策、状态流转、结果处理、事务边界 |
| 基础设施 | 技术细节 | 第三方客户端、WebSocket 客户端、数据库、Redis、Kafka、文件、缓存加载 |
| 公共 | 各层共用的只读类型与纯工具，不依赖任何层 | newtype、枚举、错误基础类型、跨层只读数据结构、纯函数工具 |

### 本项目的层级

> 下表为示例，按实际项目修改。在已有模块下新增子模块不需要改。

| 层 | 模块（单 crate）/ crate（多 crate） | 可以依赖 | 外部依赖限制 |
|---|---|---|---|
| 组装 | `src/main.rs`、`src/bootstrap` / `crates/app` | 所有层 | 无 |
| entry | `src/entry`（`http`、`mq`、`job` 子模块）/ `crates/entry` | service、common | 不直接用 sqlx、redis、rdkafka、reqwest |
| service | `src/service` / `crates/service` | infra、common | 不用 sqlx、redis、rdkafka、reqwest、tokio-tungstenite、`tokio::net`/`tokio::fs` |
| infra | `src/infra` / `crates/infra` | common | 无 |
| common | `src/common` / `crates/common` | 无 | 只允许 `rust_decimal`、`thiserror`、`chrono` |

- HTTP handler 放 `entry::http`，MQ 消费者放 `entry::mq`，定时任务放 `entry::job`；它们都只负责触发并调用 service。
- 依赖图不能有环；`Arc` 持有关系也不能成环（见「方式一」）。

### 依赖规则

- 上层可以依赖下层，**下层禁止依赖上层**。多 crate 由编译器保证；单 crate 编译不报错，写代码时自行保证。
- 跨层引用一律写完整路径 `use crate::<层>::...`，不用 `super::` 跨层，否则跨层引用不容易查出来。
- 禁止用 `static`、`OnceLock`、`LazyLock`、`lazy_static`、`thread_local!` 保存服务对象或做服务定位。所有依赖在组装层通过构造函数传入。
- 业务层不直接使用中间件或 SDK 的 API（sqlx、redis、rdkafka、reqwest、tokio-tungstenite），必须通过 infra 封装的类型访问。
- 基础设施层只处理技术问题（协议、签名、序列化、连接、重连、持久化），不做业务判断。
- 入口层只做解析、校验和调用，不写业务逻辑，不直接调用 infra。
- 公共层不依赖 tokio、网络、数据库或文件 IO，保持纯数据和纯函数。

### 新代码放哪里

- 新的外部系统接入 → infra，协议和报文转换在该层完成。
- 新的业务规则或流程 → service；计算尽量写成不依赖异步和 IO 的纯函数。
- 新的 HTTP 接口、消息消费或定时任务 → entry（`http`、`mq`、`job`），只做转换和调用。
- 新的启动预热、后台任务的启停 → 组装层的 `bootstrap`，只做接线和启停，不写业务逻辑。
- 多层共用的只读数据 → common；只在一层内使用的类型留在该层，不要提前上移。

## 跨边界数据与下层通知上层

### 类型归属

- common 只放跨层传递的只读数据结构、newtype、枚举、错误基础类型；不 derive `Serialize`/`Deserialize`，序列化格式由 infra 和 entry 决定。
- 数据库行结构、外部系统原始报文、SDK 类型只在 infra 内使用，不得进入 common；返回上层前转换为 service 需要的类型。
- 只服务于某个 HTTP 接口的请求/响应结构留在 entry，不要传进 service。

### 方式一：回调 trait（单次结果、请求-回报型）

1. trait 定义在下层，与调用它的类型放在一起；签名只用下层或 common 的类型。
2. 上层为自己的类型实现该 trait。
3. 组装处把实现以 `Arc<dyn Trait>` 或泛型参数传给下层。

```rust
// infra::payment：下层定义回调 trait
pub trait PaymentResultListener: Send + Sync {
    fn on_payment_result(&self, event: PaymentResultEvent);
}

pub struct PaymentClient { listener: Arc<dyn PaymentResultListener> /* , http 客户端等 */ }

impl PaymentClient {
    pub fn new(listener: Arc<dyn PaymentResultListener>) -> Self { /* ... */ }
    pub async fn pay(&self, order_no: &OrderNo, amount: Decimal) -> Result<(), PaymentError> { /* 结果异步到达后调用 listener */ }
}

// service：发起支付，只持有 PaymentClient
pub struct OrderService { payment: Arc<PaymentClient> }

// service：单独的类型实现回调，不持有 PaymentClient
pub struct PaymentResultHandler { tx: mpsc::Sender<PaymentResultEvent> }

impl PaymentResultListener for PaymentResultHandler {
    fn on_payment_result(&self, event: PaymentResultEvent) {
        // 回调运行在 IO 任务上，只投递；队列满时拒绝并告警，支付结果靠对账补偿
        if let Err(e) = self.tx.try_send(event) { tracing::error!(error = %e, "payment result queue full"); }
    }
}

// bootstrap：组装
let (tx, rx) = mpsc::channel(1024);
let payment = Arc::new(PaymentClient::new(Arc::new(PaymentResultHandler { tx })));
let orders = OrderService::new(payment.clone());
```

- **实现回调 trait 的类型不能再持有那个下层客户端**：上例若让 `OrderService` 同时实现 `PaymentResultListener`，构造 `PaymentClient` 需要 `Arc<OrderService>`，构造 `OrderService` 又需要 `Arc<PaymentClient>`，无法直接构造；用 `Weak`、`OnceLock` 延迟注入或 `set_listener` 绕过，会形成 `Arc` 引用环（内存泄漏、关停时 drop 不掉）或运行期未初始化状态。编译器查不到这种环。
- 异步 trait 不能直接用于 `dyn`：需要异步回调时用 `#[async_trait]`、泛型参数，或改用方式二。只有一个调用方且无需解耦时，直接调用下层，不要为形式加 trait。

### 方式二：事件通道（持续推送、流式数据）

1. 事件类型定义在下层或 common；下层持有 `mpsc::Sender<Event>`，只负责发送；上层持有 `Receiver`，在自己的任务中消费。
2. 通道在组装层创建，两端分别交给下层和上层；用有界通道 `mpsc::channel(n)`，不用 `unbounded_channel`。
3. 通道关闭由发送端 drop 完成；接收端不主动关闭；多发送者用 `CancellationToken` 统一退出。

### 跨边界约束

- trait 和事件类型的签名只能使用下层或 common 的类型，不能出现上层类型。
- 事件携带转换后的类型，不传原始报文或 SDK 类型；参数多时封装成事件结构体。
- 回调或接收任务中不做阻塞操作；耗时处理投递到业务任务，CPU 密集计算用 `spawn_blocking`。

## AI 常见劣化模式

| 劣化写法 | 正确做法 |
|---|---|
| service 直接依赖 sqlx/redis/reqwest | 通过 infra 封装访问 |
| 把类型挪到 common 让两层都能用 | 类型归真正拥有它的层，边界做转换 |
| 单实现也建 trait + factory + `dyn`，或加 `if use_new` 兼容层 | 直接用具体类型，保持单一路径 |
| 同一类型既实现下层回调 trait，又持有该下层客户端 | 回调拆到单独的类型，或改用通道 |
| 循环里逐条查库、列表不分页 | 批量查询，分页并设上限 |
| 业务路径 `unwrap()`/`expect()`/`unwrap_or_default()` | 返回 `Result`，错误向上传递 |
| 持有 `std::sync::MutexGuard` 或事务对象跨远程调用的 `.await` | 消息传递或 `tokio::sync::Mutex`；远程调用放在事务外 |
| `tokio::spawn` 不保存句柄、`unbounded_channel` | `JoinSet`/`JoinHandle` + 有界 `mpsc` |
| entry 的 handler、消费者、定时任务里写业务逻辑 | 入口只转换和调用，逻辑放 service |
| 加 `#[ignore]`、改断言，或只跑单个测试就宣称通过 | 修代码，跑完整检查并贴输出 |

## 架构自查

- 代码在正确的层和 crate，依赖方向符合本文件；service 不直接依赖 sqlx、redis、reqwest、tokio-tungstenite。
