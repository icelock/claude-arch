# CLAUDE.md

## 架构声明

本项目采用**经典分层架构**：依赖只能自上而下单向流动，层是封闭的，不能跳层。组装层（根包、`config`、`bootstrap`）是组装根。

```
组装 ──> entry ──> service ──> infra ──> common
  └────────┴──────────┴──────────┴─────> common
```

**组装层（根包、`config`、`bootstrap`）可以依赖所有层**：它只做接线，是唯一同时知道所有具体实现的地方；任何层都不能反过来依赖组装层。

common 是共享内核，各层依赖 common 不算跳层；除此之外，entry 不能直接依赖 infra。

**这不是 Clean 架构、六边形架构或依赖倒置。** service 直接依赖 infra 的具体类。未经用户明确同意，不要：

- 在 service 定义端口接口、再由 infra 实现；
- 把 infra 的类抽成接口层；
- 把分层改造成洋葱或整洁架构。

单测可以用 Mockito mock infra 的具体类，不要为了测试在生产代码里引入端口层。

## 本项目事实

> 拷贝模板后先填写本节。AI 依据这里的版本选择语法和 API；仍是 `<填写>` 的项先问用户，不要猜。

- JDK：`<填写：17 / 21>`（决定能否用 record、switch 模式匹配、虚拟线程）
- Spring Boot：`<填写：2.7 / 3.x>`（3.x 用 `jakarta.*`，2.x 用 `javax.*`，不能混用）
- 持久层：`<填写：MyBatis-Plus / MyBatis / JPA>`；日志：`<填写：SLF4J + Logback>`；表结构迁移：`<填写：Flyway / Liquibase / 无>`
- 构建与测试：`<填写，如 mvn -q test>`
- 工程结构：`<填写：单模块多包 / 多模块>`。
  - 单模块：层只按包划分。
  - 多模块：每层一个 Maven 模块（如 `xxx-boot`、`xxx-entry`、`xxx-service`、`xxx-infra`、`xxx-common`），模块之间的依赖方向由 `pom.xml` 保证，模块内的包名仍按下方层级表。

## 规则优先级与工作方式

规则冲突时按以下顺序取舍：**用户当次明确指令 > 正确性与安全 > 本文件的架构规则 > 仓库现有写法 > 通用最佳实践**。例外：安全类规则（密钥、注入、权限校验）即使用户要求放宽，也要先说明风险并得到确认。现有代码违反本文件时不要模仿，新代码按本文件写。

- 跨层改动或改动超过 3 个文件时，先用几行说明计划：落在哪层、改哪些文件、是否涉及「必须停下来问」的事项，再动手。
- 修 bug 先写一个能复现问题的失败测试，再修代码，让它变绿。
- 小的歧义（命名、放在同层哪个子包、日志措辞）不要打断用户：选仓库惯例或最保守的做法，在回复中写明假设。
- 完成时按以下格式汇报：
  1. 改动文件及所属层；
  2. 关键决策和假设；
  3. 执行的检查命令和真实输出（失败也照贴）；
  4. 遗留问题和风险，包括发现但没有修的历史违规。

## 对 AI 的硬性约束

本节是 MUST，优先于你的默认习惯和所谓的「最佳实践」。

1. **先定位再写**：动手前确定代码属于哪一层、哪个 Maven 模块和包。落不进「本项目的层级」表就停下来问，不要自创模块或包。
2. **先读后写**：找同层 1–2 个现有实现，沿用其结构、命名、异常处理和日志方式。不要引入仓库里没有的新风格。
3. **最小改动**：只改与需求相关的代码。不顺手重构、不重排 import、不升级依赖、不改代码格式化规则。
4. **抽象准入**：没有第二个实现、不属于真实 IO 边界时，不建接口、抽象类、工厂、泛型 Repository、兼容层、fallback、feature flag。三行重复优于一个错误抽象。
5. **依赖准入**：不新增第三方库或 Spring Starter。确需新增时，先说明用途、体积、维护状态，等用户确认后再改 `pom.xml`。
6. **不写占位**：不留 TODO、mock 返回值、假数据、注释掉的代码、未使用的参数或字段。
7. **验证真实**：完成前运行「检查」一节列出的全部检查，贴出命令和真实输出。失败就说失败并继续修，禁止改断言、`@Disabled`、放宽超时、更新快照来让结果变绿。
8. **必须停下来问**：层级归属不明；新增模块或层；新增依赖；修改公共类型、对外接口、表结构、消息格式；修复历史违规会超出本次需求的改动范围。

