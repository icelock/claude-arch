# CLAUDE.md

## 架构声明

本项目采用**经典分层架构**：依赖只能自上而下单向流动，层是封闭的，不能跳层。`boot` 是组装根。

```
boot ──> entry ──> service ──> infra ──> common
  └────────┴──────────┴────────────────> common
```

**这不是 Clean 架构、六边形架构或依赖倒置。** service 直接依赖 infra 的具体类。未经用户明确同意，不要：

- 在 service 定义端口接口、再由 infra 实现；
- 把 infra 的类抽成接口层；
- 把分层改造成洋葱或整洁架构。

单测可以用 Mockito mock infra 的具体类，不要为了测试在生产代码里引入端口层。

## 对 AI 的硬性约束

本节是 MUST，优先于你的默认习惯和所谓的「最佳实践」。

1. **先定位再写**：动手前确定代码属于哪一层、哪个 Maven 模块和包。落不进「本项目的层级」表就停下来问，不要自创模块或包。
2. **先读后写**：找同层 1–2 个现有实现，沿用其结构、命名、异常处理和日志方式。不要引入仓库里没有的新风格。
3. **最小改动**：只改与需求相关的代码。不顺手重构、不重排 import、不升级依赖、不改代码格式化规则。
4. **抽象准入**：没有第二个实现、不属于真实 IO 边界时，不建接口、抽象类、工厂、泛型 Repository、兼容层、fallback、feature flag。三行重复优于一个错误抽象。
5. **依赖准入**：不新增第三方库或 Spring Starter。确需新增时，先说明用途、体积、维护状态，等用户确认后再改 `pom.xml`。
6. **不写占位**：不留 TODO、mock 返回值、假数据、注释掉的代码、未使用的参数或字段。
7. **验证真实**：完成前运行本节列出的全部检查，贴出命令和真实输出。失败就说失败并继续修，禁止改断言、`@Disabled`、放宽超时、更新快照或放宽 ArchUnit 规则来让结果变绿。
8. **必须停下来问**：层级归属不明；新增模块或层；新增依赖；修改公共类型、对外接口、表结构、消息格式；需要放宽架构检查；修复历史违规会显著扩大改动范围。

## 分层与依赖

### 层的角色

每一层必须归入以下某种角色。可以按业务拆子包，但不存在不属于任何角色的层。

| 角色 | 职责 | 典型内容 |
|---|---|---|
| 入口 | 接收外部触发，转换后调用业务层 | Controller、WS 推送接收、MQ Consumer、定时任务、启动监听 |
| 业务 | 业务规则与流程编排 | 计算、决策、状态流转、结果处理、事务边界 |
| 基础设施 | 技术细节 | 交易所/第三方客户端、Mapper/Repository、Redis、Kafka、文件、缓存加载 |
| 公共 | 各层共用的只读类型与纯工具，不依赖任何层 | 枚举、常量、哨兵异常基类、值类型、纯函数工具 |
| 组装 | Bean 装配、配置绑定、生命周期，只做接线 | `@SpringBootApplication`、`@Configuration` |

### 本项目的层级

> 下表为示例，按实际项目修改。新增层、拆分模块或包时，同步更新本表、依赖图和 ArchUnit 配置。

| 层 | Maven 模块 / 包 | 可以依赖 | 外部依赖限制 |
|---|---|---|---|
| boot | `xxx-boot` / `com.xxx.boot`、`com.xxx.config` | 所有层 | 无 |
| entry | `xxx-entry` / `com.xxx.controller`、`com.xxx.listener`、`com.xxx.schedule` | service、common | 不直接用 Mapper、RedisTemplate、KafkaTemplate |
| service | `xxx-service` / `com.xxx.service` | infra、common | 无 Spring 数据/消息/HTTP 客户端依赖 |
| infra | `xxx-infra` / `com.xxx.infra` | common | 无 |
| common | `xxx-common` / `com.xxx.common` | 无 | 只允许 JDK 和基础工具库 |

- 单模块多包时，下层包不得 import 上层包，由 ArchUnit 保证。
- 依赖图不能有环，包括 Spring Bean 之间的循环依赖。

### 依赖规则

- 上层可以依赖下层，**下层禁止依赖上层**。Maven 多模块由 `pom.xml` 保证；单模块多包由 ArchUnit 保证。
- 禁止通过容器或静态状态绕过依赖规则：`ApplicationContext.getBean(...)`、`SpringContextHolder`、静态字段保存 Bean、`@PostConstruct` 里手工注册。
- 所有依赖通过构造器注入，字段用 `final`；禁止字段注入（`@Autowired` 直接标在字段上）。
- 业务层不直接使用中间件或 SDK 的 API（`RedisTemplate`、`KafkaTemplate`、Mapper、HTTP/WS 客户端），必须通过 infra 封装的类访问。
- 基础设施层只处理技术问题（协议、签名、序列化、连接、重连、持久化），不做业务判断。
- 入口层只做参数转换、校验和调用，不写业务逻辑，不直接调用 infra。
- 公共层不依赖 Spring、数据库、网络库。**common 只放跨层共用的只读类型、枚举、常量、异常基类；禁止放请求/响应 DTO、PO、Entity。**
- 数据库 PO 只在 infra 内使用，返回上层前转换为业务类型。

