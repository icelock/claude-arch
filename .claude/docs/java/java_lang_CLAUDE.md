# CLAUDE.md

本文件是 Java 语言级通用规则，供 `java_arch_layer_CLAUDE.md`（经典分层）和 `java_arch_onion_CLAUDE.md`（洋葱）共用。架构规则（分层、依赖方向、类型归属、端口）以对应的架构文件为准；两者冲突时以架构文件为准。

## 数据与一致性

- 金额、价格、数量统一使用 `BigDecimal`，禁止 `double`/`float`/`Double`，禁止在金额路径调用 `doubleValue()`/`floatValue()`；上游给的是 double 时先转字符串。
- 禁止 `new BigDecimal(0.1)`；用 `new BigDecimal("0.1")`、`BigDecimal.valueOf(long)` 或 `BigDecimal.valueOf(String)`。金额比较用 `compareTo`，不用 `equals`（`equals` 会比较 scale，`1.0` 不等于 `1.00`）。
- 除法必须指定 scale 和 `RoundingMode`；任何隐式舍入都视为错误。
- 下游写入使用确定性幂等键，数据库加唯一约束；重复消息不能产生重复副作用。
- 业务代码不直接调用 `Instant.now()`/`LocalDateTime.now()`/`System.currentTimeMillis()`：把 `now` 作为参数传入，或注入 `java.time.Clock`。领域层和纯计算代码只接受传入的 `now`，不注入时钟。测量延迟和超时用 `System.nanoTime()`。
- 契约演进：表结构变更走迁移脚本并保持向后兼容；消息和接口只加可选字段，不改语义、不删字段。这类改动仍需先问（硬性约束第 8 条）。

## 数据访问

- 禁止在循环里逐条查库或调用远程接口（N+1），改为按 ID 集合批量查询；需要时由下层提供批量方法（洋葱架构在端口上提供）。
- 列表查询必须分页并设置上限；不允许无条件的全表查询。
- 批量写入用批量 API（如 `saveBatch`、批量 `INSERT`），不要循环单条写。
- 只查需要的列，不写 `select *`；大字段按需单独查询。
- 事务尽量小：只包住必须原子完成的写操作，查询、计算、远程调用放在事务外。

## 并发

- 禁止 `new Thread()`、匿名线程和默认 `ForkJoinPool.commonPool()`。使用有名字、有界队列、明确拒绝策略的 `ThreadPoolExecutor`。
- `@Async` 必须指定自定义线程池；`CompletableFuture` 必须有超时，禁止无超时的 `.get()`/`.join()`。
- 有界队列必须明确队列满时的策略（阻塞、丢弃、拒绝受理、熔断），写在代码注释中；不能丢的数据，队列满时只能拒绝受理或熔断，不能丢弃。
- 禁止在 IO 回调线程、`@EventListener` 同步链路中做 DB、HTTP、文件 IO 或 `Thread.sleep`。
- 共享可变状态用 `ConcurrentHashMap`、`Atomic*`、`ReentrantLock` 或不可变对象；禁止 `synchronized(this)` 锁住整个对象。
- 锁内不做 IO、不调用回调、不获取其他锁；需要多把锁时按固定顺序获取。
- 禁止共享 `SimpleDateFormat`，用不可变的 `DateTimeFormatter`。
- 捕获 `InterruptedException` 后必须 `Thread.currentThread().interrupt()`，不要吞掉中断。

## 错误处理与事务