## 分层与依赖

### 层的角色

每一层必须归入以下某种角色。可以按业务拆子包，但不存在不属于任何角色的层。

| 角色 | 职责 | 典型内容 |
|---|---|---|
| 组装 | Bean 装配、配置绑定、启动与生命周期，只做接线 | `@SpringBootApplication`、`@Configuration`、`ApplicationRunner`、`SmartLifecycle` |
| 入口 | 接收外部请求或定时触发，转换和校验后调用业务层 | Controller、MQ Listener、`@Scheduled` 定时任务 |
| 业务 | 业务规则与流程编排 | 计算、决策、状态流转、结果处理、事务边界 |
| 基础设施 | 技术细节 | 第三方客户端、WebSocket 客户端、Mapper/Repository、Redis、Kafka、文件、缓存加载 |
| 公共 | 各层共用的只读类型与纯工具，不依赖任何层 | 跨层 DTO、枚举、常量、异常基类、纯函数工具 |

### 本项目的层级

> 下表为示例，按实际项目修改。在已有包下新增子包不需要改。

| 层 | 包（相对根包 `com.xxx`） | 可以依赖 | 外部依赖限制 |
|---|---|---|---|
| 组装 | 根包、`config..`、`bootstrap..` | 所有层 | 无 |
| entry | `controller..`、`listener..`、`schedule..` | service、common | 不直接用 Mapper、RedisTemplate、KafkaTemplate |
| service | `service..` | infra、common | 不用 Redis、Kafka、MyBatis 等中间件 API |
| infra | `infra..`、`websocket..` | common | 无 |
| common | `dto..`、`enums..`、`utils..` | 无 | 只允许 JDK 和基础工具库 |

- 所有类都必须属于上表某一层；根包只放 `@SpringBootApplication` 启动类，`@Configuration` 放 `config`，启动预热、生命周期管理（`ApplicationRunner`、`SmartLifecycle`）放 `bootstrap`。新增顶层包落不进上表就停下来问。
- 包和模块之间不能有环，Spring Bean 之间也不能循环依赖。

### 依赖规则

- 上层可以依赖下层，**下层禁止依赖上层**。Maven 多模块由 `pom.xml` 保证，单模块多包写代码时自行保证。
- 禁止通过容器或静态状态绕过依赖规则：`ApplicationContext.getBean(...)`、`SpringContextHolder`、静态字段保存 Bean、`@PostConstruct` 里手工注册。
- 所有依赖通过构造器注入，字段用 `final`；禁止字段注入（`@Autowired` 直接标在字段上）。
- 业务层不直接使用中间件或 SDK 的 API（`RedisTemplate`、`KafkaTemplate`、Mapper、HTTP/WS 客户端），必须通过 infra 封装的类访问。
- 基础设施层只处理技术问题（协议、签名、序列化、连接、重连、持久化），不做业务判断。
- 入口层只做参数转换、校验和调用，不写业务逻辑，不直接调用 infra。
- 定时任务放 `schedule` 包，归入口层：只负责触发并调用 service，不写业务逻辑，不直接调用 infra。
- 公共层不依赖 Spring、数据库、网络库。

### 新代码放哪里