### 新代码放哪里

- 新的外部系统接入 → infra，协议和报文转换在该层完成。
- 新的业务规则或流程 → service；计算逻辑尽量写成不依赖 Spring 和 IO 的普通类。
- 新的触发方式 → entry，只做转换和调用。
- 多层共用的只读类型 → common；只在一层内使用的类型留在该层。
- 交易所报文 DTO 与业务 DTO 分开命名和存放；PO 不得离开 infra。

## 跨边界数据与下层通知上层

### 类型归属

- PO、Entity、交易所原始报文、SDK 请求/响应对象只在 infra 内使用；返回前转换为 service 需要的类型。
- 入站请求/响应 DTO 只在 entry 使用，不要传进 service。

### 方式一：回调接口（单次结果、请求-回报型）

1. 接口定义在下层，与调用它的类放在一起；签名只用 infra 或 common 的类型。
2. 上层实现该接口。
3. Spring 注入把实现交给下层，下层只持有接口类型。

```java
// infra：下层定义接口
public interface OrderResultListener { void onOrderResult(OrderResultEvent event); }

@Component
@RequiredArgsConstructor
public class OrderClient { private final OrderResultListener listener; }

// service：实现下层接口（service -> infra，方向合法）
@Service
public class HedgeService implements OrderResultListener {
    @Override public void onOrderResult(OrderResultEvent event) { /* ... */ }
}
```

多个实现同时接收时注入 `List<OrderResultListener>`，逐个调用。只有一个调用方且无需解耦时，直接调用下层，不要为形式加接口。

### 方式二：事件或队列（持续推送、流式数据）

- 低频事件：事件类定义在下层或 common，用 `ApplicationEventPublisher` 发布，上层 `@EventListener` 消费。默认同步，需要异步时配 `@Async` 和指定线程池。
- 高频推送：下层投递到 `BlockingQueue`/Disruptor，上层在自己的线程消费。队列在 boot 用 `@Bean` 创建（如 `new ArrayBlockingQueue<>(10_000)`），两端分别注入；有界队列必须明确满了的策略（阻塞、丢弃、告警），写在代码注释中；订单/回报/成交类不能丢，满了应拒绝受理或熔断。
- 队列满、丢弃、延迟、重连次数必须有指标或限频日志。

### 跨边界约束

- 接口和事件类的签名只能使用下层或 common 的类型，不能出现上层类型。
- 事件携带转换后的类型，不传交易所原始报文或 SDK 类型；参数多时封装成事件对象。
- 不要把上层自己注册给下层（如 `@PostConstruct` 中 `client.setListener(this)`），由注入完成装配。
- 回调运行在 IO/WS/Netty 线程上时，耗时处理必须投递到业务线程池，不能阻塞回调线程。

## 数据、时间与一致性

- 价格、数量、金额统一使用 `BigDecimal`，禁止 `double`/`float`/`Double`，禁止在金额路径调用 `doubleValue()`/`floatValue()`；上游 double 先转字符串。
- 禁止 `new BigDecimal(0.1)`；用 `new BigDecimal("0.1")`、`BigDecimal.valueOf(long)` 或 `BigDecimal.valueOf(String)`。金额比较用 `compareTo`，不用 `equals`。
- 除法必须指定 scale 和 `RoundingMode`；任何隐式舍入都视为错误。
- 下游写入使用确定性幂等键，数据库加唯一约束；重复消息不能产生重复副作用。
- 消息流必须处理重复、乱序、gap、断线重连：gap 时作废本地缓存并重拉快照，不能跳过继续用；重连后必须重订阅。
- 业务代码不直接调用 `Instant.now()`/`System.currentTimeMillis()`：注入 `java.time.Clock` 或把 `now` 作为参数；延迟和超时用 `System.nanoTime()`。

## 并发与性能

- 禁止 `new Thread()`、匿名线程和默认 `ForkJoinPool.commonPool()`。使用有名字、有界队列、明确拒绝策略的 `ThreadPoolExecutor`。
- `@Async` 必须指定自定义线程池；`CompletableFuture` 必须有超时，禁止无超时 `.get()`/`.join()`。
- 禁止在 Netty/WS 回调线程、`@EventListener` 同步链路中做 DB、HTTP、文件 IO 或 `Thread.sleep`。
- 共享可变状态用 `ConcurrentHashMap`、`Atomic*`、`ReentrantLock` 或不可变对象；禁止 `synchronized(this)` 锁住整个对象。
- 锁内不做 IO、不调用回调、不获取其他锁；多锁按固定顺序获取。
- 禁止共享 `SimpleDateFormat`，用 `DateTimeFormatter`（不可变）；禁止静态可变 `Calendar`。
- 热路径禁止逐笔日志、字符串拼接、JSON 序列化、反射、无界重试和不必要的对象分配。
- 捕获 `InterruptedException` 后必须 `Thread.currentThread().interrupt()`，不要吞掉中断。