- 业务异常定义在业务层：经典分层在 service 或 common；洋葱在 domain（领域异常）和 application（应用异常、端口异常）。入口层统一转换为响应（如 `@RestControllerAdvice`）。
- 基础设施层（经典分层的 infra、洋葱的出站适配器）把第三方异常（`SQLException`、`IOException`、SDK 异常）转换为本层/端口约定的异常，不直接抛给上层。
- 禁止空 `catch`、`catch (Exception e) { log.error(...) }` 后继续执行、`printStackTrace()`；失败必须明确采取以下一种处理：重试（有上限和退避）、死信、熔断、返回错误、停机。
- `@Transactional` 只加在业务编排层的 public、非 final 方法上（经典分层为 service，洋葱为 application）。同类自调用不经过 Spring 代理，事务不生效，必须拆到另一个 Bean 或改用编程式事务。
- 受检异常默认不回滚，需要回滚时显式写 `rollbackFor = Exception.class`；使用非默认的传播行为（如 `REQUIRES_NEW`、`NESTED`）时必须显式声明并注释原因，默认的 `REQUIRED` 不用写。
- 事务内不做远程调用（第三方 HTTP、MQ 同步发送）；数据库与消息的一致性用 outbox 或 `TransactionSynchronization.afterCommit`，不做双写。
- 入口层、基础设施层和组装层不开事务。

## 日志、安全与配置

- 日志用 SLF4J 占位符：`log.info("order {} paid", orderId)`，不拼接字符串；异常作为最后一个参数传入（`log.error("pay failed, order={}", id, e)`），保留堆栈。
- 领域层和纯计算代码不打日志，用返回值或业务异常表达结果；日志放在业务编排层（service / application）和外层。
- 链路标识通过 MDC 传递（如 `traceId`）；切换线程池时要把 MDC 一起传过去。
- 日志级别：error 只用于需要人处理的问题，可预期的业务失败用 warn 或 info；循环内和高频路径的日志必须限频。
- 队列积压、丢弃、重试、重连次数必须有指标或限频日志。
- 密钥、token、密码不硬编码，不写进日志和异常信息；从配置中心或环境变量读取。
- MyBatis 只用 `#{}`，禁止 `${}` 拼接用户输入；手写 SQL 一律参数化。
- 入参格式校验在入口层完成（经典分层为 entry，洋葱为入站适配器；`@Validated` + Bean Validation）；业务规则和不变量由业务层/领域对象在构造和状态流转时校验。
- 配置用 `@ConfigurationProperties` + `@Validated` 绑定成配置类，只在组装/外层读取，不在各处散落 Spring 的 `@Value`；内层需要的参数通过构造器传入，不读配置。URL、超时、线程数、队列长度不硬编码。
- 所有外部调用（HTTP、RPC、DB、Redis、MQ）必须设置明确的连接超时和读超时。

## Java 编码细则

- Lombok 只用 `@RequiredArgsConstructor`、`@Getter`、`@Builder`、`@Slf4j`；Entity/PO 上禁止 `@Data`（它生成的 `equals`/`hashCode` 包含可变字段），禁止 `@SneakyThrows`。
- 值对象和 DTO 优先用 record，并在紧凑构造器中校验不变量；集合字段对外暴露时返回不可变视图。实体不暴露 setter，状态变化通过表达业务意图的方法完成。
- 方法不返回 null 集合，返回空集合；`Optional` 只用作返回值，不用作字段和参数。
- 注释写「为什么」，不写「做了什么」；公开方法的非显然前置条件、线程安全性写在 Javadoc 里。

## 测试与验证

- 业务计算、状态流转必须有纯 JUnit 5 + AssertJ 单元测试，不启动 Spring，覆盖正常、边界、非法输入。
- 业务编排层测试用 Mockito mock 下层依赖（经典分层 mock infra 具体类，洋葱 mock 端口）或使用内存 fake；不连接真实数据库、Redis、第三方服务。
- 基础设施层（infra / 出站适配器）用集成测试或契约测试验证协议、序列化、重连；不要 mock 掉被测的转换逻辑。
- 测试必须确定性：不依赖当前时间、随机数、真实网络、执行顺序；注入固定的 `Clock`。禁止用 `Thread.sleep` 等待异步结果，用 Awaitility 或带超时的 `CompletableFuture`。
- 禁止为了变绿修改断言、删除用例、加 `@Disabled`、放宽超时；测试只加严不放宽。
- 覆盖率不是目标，但核心业务分支必须有测试；新增分支不得没有测试。