- 新的外部系统接入 → infra，协议和报文转换在该层完成。
- 新的业务规则或流程 → service；计算逻辑尽量写成不依赖 Spring 和 IO 的普通类。
- 新的 HTTP 接口、消息消费或定时任务 → entry（`controller`、`listener`、`schedule`），只做转换和调用。
- 新的启动预热或生命周期管理 → 组装层的 `bootstrap`，只做接线和启停，不写业务逻辑。
- 多层共用的只读数据 → common 的 `dto`；只在一层内使用的类型留在该层，不要提前上移。

## 跨边界数据与下层通知上层

### 类型归属

- `dto` 只放跨层传递的只读数据对象，不可变优先，用 record。
- PO、Entity、外部系统原始报文、SDK 请求/响应对象只在 infra 内使用，不得进入 common；返回上层前转换为 service 需要的类型。
- 只服务于某个 HTTP 接口的请求/响应对象留在 entry，不要传进 service。

### 方式一：回调接口（单次结果、请求-回报型）

1. 接口定义在下层，与调用它的类放在一起；签名只用 infra 或 common 的类型。
2. 上层实现该接口。
3. Spring 注入把实现交给下层，下层只持有接口类型。

```java
// infra：下层定义回调接口
public interface PaymentResultListener { void onPaymentResult(PaymentResultEvent event); }

@Component
@RequiredArgsConstructor
public class PaymentClient {
    private final PaymentResultListener listener;
    public void pay(String orderNo, BigDecimal amount) { /* 发起支付；结果异步到达后调用 listener */ }
}

// service：发起支付，只注入 PaymentClient
@Service
@RequiredArgsConstructor
public class OrderService { private final PaymentClient paymentClient; }

// service：单独的 Bean 实现回调（service -> infra，方向合法），不注入 PaymentClient
@Service
public class PaymentResultHandler implements PaymentResultListener {
    @Override public void onPaymentResult(PaymentResultEvent event) { /* 更新订单状态 */ }
}
```

- **实现回调接口的 Bean 不能再注入持有该接口的下层 Bean**：上例若让 `OrderService` 同时实现 `PaymentResultListener`，`PaymentClient` 与 `OrderService` 的构造器会互相等待，启动时报循环依赖。包依赖是合法的，但运行期会成环。
- 多个实现同时接收时注入 `List<PaymentResultListener>`，逐个调用。只有一个调用方且无需解耦时，直接调用下层，不要为形式加接口。

### 方式二：事件或队列（持续推送、流式数据）

- 低频事件：事件类定义在下层或 common，用 `ApplicationEventPublisher` 发布，上层 `@EventListener` 消费。默认同步，需要异步时配 `@Async` 并指定线程池。
- 高频推送：下层投递到有界 `BlockingQueue`，上层在自己的线程消费。队列在组装层用 `@Bean` 创建（如 `new ArrayBlockingQueue<>(10_000)`），两端分别注入。

### 跨边界约束

- 接口和事件类的签名只能使用下层或 common 的类型，不能出现上层类型。
- 事件携带转换后的类型，不传原始报文或 SDK 类型；参数多时封装成事件对象。
- 不要把上层自己注册给下层（如 `@PostConstruct` 中 `client.setListener(this)`），由注入完成装配。
- 回调运行在 IO 线程（WS、Netty、MQ 客户端）上时，耗时处理必须投递到业务线程池，不能阻塞回调线程。

## 数据与一致性

- 金额、价格、数量统一使用 `BigDecimal`，禁止 `double`/`float`/`Double`，禁止在金额路径调用 `doubleValue()`/`floatValue()`；上游给的是 double 时先转字符串。
- 禁止 `new BigDecimal(0.1)`；用 `new BigDecimal("0.1")`、`BigDecimal.valueOf(long)` 或 `BigDecimal.valueOf(String)`。金额比较用 `compareTo`，不用 `equals`（`equals` 会比较 scale，`1.0` 不等于 `1.00`）。
- 除法必须指定 scale 和 `RoundingMode`；任何隐式舍入都视为错误。
- 下游写入使用确定性幂等键，数据库加唯一约束；重复消息不能产生重复副作用。
- 业务代码不直接调用 `Instant.now()`/`LocalDateTime.now()`/`System.currentTimeMillis()`：注入 `java.time.Clock` 或把 `now` 作为参数；测量延迟和超时用 `System.nanoTime()`。
- 契约演进：表结构变更走迁移脚本并保持向后兼容；消息和接口只加可选字段，不改语义、不删字段。这类改动仍需先问（硬性约束第 8 条）。

