# CLAUDE.md

## 架构声明

本项目采用**经典分层架构**：依赖只能自上而下单向流动，层是封闭的，不能跳层。`app` 是组装根。

```
app（组装）──> entry、service、infra、common

entry ──> service ──> infra ──> common
  └──────────┴──────────────────> common
```

**这不是 Clean 架构、六边形架构或依赖倒置。** service 直接依赖 infra 的具体类型。未经用户明确同意，不要：

- 在 service 定义端口 trait、再由 infra 实现；
- 把 infra 类型抽象成 `dyn Trait` 层；
- 把分层改造成洋葱或整洁架构。

需要替换 IO 做单测时，用泛型参数、函数类型或 `#[cfg(test)]` 的 fake；不要为每个 infra 类型定义生产端口。

## 对 AI 的硬性约束

本节是 MUST，优先于你的默认习惯和所谓的「最佳实践」。

1. **先定位再写**：动手前确定代码属于哪一层、哪个 crate 或模块。落不进「本项目的层级」表就停下来问，不要自创 crate。
2. **先读后写**：找同层 1–2 个现有实现，沿用其结构、命名、错误处理和日志方式。不要引入仓库里没有的新风格。
3. **最小改动**：只改与需求相关的代码。不顺手重构、不重排 `use`、不升级依赖、不改 rustfmt/clippy 配置。
4. **抽象准入**：没有第二个实现、不属于真实 IO 边界时，不建 trait、工厂、泛型仓储、兼容层、fallback、feature flag。三行重复优于一个错误抽象。
5. **依赖准入**：不新增 crate。确需新增时，先说明用途、体积、维护状态和 license，等用户确认后再改 `Cargo.toml`。
6. **不写占位**：不留 TODO、`todo!()`、`unimplemented!()`、mock 返回值、假数据、注释掉的代码、未使用的参数。
7. **验证真实**：完成前运行本节列出的全部检查，贴出命令和真实输出。失败就说失败并继续修，禁止 `#[ignore]`、放宽 clippy、改断言或删除测试来让结果变绿。
8. **必须停下来问**：层级归属不明；新增 crate 或模块；新增依赖；修改公共类型、对外接口、消息格式、表结构；需要放宽架构检查；修复历史违规会显著扩大改动范围。

## 分层与依赖

### 层的角色

每一层必须归入以下某种角色。可以按业务拆子模块，但不存在不属于任何角色的层。

| 角色 | 职责 | 典型内容 |
|---|---|---|
| 入口 | 接收外部触发，转换后调用业务层 | HTTP handler、WS 推送接收、MQ 消费、定时任务、CLI |
| 业务 | 业务规则与流程编排 | 计算、决策、状态流转、结果处理、事务边界 |
| 基础设施 | 技术细节 | 交易所/第三方客户端、数据库、Redis、Kafka、文件、缓存加载 |
| 公共 | 各层共用的只读类型与纯工具，不依赖任何层 | newtype、枚举、错误基础类型、纯函数工具 |
| 组装 | 创建对象、注入依赖、启停任务，只做接线 | `src/main.rs`、`bootstrap` 模块 |

### 本项目的层级

> 下表为示例，按实际项目修改。新增层、拆分 crate 或模块时，同步更新本表和依赖图。

| 层 | crate / 模块 | 可以依赖 | 外部依赖限制 |
|---|---|---|---|
| app | `src/main.rs` | 所有层 | 无 |
| entry | `src/entry` | service、common | 不直接用 sqlx、redis、rdkafka、reqwest |
| service | `src/service` | infra、common | 禁 tokio 网络、sqlx、redis、rdkafka、reqwest |
| infra | `src/infra` | common | 无 |
| common | `src/common` | 无 | 只允许 `rust_decimal`、`thiserror`、`chrono` |

- 多 crate：下层 crate 的 `Cargo.toml` 不得声明上层 crate，由编译器保证。
- 单 crate：下层模块不得出现 `use crate::<上层模块>`，由代码评审和 CI 检查保证。

### 依赖规则

- 上层可以依赖下层，**下层禁止依赖上层**。多 crate 由编译器保证；单 crate 编译不报错，必须人工检查。
- 依赖图不能有环。新增层时，先确定它在图中的位置，再写代码。
- 禁止用 `static`、`OnceLock`、`lazy_static`、`thread_local!` 保存服务对象或做服务定位。所有依赖在 app 通过构造函数传入。
- 业务层不直接使用中间件或 SDK 的 API（sqlx、redis、rdkafka、reqwest、tokio-tungstenite），必须通过 infra 封装的类型访问。
- 基础设施层只处理技术问题（协议、签名、序列化、连接、重连、持久化），不做业务判断。
- 入口层只做解析、校验和调用，不写业务逻辑，不直接调用 infra。
- 公共层不依赖 tokio、网络、数据库或文件 IO，保持纯数据和纯函数。
- 数据库行结构、交易所报文、SDK 类型只在 infra 内使用；返回上层前转换为业务类型。

