# CLAUDE.md

本文件是 Rust 语言级通用规则，供 `rust_arch_layer_CLAUDE.md`（经典分层）和 `rust_arch_onion_CLAUDE.md`（洋葱）共用。架构规则（分层、依赖方向、类型归属、端口）以对应的架构文件为准；两者冲突时以架构文件为准。

## 数据与一致性

- 金额、价格、数量统一使用 `rust_decimal::Decimal`，禁止 `f64`/`f32`；同一模块不得混用两种表示。
- 用 `checked_add`/`checked_sub`/`checked_mul`/`checked_div`，溢出返回错误；禁止用 `as` 做金额截断。
- 舍入必须显式：`round_dp_with_strategy`，策略按业务选择，不依赖默认值。禁止 `Decimal::from_f64` 后忽略 `None`；从字符串或整数构造，序列化为字符串。
- 下游写入使用确定性幂等键，数据库加唯一约束；重复消息不能产生重复副作用。
- 业务代码不直接调用 `Utc::now()`/`SystemTime::now()`：把 `now` 作为参数传入，或注入时钟。领域层只接受传入的 `now`，应用层通过 `Clock` 端口取时间。测量延迟和超时用 `Instant`（单调时钟）。
- 契约演进：表结构变更走迁移脚本并保持向后兼容；消息和接口只加可选字段，不改语义、不删字段。这类改动仍需先问（硬性约束第 8 条）。

## 数据访问

- 禁止在循环里逐条查库或调用远程接口（N+1），改为按 ID 集合批量查询（如 `WHERE id = ANY($1)`）；需要时由下层提供批量方法（洋葱架构在端口上提供）。
- 列表查询必须分页并设置上限；不允许无条件的全表查询。
- 批量写入用批量语句（如 sqlx `QueryBuilder::push_values`），不要循环单条写。
- 只查需要的列，不写 `select *`；大字段按需单独查询。
- 事务尽量小：只包住必须原子完成的写操作；查询、计算、远程调用放在事务外。业务编排层（service / application）通过 infra/出站适配器封装的事务类型或端口控制边界，不直接接触 `sqlx::Transaction`。

## 并发

- async 代码中禁止阻塞：`std::thread::sleep`、`block_on`、同步文件/数据库 IO、长时间 CPU 计算；用 `tokio::time::sleep`、异步客户端、`spawn_blocking`。
- 禁止在 `.await` 期间持有 `std::sync::MutexGuard`；需要跨 await 时用 `tokio::sync::Mutex`，或改为消息传递。
- 每个 `tokio::spawn` 必须保留 `JoinHandle` 或加入 `JoinSet`，并在关停时等待；禁止 detach 后不管。
- 任务必须能被取消：使用 `CancellationToken`、`select!` 或 drop 语义；取消时清理资源并向上传播。
- `select!` 的分支必须考虑取消安全：被取消时不能丢失已读取的消息或半写入的状态。
- 优先使用有界 `mpsc::channel`，不用 `unbounded_channel`；必须明确满了的策略（`send().await` 阻塞、`try_send` 丢弃、拒绝受理、熔断），写在代码注释中；处理 `try_send` 的 `Full` 错误，不得静默丢弃。不能丢的数据满时只能拒绝受理或熔断。
- 锁内不做 IO、不 `.await`、不调用可能重入的回调；锁范围尽量短。

## 错误处理与事务

- 每层用 `thiserror` 定义自己的错误类型；`anyhow` 只在组装层和入口层边缘使用（洋葱为 app 和入站适配器），不进入业务层和基础设施层的公开 API。
- 基础设施层（infra / 出站适配器）把第三方错误（`sqlx::Error`、`reqwest::Error`）转换为本层/端口错误，用 `?` + `From` 或显式 `map_err`，不直接暴露给上层。
- 业务路径禁止 `unwrap()`/`expect()`/`panic!`；禁止用 `unwrap_or_default()` 吞掉错误。
- 失败不能只打日志后继续，必须明确采取以下一种处理：重试（有上限、退避、抖动）、死信、熔断、返回错误、停机。
- 事务边界在业务编排层（经典分层为 service，洋葱为 application）：经典分层通过 infra 封装的事务类型控制；洋葱通过 `UnitOfWork` 端口控制。入口层和组装层不开事务，infra/出站适配器只提供事务类型，不自行决定事务范围。
- 事务内不做远程调用：持有事务对象时不能 `.await` 第三方 HTTP、MQ 同步发送；数据库与消息的一致性用 outbox 或提交后发送，不做双写。
- 只有启动阶段遇到无法恢复的配置错误才允许 `panic!`/`expect()`，错误信息必须包含配置项名称。