## 数据访问

- 禁止在循环里逐条查库或调用远程接口（N+1），改为按 ID 集合批量查询。
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

- infra 把第三方异常（`SQLException`、`IOException`、SDK 异常）转换为本层异常，不直接抛给上层。
- 业务异常定义在 service 或 common；entry 用 `@RestControllerAdvice` 统一转换为响应。
- 禁止空 `catch`、`catch (Exception e) { log.error(...) }` 后继续执行、`printStackTrace()`；失败必须明确采取以下一种处理：重试（有上限和退避）、死信、熔断、返回错误、停机。
- `@Transactional` 只加在 service 的 public、非 final 方法上。同类自调用不经过 Spring 代理，事务不生效，必须拆到另一个 Bean 或改用编程式事务。
- 受检异常默认不回滚，需要回滚时显式写 `rollbackFor = Exception.class`；使用非默认的传播行为（如 `REQUIRES_NEW`、`NESTED`）时必须显式声明并注释原因，默认的 `REQUIRED` 不用写。
- 事务内不做远程调用（第三方 HTTP、MQ 同步发送）；数据库与消息的一致性用 outbox 或 `TransactionSynchronization.afterCommit`，不做双写。
- 不在 entry、infra 或组装层开事务。

## 日志、安全与配置

- 日志用 SLF4J 占位符：`log.info("order {} paid", orderId)`，不拼接字符串；异常作为最后一个参数传入（`log.error("pay failed, order={}", id, e)`），保留堆栈。
- 链路标识通过 MDC 传递（如 `traceId`）；切换线程池时要把 MDC 一起传过去。
- 日志级别：error 只用于需要人处理的问题，可预期的业务失败用 warn 或 info；循环内和高频路径的日志必须限频。
- 队列积压、丢弃、重试、重连次数必须有指标或限频日志。
- 密钥、token、密码不硬编码，不写进日志和异常信息；从配置中心或环境变量读取。
- MyBatis 只用 `#{}`，禁止 `${}` 拼接用户输入；手写 SQL 一律参数化。
- 入参校验在 entry 完成（`@Validated` + Bean Validation），service 只校验业务规则。
- 配置用 `@ConfigurationProperties` + `@Validated` 绑定成配置类，不在各处散落 Spring 的 `@Value`；URL、超时、线程数、队列长度不硬编码。
- 所有外部调用（HTTP、RPC、DB、Redis、MQ）必须设置明确的连接超时和读超时。

## Java 编码细则

- Lombok 只用 `@RequiredArgsConstructor`、`@Getter`、`@Builder`、`@Slf4j`；Entity/PO 上禁止 `@Data`（它生成的 `equals`/`hashCode` 包含可变字段），禁止 `@SneakyThrows`。
- 值对象和 DTO 优先用 record；集合字段对外暴露时返回不可变视图。
- 方法不返回 null 集合，返回空集合；`Optional` 只用作返回值，不用作字段和参数。
- 注释写「为什么」，不写「做了什么」；公开方法的非显然前置条件、线程安全性写在 Javadoc 里。

## 测试与验证