### 新代码放哪里

- 新的外部系统接入 → infra，协议和报文转换在该层完成。
- 新的业务规则或流程 → service；计算尽量写成不依赖异步和 IO 的纯函数。
- 新的触发方式 → entry，只做转换和调用。
- 多层共用的只读类型 → common；只在一层内使用的类型留在该层。
- 交易所报文、数据库模型、SDK 类型不得离开 infra。

## 跨边界数据与下层通知上层

### 类型归属

- 数据库行结构、交易所原始报文、SDK 类型只在 infra 内使用；返回前转换为 service 需要的类型。
- 领域类型不 derive `Serialize`/`Deserialize`；序列化格式由 infra 决定。
- common 只放跨层共用的只读类型、枚举、错误基础类型、纯函数工具；禁止放 serde 模型、数据库行结构、请求/响应 DTO。

### 方式一：回调 trait（单次结果、请求-回报型）

1. trait 定义在下层，与调用它的类型放在一起；签名只用下层或 common 的类型。
2. 上层为自己的类型实现该 trait。
3. 组装处把实现以 `Arc<dyn Trait>` 或泛型参数传给下层。

```rust
// infra：下层定义 trait
pub trait OrderResultListener: Send + Sync {
    fn on_order_result(&self, event: OrderResultEvent);
}

pub struct OrderClient { listener: Arc<dyn OrderResultListener> }

// service：上层实现（service -> infra，方向合法）
impl infra::order::OrderResultListener for HedgeService {
    fn on_order_result(&self, event: OrderResultEvent) { /* ... */ }
}
```

异步 trait 不能直接用于 `dyn`：用 `#[async_trait]`、泛型参数，或改用方式二。只有一个调用方且无需解耦时，直接调用下层。

### 方式二：事件通道（持续推送、流式数据）

1. 事件类型定义在下层或 common。
2. 下层持有 `mpsc::Sender<Event>`，只负责发送。
3. 上层持有 `Receiver`，在自己的任务中消费。
4. 通道在 app 创建，两端分别交给下层和上层；使用有界通道（`mpsc::channel(n)`），不用 `unbounded`。
5. 订单/回报/成交类不能丢，满了应拒绝受理或熔断；行情类可以丢弃并置 gap 标记。

### 跨边界约束

- trait 和事件类型的签名只能使用下层或 common 的类型，不能出现上层类型。
- 事件携带转换后的类型，不传交易所原始报文或 SDK 类型；参数多时封装成事件结构体。
- 回调或接收任务中不做阻塞操作；CPU 密集计算用 `spawn_blocking`。
- 通道关闭由发送端 drop 完成；接收端不主动关闭；多发送者用取消信号统一退出。
- 队列满、丢弃、延迟、重连次数必须有指标或限频日志。

## 数据、时间与一致性

- 价格、数量、金额统一使用 `rust_decimal::Decimal`，禁止 `f64`/`f32`；同一模块不得混用两种表示。
- 用 `checked_add`/`checked_sub`/`checked_mul`/`checked_div`，溢出返回错误；禁止用 `as` 做金额截断。
- 舍入必须显式：`round_dp_with_strategy`，策略按业务选择，不依赖默认。
- 禁止 `Decimal::from_f64` 后忽略 `None`；从字符串或整数构造，序列化为字符串。
- 下游写入使用确定性幂等键，数据库加唯一约束；重复消息不能产生重复副作用。
- 消息流必须处理重复、乱序、gap、断线重连：gap 时作废本地缓存并重拉快照，不能跳过继续用；重连后必须重订阅。
- 业务代码不直接调用 `Utc::now()`/`SystemTime::now()`：把 `now` 作为参数注入；延迟和超时用 `Instant`（单调时钟）。

## 并发与性能

- async 代码中禁止阻塞：`std::thread::sleep`、`block_on`、同步文件/数据库 IO、长时间 CPU 计算；用 `tokio::time::sleep`、异步客户端、`spawn_blocking`。
- 禁止在 `.await` 期间持有 `std::sync::MutexGuard`；需要跨 await 时用 `tokio::sync::Mutex`，或改为消息传递。
- 每个 `tokio::spawn` 必须保留 `JoinHandle`，或加入 `JoinSet`，并在关停时等待；禁止 detach 后不管。
- 任务必须能被取消：使用 `CancellationToken`、`select!` 或 drop 语义；取消时清理资源并向上传播。
- 优先使用有界 `mpsc::channel`；处理 `try_send` 的 `Full` 错误，不得静默丢弃。
- `select!` 的分支必须考虑取消安全：被取消时不能丢失已读取的消息或半写入的状态。
- 锁内不做 IO、不 `.await`、不调用可能重入的回调；锁范围尽量短。
- 热路径禁止逐笔日志、`serde_json`、`format!`、`Arc<dyn Trait>` 动态分发、无界重试和不必要的 `clone`。

