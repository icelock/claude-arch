# CLAUDE.md

## 应用架构：分层架构

本项目采用分层架构，依赖只能自上而下单向流动。层级和模块/crate 名称根据业务需要定义，但必须遵守下面的依赖规则。新增或修改代码前，先确认代码属于哪一层。

### 层的角色

每一层都必须归入以下某种角色。可以按业务拆出更多层或子模块，但不能出现不属于任何角色的层。

| 角色 | 职责 | 典型内容 |
|---|---|---|
| 入口 | 接收外部触发，转换后调用业务层 | `main`、HTTP handler、WS 推送接收、MQ 消费、定时任务、CLI |
| 业务 | 业务规则与流程编排 | 计算、决策、状态流转、结果处理 |
| 基础设施 | 技术细节 | 交易所/第三方客户端、数据库、Redis、Kafka、文件、缓存加载 |
| 公共 | 各层共用的类型与工具，不依赖任何层 | 领域通用类型、枚举、错误基础类型、工具函数 |
| 组装 | 创建对象、注入依赖、启动任务，只做接线 | `main.rs` 或 `bootstrap` 模块 |

### 本项目的层级

> 新增层、拆分 crate 或模块时，同步更新本表和依赖图。下表为示例，按实际项目修改。

| 层 | crate / 模块 | 角色 | 可以依赖 |
|---|---|---|---|
| app | `src/main.rs` | 组装 | 所有层 |
| entry | `src/entry` | 入口 | service、common |
| service | `src/service` | 业务 | infra、common |
| infra | `src/infra` | 基础设施 | common |
| common | `src/common` | 公共 | 无（只允许第三方基础库） |

```
app（组装）──> entry、service、infra、common

entry ──> service ──> infra ──> common
  └──────────┴──────────────────> common
```

### 依赖规则

- 上层可以依赖下层，**下层禁止依赖上层**：
  - 多 crate：下层 crate 的 `Cargo.toml` 不得声明上层 crate 依赖（由编译器保证）。
  - 单 crate 多模块：下层模块中不得出现 `use crate::<上层模块>`。
- 依赖图中不能有环。新增层时，先确定它在图中的位置，再写代码。
- 禁止通过全局单例绕过依赖规则，例如在下层用 `static` / `OnceLock` / `lazy_static` 保存上层对象再调用。所有依赖在组装处通过构造函数传入。
- 业务层不直接使用中间件或 SDK 的 API（`sqlx`、`redis`、`rdkafka`、`reqwest`、`tokio-tungstenite` 等），必须通过基础设施层封装的类型访问。
- 基础设施层只处理技术问题（协议、签名、序列化、连接、重连、持久化），不做业务判断。
- 公共层不依赖 tokio 运行时、网络或 IO 相关 crate，保持纯数据和纯函数。

### 下层通知上层：下层定义 trait 或事件通道，上层实现/消费

当下层需要把结果交给业务层（下单回报、WS 推送、MQ 消息等）时，**不能直接调用上层**。按以下两种方式之一处理。

#### 方式一：回调 trait（单次结果、请求-回报型）

1. trait 定义在下层，与调用它的类型放在一起。
2. 上层为自己的类型实现该 trait。
3. 组装处把实现以 `Arc<dyn Trait>` 或泛型参数传给下层。

```rust
// infra：下层定义 trait，签名只用 infra 或 common 中的类型
pub trait OrderResultListener: Send + Sync {
    fn on_order_result(&self, event: OrderResultEvent);
}

pub struct OrderClient {
    listener: Arc<dyn OrderResultListener>,
}

impl OrderClient {
    pub fn new(listener: Arc<dyn OrderResultListener>) -> Self {
        Self { listener }
    }
}

// service：上层实现下层 trait（service -> infra，方向合法）
impl infra::order::OrderResultListener for HedgeService {
    fn on_order_result(&self, event: OrderResultEvent) { /* ... */ }
}

// app：组装
let hedge = Arc::new(HedgeService::new(/* ... */));
let client = OrderClient::new(hedge.clone());
```

需要异步回调时，trait 中的 `async fn` 不能用于 `dyn`。可以选择以下做法之一：使用 `async-trait`；改用泛型参数；或者改用方式二。

#### 方式二：事件通道（持续推送、流式数据，优先使用）

1. 事件类型定义在下层或 common。
2. 下层持有 `mpsc::Sender<Event>`，只负责发送。
3. 上层持有 `Receiver`，在自己的任务中消费。
4. 通道在组装处创建，两端分别交给下层和上层。

```rust
// infra
pub struct TradeWsClient {
    tx: mpsc::Sender<TradeEvent>,
}

// app：组装
let (tx, rx) = tokio::sync::mpsc::channel::<TradeEvent>(10_000);
let ws = TradeWsClient::new(config, tx);
let hedge = HedgeService::new(/* ... */);
tokio::spawn(hedge.run(rx));
tokio::spawn(ws.run());
```

通道天然把 IO 任务和业务处理隔离开，WS 接收不会被业务处理阻塞，适合行情、成交等推送场景。

#### 约束

- trait 和事件类型的签名只能使用下层或 common 中的类型，不能出现上层类型。
- 事件携带的是转换后的类型，不能直接传递交易所原始报文或 SDK 类型。
- 参数较多时封装成事件结构体，不要堆叠参数。
- 回调或接收任务中不做阻塞操作；CPU 密集计算用 `spawn_blocking`；不要在 `.await` 期间持有 `std::sync::Mutex`。
- 有界通道需要明确满了以后的策略（等待、丢弃、告警），并写在代码注释中。
- 只有一个调用方且无需解耦时，上层直接调用下层即可，不要为了形式增加 trait。

### 错误处理

- 每层定义自己的错误类型（推荐 `thiserror`），下层的第三方错误（`reqwest::Error`、`sqlx::Error` 等）在本层内转换，不直接暴露给上层。
- 上层通过 `From` 转换下层错误，或在边界处显式 `map_err`。
- 入口层和组装处可以使用 `anyhow` 汇总错误；业务层和基础设施层的公开接口使用具体错误类型。

### 新代码放哪里

- 新的外部系统接入（交易所、第三方 API、存储）→ 基础设施层，协议和报文转换在该层内完成。
- 新的业务规则或流程 → 业务层；计算逻辑尽量写成不依赖异步和 IO 的纯函数，并配单元测试。
- 新的触发方式（新接口、新定时任务、新消费者）→ 入口层，只做转换和调用。
- 多个层都用到的类型 → common；只在一层内使用的类型留在该层，不要提前放到 common。
- 不属于现有任何层的代码：先在「本项目的层级」表中新增层、明确角色和依赖，再写代码。

### 检查方式

- 多 crate：`cargo tree -p <下层 crate> -e normal` 的输出中不应出现上层 crate。
- 单 crate：在下层模块目录中搜索 `use crate::<上层模块>` / `crate::<上层模块>::`，结果应为空。
- 提交前运行 `cargo clippy --all-targets` 和 `cargo test`。

### 已知的历史违规

> 记录现存不符合上述规则的代码，避免被模仿；修改相关代码时顺手修正。没有则保持为空。

- （无）