- 业务计算、状态流转必须有纯 JUnit 5 + AssertJ 单元测试，不启动 Spring，覆盖正常、边界、非法输入。
- service 测试用 Mockito mock infra 具体类或使用内存 fake；不连接真实数据库、Redis、第三方服务。
- infra 用集成测试或契约测试验证协议、序列化、重连；不要 mock 掉被测的转换逻辑。
- 测试必须确定性：不依赖当前时间、随机数、真实网络、执行顺序；注入固定的 `Clock`。禁止用 `Thread.sleep` 等待异步结果，用 Awaitility 或带超时的 `CompletableFuture`。
- 禁止为了变绿修改断言、删除用例、加 `@Disabled`、放宽超时；测试只加严不放宽。
- 覆盖率不是目标，但核心业务分支必须有测试；新增分支不得没有测试。

## AI 常见劣化模式

| 劣化写法 | 正确做法 |
|---|---|
| service 直接注入 `RedisTemplate`/`KafkaTemplate`/Mapper | 通过 infra 封装访问 |
| 把类型挪到 common 让两层都能用 | 类型归真正拥有它的层，边界做转换 |
| 单实现也建 `IXxxService` + `XxxFactory` + `AbstractXxx`，或加 `if (useNew)` 兼容层 | 直接用具体类，保持单一路径 |
| 同一个 Bean 既实现下层回调接口，又注入该下层类 | 回调拆到单独的 Bean |
| 循环里逐条查库、列表不分页 | 批量查询，分页并设上限 |
| `catch (Exception e) { log.error(...) }` 后继续 | 转换后抛出，或明确重试、死信、熔断 |
| `@Transactional` 加在私有方法上或通过自调用触发 | 拆到另一个 Bean 或用编程式事务 |
| controller、listener、schedule 里写业务逻辑 | 入口只转换和调用，逻辑放 service |
| 改断言、加 `@Disabled`、放宽超时让测试变绿 | 修代码；测试只加严 |
| 只跑单个测试就宣称全部通过 | 跑完整检查并贴输出 |

## 检查方式与完成定义

### 检查

- `mvn test`
- 以下规则靠自查和 review：金额构造与舍入、事务内远程调用、锁内 IO、日志中的敏感信息、队列满时的策略。

### 完成定义（DoD）

- [ ] 代码在正确的层和模块，依赖方向符合本文件。
- [ ] `mvn test` 全部通过，并贴出真实输出。
- [ ] 新增业务分支有单元测试，覆盖失败路径；修 bug 有复现测试。
- [ ] 没有 TODO、注释掉的代码、调试日志、未使用的参数。
- [ ] 错误处理、日志、超时与中断、配置方式符合本文件。
- [ ] 没有新增依赖、接口、工厂或兼容层；如确有必要，已在回复中说明理由。
- [ ] 已按「工作方式」的格式汇报。

## 已知的历史违规

> 记录现存不符合上述规则的代码，避免被模仿。只在本次需求本来就要改到的代码里顺带修正；超出这个范围的不动，在汇报中列出。没有则保持为空。

- （无）

## 附录：实时交易系统（可选）

> 非交易类项目删除本节。本节规则补充正文，冲突时以本节为准。

- 交易所报文、签名、限频（rate limit）、错误码只在 infra 处理；返回上层前转换为业务类型，不把交易所字段名带进 service。
- 下单使用确定性的 `clientOrderId` 作为幂等键，超时或网络错误后先按它查询订单状态再决定是否重下，禁止盲目重试。
- 行情、订单推送必须处理重复、乱序、gap、断线重连：出现 gap 时作废本地缓存并重拉快照，不能跳过继续用；重连后必须重新订阅。
- 背压分级：行情类可以丢弃并置 gap 标记；订单、回报、成交类不能丢，队列满时拒绝受理或熔断。
- WS/Netty 回调线程只做解析和投递，不查库、不发 HTTP、不 `Thread.sleep`。
- 热路径禁止逐笔日志、字符串拼接、JSON 序列化、反射、无界重试和不必要的对象分配。
- 价格和数量按交易对的 tickSize/stepSize 显式取整，舍入方向由业务决定（买卖方向不同），写在代码里，不依赖默认值。