## 错误处理与事务

- 每层用 `thiserror` 定义自己的错误类型，service 的公开接口使用具体错误；`anyhow` 只在 app 和 entry 边缘使用，不进入 service/infra 公开 API。
- 出站调用把第三方错误（`sqlx::Error`、`reqwest::Error`）转换为本层错误，用 `?` + `From` 或显式 `map_err`。
- 业务路径禁止 `unwrap()`/`expect()`/`panic!`；禁止用 `unwrap_or_default()` 吞掉错误。
- 失败必须明确重试（有上限、退避、抖动）、进入死信、熔断、返回错误或停机中的一种，不能只打日志继续。
- 事务边界在 service，通过 infra 的 repository/UoW 控制；entry 不开事务，infra 不自开事务。
- 事务内不做远程调用（交易所、HTTP、MQ 同步发送）；数据库与消息的一致性用 outbox 或提交后发送，不做双写。
- 只有在启动阶段遇到无法恢复的配置错误才允许 `panic!`/`expect()`，并保证错误信息包含配置项名称。

## 测试与验证

- 领域计算、状态流转必须有普通 `#[test]` 单元测试，不启动 tokio runtime，覆盖正常、边界、非法输入。
- service 测试用泛型参数、函数类型或 fake 替换 IO；不连接真实数据库、Redis、交易所；不要 mock 被测逻辑本身。
- async 测试用 `#[tokio::test]`；时间相关逻辑用 `tokio::time::pause/advance`，不要真实 sleep。
- infra 用集成测试或契约测试验证协议、序列化、重连；不要 mock 掉被测的转换逻辑。
- 测试必须确定性：不依赖当前时间、随机数、真实网络、执行顺序；`now` 通过参数注入。
- 禁止为了变绿加 `#[ignore]`、修改断言、删除用例、放宽 clippy 或超时；测试只加严不放宽。
- 覆盖率不是目标，但核心业务分支必须有测试；新增分支不得无测试。

## AI 常见劣化模式

| 劣化写法 | 正确做法 |
|---|---|
| service 直接依赖 sqlx/redis/reqwest | 通过 infra 封装访问 |
| 把类型挪到 common 让两层都能用 | 类型归真正拥有它的层，边界做转换 |
| 单实现也建 trait + factory + `dyn` | 直接用具体类型，出现第二实现再抽象 |
| 加 `if use_new { ... } else { ... }` 兼容层 | 删旧路径，保持单一路径 |
| 加 `#[ignore]`、改断言让测试变绿 | 修代码；测试只加严 |
| 只跑单个测试就宣称全部通过 | 跑完整检查并贴输出 |
| 业务路径 `unwrap()`/`expect()`/`unwrap_or_default()` | 返回 `Result`，错误向上传递 |
| `f64` 金额、`as` 截断、`Decimal::from_f64` 忽略 None | `rust_decimal` + checked 运算 + 显式舍入 |
| 持有 `std::sync::MutexGuard` 跨 `.await` | `tokio::sync::Mutex` 或消息传递 |
| `tokio::spawn` 不保存句柄、`unbounded` 通道 | `JoinSet`/`JoinHandle` + 有界 `mpsc` |

## 检查方式与完成定义

### 检查

- `cargo fmt --check`
- `cargo clippy --all-targets -- -D warnings`
- `cargo test`
- 多 crate：`cargo tree -p <下层 crate> -e normal`，输出中不应出现上层 crate。
- 单 crate：在下层模块目录搜索 `use crate::<上层模块>` 和 `crate::<上层模块>::`，结果应为空；这项不是编译门禁，CI 必须显式执行。
- 单 crate 建议拆成多 crate，让依赖方向由 `Cargo.toml` 强制保证；不拆时必须在 CI 加脚本检查。

### 完成定义（DoD）

- [ ] 代码在正确的层和 crate，依赖方向正确，`cargo tree` 或 CI 检查通过。
- [ ] `cargo fmt`、`clippy -D warnings`、`cargo test` 全部通过，并贴出真实输出。
- [ ] 新增业务分支有单元测试，覆盖失败路径。
- [ ] 没有 TODO、`todo!()`、注释掉的代码、调试日志、未使用的参数。
- [ ] 错误处理、日志上下文、超时和取消符合本文件。
- [ ] 没有新增依赖、trait、工厂或兼容层；如确有必要，已在回复中说明理由。

## 已知的历史违规

> 记录现存不符合上述规则的代码，避免被模仿；修改相关代码时顺手修正。没有则保持为空。

- （无）