## 错误处理与事务

- infra 把第三方异常（`SQLException`、`IOException`、SDK 异常）转换为本层异常，不直接抛给上层。
- 业务异常定义在 service 或 common；entry 用 `@RestControllerAdvice` 统一转换为响应。
- 禁止空 `catch`、`catch (Exception e) { log.error(...) }` 后继续执行、`printStackTrace()`；失败必须明确重试（有上限和退避）、死信、熔断、返回错误或停机中的一种。
- `@Transactional` 只加在 service 的 public、非 final 方法上；同类自调用不经过代理，事务不生效，必须拆到另一个 Bean 或改用编程式事务。
- 受检异常默认不回滚，需要回滚时显式 `rollbackFor = Exception.class`；传播行为显式声明，不依赖默认值。
- 事务内不做远程调用（交易所、HTTP、MQ 同步发送）；数据库与消息的一致性用 outbox 或 `TransactionSynchronization.afterCommit`，不做双写。
- 不在 entry 或 infra 开事务。

## 测试与验证

- 领域计算、状态流转必须有纯 JUnit 5 + AssertJ 单元测试，不启动 Spring，覆盖正常、边界、非法输入。
- service 测试用 Mockito mock infra 具体类或使用内存 fake；不连接真实数据库、Redis、交易所。
- infra 用集成测试或契约测试验证协议、序列化、重连；不要 mock 掉被测的转换逻辑。
- 测试必须确定性：不依赖当前时间、随机数、真实网络、执行顺序；注入固定 `Clock`；禁止用 `Thread.sleep` 等待异步结果，用 Awaitility 或有界 `CompletableFuture` 等待。
- 禁止为了变绿修改断言、删除用例、加 `@Disabled`、放宽超时；测试只加严不放宽。
- 覆盖率不是目标，但核心业务分支必须有测试；新增分支不得无测试。

## AI 常见劣化模式

| 劣化写法 | 正确做法 |
|---|---|
| service 直接注入 `RedisTemplate`/`KafkaTemplate`/Mapper | 通过 infra 封装访问 |
| 把类型挪到 common 让两层都能用 | 类型归真正拥有它的层，边界做转换 |
| 单实现也建 `IXxxService` + `XxxFactory` + `AbstractXxx` | 直接用具体类，出现第二实现再抽象 |
| 加 `if (useNew) ... else ...` 兼容层 | 删旧路径，保持单一路径 |
| 改断言、加 `@Disabled`、放宽超时让测试变绿 | 修代码；测试只加严 |
| 只跑单个测试就宣称全部通过 | 跑完整检查并贴输出 |
| `catch (Exception e) { log.error(...) }` 后继续 | 转换抛出，或明确重试/死信/熔断 |
| `new BigDecimal(0.1)`、`equals` 比金额、`divide` 不指定精度 | 字符串构造、`compareTo`、显式 `RoundingMode` |
| WS 回调里查库、发 HTTP、`Thread.sleep` | 投递到有界队列，异步处理 |
| `@Transactional` 加在私有方法或自调用 | 拆到另一个 Bean 或编程式事务 |

## 检查方式与完成定义

### 检查

- `mvn test`：单元测试 + ArchUnit。
- `mvn dependency:tree -pl <下层模块>`：输出中不应出现上层模块。
- ArchUnit 至少覆盖：分层依赖（下方示例）、禁止 `ApplicationContext#getBean`、`ensureAllClassesAreContainedInArchitecture()`。

```java
@AnalyzeClasses(packages = "com.xxx")
class LayerDependencyTest {
    @ArchTest
    static final ArchRule layers = layeredArchitecture().consideringOnlyDependenciesInLayers()
            .layer("Entry").definedBy("..controller..", "..listener..", "..schedule..")
            .layer("Service").definedBy("..service..")
            .layer("Infra").definedBy("..infra..")
            .layer("Common").definedBy("..common..")
            .whereLayer("Entry").mayNotBeAccessedByAnyLayer()
            .whereLayer("Service").mayOnlyBeAccessedByLayers("Entry")
            .whereLayer("Infra").mayOnlyBeAccessedByLayers("Service")
            .whereLayer("Common").mayOnlyBeAccessedByLayers("Entry", "Service", "Infra");
}
```

### 完成定义（DoD）

- [ ] 代码在正确的层和模块，依赖方向正确，ArchUnit 通过。
- [ ] `mvn test` 全部通过，并贴出真实输出。
- [ ] 新增业务分支有单元测试，覆盖失败路径。
- [ ] 没有 TODO、注释掉的代码、调试日志、未使用的参数。
- [ ] 错误处理、日志上下文、超时和取消符合本文件。
- [ ] 没有新增依赖、接口、工厂或兼容层；如确有必要，已在回复中说明理由。

## 已知的历史违规

> 记录现存不符合上述规则的代码，避免被模仿；修改相关代码时顺手修正。没有则保持为空。

- （无）