## 日志、安全与配置

- 日志用 `tracing` 结构化字段：`info!(order_no = %no, "order paid")`，不用 `format!` 拼接消息；错误作为字段记录：`error!(error = ?e, order_no = %no, "pay failed")`。
- 领域层和纯计算代码不打日志，用返回值或领域错误表达结果；日志放在业务编排层（service / application）和外层。
- 链路上下文用 span 传递（`#[instrument(skip(self), fields(order_no = %no))]`）；`tokio::spawn` 新任务时用 `.instrument(span)` 把 span 带过去。
- 日志级别：error 只用于需要人处理的问题，可预期的业务失败用 warn 或 info；循环内和高频路径的日志必须限频。
- 通道积压、丢弃、重试、重连次数必须有指标或限频日志。
- 密钥、token、密码不硬编码，不写进日志和错误信息；含密钥的配置结构手写 `Debug` 并脱敏，不要 derive。
- SQL 一律参数化（`query!`/`bind`），禁止用 `format!` 拼接 SQL。
- 入参格式校验在入口层完成（经典分层为 entry，洋葱为入站适配器）：把外部输入解析成 newtype（如 `OrderNo::parse`），业务层只接收已校验的类型，只校验业务规则；业务不变量由领域对象在构造和状态流转时校验。
- 配置在组装层反序列化为类型化的配置结构并在启动时校验，只在外层使用；内层需要的参数通过构造函数传入，不读环境变量或配置文件。URL、超时、并发数、通道容量不硬编码。
- 所有外部调用（HTTP、DB、Redis、MQ）必须有明确的超时：客户端级超时（如 reqwest `timeout`、连接池 `acquire_timeout`）或 `tokio::time::timeout`。

## Rust 编码细则

- ID、金额、数量用 newtype 区分，不在业务签名里传裸 `String`/`i64`/`Decimal`。
- 实体字段私有，状态变化通过表达业务意图的方法完成（`start_payment`、`mark_paid`），不提供通用 setter，不 `pub` 字段。
- 值对象在构造函数中校验不变量，构造失败返回 `Result`；校验通过后的值不可变。
- 业务枚举的 `match` 不写 `_ =>` 兜底，新增变体时让编译器提示所有需要处理的地方。
- 可见性最小化：默认私有，crate 内共享用 `pub(crate)`，只有真正的对外 API 才 `pub`。
- 不为了通过借用检查随手 `clone()` 或包 `Arc<Mutex<_>>`；先调整所有权和借用，确需共享再用 `Arc`。
- 禁止 `unsafe`，确需使用时先问，并写 `// SAFETY:` 注释说明成立条件。
- 注释写「为什么」，不写「做了什么」；公开 API 的非显然前置条件、`Send`/`Sync` 要求、可能的 panic 写在文档注释里。

## 测试与验证

- 业务计算、状态流转必须有普通 `#[test]` 单元测试，不启动 tokio runtime，覆盖正常、边界、非法输入。
- 业务编排层测试用下层 fake 替换 IO（经典分层用窄 trait 的 fake、函数类型或 `#[cfg(test)]` fake 替换 infra；洋葱用端口的内存 fake，包括 `UnitOfWork` 和 `Clock`），不连接真实数据库、Redis、第三方服务；不要 mock 被测逻辑本身。
- async 测试用 `#[tokio::test]`；时间相关逻辑用 `tokio::time::pause`/`advance`，不要真实 sleep。
- 基础设施层（infra / 出站适配器）用集成测试或契约测试验证协议、序列化、重连、事务回滚；不要 mock 掉被测的转换逻辑。
- 测试必须确定性：不依赖当前时间、随机数、真实网络、执行顺序；时间通过参数或 fake `Clock` 注入。
- 禁止为了变绿加 `#[ignore]`、修改断言、删除用例、放宽 clippy 或超时；测试只加严不放宽。
- 覆盖率不是目标，但核心业务分支必须有测试；新增分支不得没有测试。
