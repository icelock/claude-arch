# CLAUDE.md

## 架构声明

本项目采用**洋葱架构**：从内到外分为**领域层（domain）→ 应用层（application）→ 外层（adapter + boot）**，依赖只能由外向内。

```
外层：adapter.in（入站）、adapter.out（出站）、boot（组装）
  └── 应用层：用例编排、事务边界、出站端口
        └── 领域层：实体、值对象、领域服务、领域事件
依赖只能由外向内
```

六边形架构和整洁架构与本项目是同一条依赖规则的不同命名：六边形强调入站/出站端口对称，整洁架构强调用例输入/输出端口。**本项目统一按洋葱落法，不要引入第四种命名、分层或目录结构。**

## 本项目事实

> 拷贝模板后先填写本节。AI 依据这里的版本选择语法和 API；仍是 `<填写>` 的项先问用户，不要猜。

- JDK：`<填写：17 / 21>`（决定能否用 record、switch 模式匹配、虚拟线程）
- Spring Boot：`<填写：2.7 / 3.x>`（3.x 用 `jakarta.*`，2.x 用 `javax.*`，不能混用）
- 持久层：`<填写：MyBatis-Plus / MyBatis / JPA>`；日志：`<填写：SLF4J + Logback>`；表结构迁移：`<填写：Flyway / Liquibase / 无>`
- 构建与测试：`<填写，如 mvn -q test>`
- 模块：`<填写，如 xxx-domain、xxx-application、xxx-adapter、xxx-boot>`

## 规则优先级与工作方式

规则冲突时按以下顺序取舍：**用户当次明确指令 > 正确性与安全 > 本文件的架构规则 > 仓库现有写法 > 通用最佳实践**。例外：安全类规则（密钥、注入、权限校验）即使用户要求放宽，也要先说明风险并得到确认。现有代码违反本文件时不要模仿，新代码按本文件写。

- 跨层改动或改动超过 3 个文件时，先用几行说明计划：落在哪层、改哪些文件、是否新增端口、是否涉及「必须停下来问」的事项，再动手。
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
4. **抽象准入**：真实 IO 边界的出站端口是架构要求，必须建；一个外部能力对应一个端口，不按方法拆。除此之外，没有第二个实现时，不建接口、工厂、抽象类、泛型 Repository、兼容层、fallback、feature flag；纯计算不建端口。三行重复优于一个错误抽象。
5. **依赖准入**：不新增第三方库或 Spring Starter。确需新增时，先说明用途、体积、维护状态，等用户确认后再改 `pom.xml`。
6. **不写占位**：不留 TODO、mock 返回值、假数据、注释掉的代码、未使用的参数或字段。
7. **验证真实**：完成前运行「检查」一节列出的全部检查，贴出命令和真实输出。失败就说失败并继续修，禁止改断言、`@Disabled`、放宽超时、更新快照或放宽 ArchUnit 规则来让结果变绿。
8. **必须停下来问**：层级归属不明；新增模块或层；新增依赖；修改公共类型、端口签名、对外接口、表结构、消息格式；需要放宽架构检查；修复历史违规会超出本次需求的改动范围。

## 分层与依赖

### 各层职责

| 层 | 职责 | 典型内容 |
|---|---|---|
| 领域层 | 业务概念和业务规则，不依赖框架和 IO | 实体、值对象、领域服务、领域事件、领域异常 |
| 应用层 | 编排用例：加载、调用领域对象、调用端口、发布事件、控制事务；定义出站端口 | 应用服务、端口接口、命令/结果对象、应用异常 |
| 外层·入站适配器 | 接收外部触发，转换为内层类型后调用应用层 | Controller、WS 推送接收、MQ Consumer、定时任务 |
| 外层·出站适配器 | 实现应用层端口，访问外部系统 | 第三方服务客户端、Mapper/Repository、Redis、Kafka 发布 |
| 外层·组装 | Bean 装配、配置绑定、生命周期，只做接线 | `@SpringBootApplication`、`@Configuration` |

不设单独的 common 层：业务通用类型放领域层；与业务无关的纯工具放 `domain.shared`，不得依赖框架。

### 本项目的层级

> 下表为示例，按实际项目修改。新增模块、拆分适配器时，同步更新本表和 ArchUnit 配置。

| 层 | Maven 模块 / 包 | 可以依赖 | 外部依赖限制 |
|---|---|---|---|
| 领域层 | `xxx-domain` / `com.xxx.domain.model`、`domain.service`、`domain.shared` | 无 | 只允许 JDK 非 IO 包和 Lombok |
| 应用层 | `xxx-application` / `com.xxx.application.service`、`application.query`、`application.port` | 领域层 | 仅可用 `@Service`、`@Transactional`，其余框架依赖禁止 |
| 入站适配器 | `xxx-adapter` / `com.xxx.adapter.in.web`、`in.ws`、`in.mq`、`in.job` | 应用层、领域层 | 不依赖出站适配器 |
| 出站适配器 | `xxx-adapter` / `com.xxx.adapter.out.payment`、`out.persistence`、`out.mq` | 应用层、领域层 | 不依赖入站适配器 |
| 组装 | `xxx-boot` / `com.xxx.boot` | 所有层 | 无 |

### 依赖规则

- **依赖只能由外向内**：领域层不依赖任何层；应用层只依赖领域层；适配器依赖应用层和领域层；组装依赖所有层。
- **适配器之间互不依赖**：入站适配器只调用应用层，不能直接调用出站适配器或查询类；出站适配器之间也不互相调用。只读查询走应用层的查询服务。
- **领域层不依赖任何框架**：不出现 Spring、MyBatis、JPA、Jackson、Kafka、Redis 的注解和类型。
- **应用层只允许 `@Service` 和 `@Transactional`**：用构造器注入，字段 `final`；禁止字段注入、`ApplicationContext.getBean`、静态 holder。
- 依赖图不能有环，Spring Bean 之间也不能循环依赖。

### 新代码放哪里

- 新的业务概念、业务规则 → 领域层：规则优先写在实体和值对象中，跨实体的规则写领域服务。
- 新的业务流程 → 应用服务；需要外部能力时在 `port` 新增接口。
- 新的外部系统接入 → 出站适配器，实现已有或新增的端口。
- 新的触发方式 → 入站适配器，只做转换和调用。
- 新增模块或适配器：先更新层级表和 ArchUnit 配置，再写代码。

## 端口与跨边界类型

- **出站端口定义在应用层**的 `port` 包，由出站适配器实现。只有领域服务本身需要调用的端口才放领域层。
- 入站端口默认不定义，入站适配器直接调用应用服务；只有需要多种实现时才定义。
- **查询侧**：只读查询放 `application.query` 的查询服务，查询端口放 `application.port`，由持久化适配器实现；可以直接返回只读视图对象（record），不必经过领域模型。入站适配器不得直接调用 Mapper。
- 端口按业务需要设计（`charge(Order)`），不按外部系统 API 设计（禁止 `charge(StripeChargeRequest)`）。
- **跨边界的类型归内层所有，转换由外层负责**：入站适配器把 HTTP/MQ 报文转换为应用命令或领域类型；出站适配器把领域类型转换为第三方请求、PO、Kafka 消息，返回时再转回内层类型。
- 第三方报文 DTO、数据库 PO、Kafka 消息体、HTTP 请求/响应对象只能出现在外层。
- 装配在 boot 用 `@Bean` 或构造器注入完成，不要在应用层写 `new` 装配链。

```java
// domain：业务规则，不依赖框架
public record Money(BigDecimal amount, Currency currency) { /* 加减、比较，舍入规则 */ }

public class Order {
    public static Order place(Cart cart, Instant now) { /* 计算金额、校验约束 */ }
    public void startPayment(Instant now) { /* 只有「待支付」可以发起，否则抛领域异常 */ }
    public OrderPaid markPaid(PaymentReceipt receipt, Instant now) { /* 只有「支付中」可以确认；生成领域事件 */ }
}

// application.port：出站端口，签名只用内层类型
public interface PaymentGateway {
    /** 实现方必须以订单号作为幂等键，重复调用不会重复扣款。 */
    PaymentReceipt charge(Order order);
}

// application.service：编排支付流程，只编排，不写业务计算；远程调用在事务外
@Service
@RequiredArgsConstructor
public class PayOrderAppService {
    private final OrderPaymentSteps steps;  // 事务步骤放在另一个 Bean，避免同类自调用导致事务失效
    private final PaymentGateway gateway;

    public void pay(OrderId id) {
        Order order = steps.startPayment(id);            // 事务 1：待支付 → 支付中，提交后才调用外部
        PaymentReceipt receipt = gateway.charge(order);  // 事务外；超时可安全重试
        steps.markPaid(id, receipt);                     // 事务 2：支付中 → 已支付，领域事件写入 outbox
    }   // charge 失败或进程崩溃时订单停在「支付中」，由对账任务按订单号查询支付结果补偿
}

@Service
@RequiredArgsConstructor
public class OrderPaymentSteps {
    private final OrderRepository orders;
    private final OrderEventPublisher publisher;     // outbox 实现：与订单写入同库同事务
    private final Clock clock;

    @Transactional
    public Order startPayment(OrderId id) {
        Order order = orders.get(id);
        order.startPayment(clock.instant());
        orders.save(order);
        return order;
    }

    @Transactional
    public void markPaid(OrderId id, PaymentReceipt receipt) {
        Order order = orders.get(id);
        publisher.publish(order.markPaid(receipt, clock.instant()));
        orders.save(order);
    }
}

// adapter.out：实现端口，在这里转换外部格式
@Component
@RequiredArgsConstructor
public class StripePaymentGateway implements PaymentGateway {
    private final StripeClient client;
    public PaymentReceipt charge(Order order) { return toReceipt(client.charge(toRequest(order))); }
}

// adapter.in：转换后调用应用层
@RestController
@RequiredArgsConstructor
public class OrderController {
    private final PayOrderAppService payOrderAppService;
    @PostMapping("/orders/{id}/pay") void pay(@PathVariable long id) { payOrderAppService.pay(new OrderId(id)); }
}
```

### 层级内部规则

- 领域层：不直接取当前时间、随机数、全局 ID，这些都通过参数传入；必须有纯单元测试。
- 聚合：一个事务只修改一个聚合；聚合之间按 ID 引用，不持有对方对象；跨聚合的一致性用领域事件实现最终一致。Repository 按聚合定义，一个聚合一个。
- 应用层：只做加载、调用领域对象、调用端口、发布事件、控制事务；不写金额计算，也不写承载业务规则的条件分支。幂等、去重、并发控制属于应用层，具体实现通过端口交给外层。
- 外层：入站只转换和调用；出站只处理协议、签名、序列化、连接、重连、持久化，不做业务判断。回调运行在 IO 线程（WS、Netty、MQ 客户端）上时，耗时处理必须投递到业务线程池。

## 数据与一致性

- 金额、价格、数量统一使用 `BigDecimal`，禁止 `double`/`float`/`Double`，禁止在金额路径调用 `doubleValue()`/`floatValue()`；上游给的是 double 时先转字符串。
- 禁止 `new BigDecimal(0.1)`；用 `new BigDecimal("0.1")`、`BigDecimal.valueOf(long)` 或 `BigDecimal.valueOf(String)`。金额比较用 `compareTo`，不用 `equals`（`equals` 会比较 scale，`1.0` 不等于 `1.00`）。
- 除法必须指定 scale 和 `RoundingMode`；任何隐式舍入都视为错误。
- 下游写入使用确定性幂等键，数据库加唯一约束；重复消息不能产生重复副作用。
- 领域层不调用 `Instant.now()`/`LocalDateTime.now()`/`System.currentTimeMillis()`，`now` 作为参数传入；应用层注入 `java.time.Clock` 取时间。测量延迟和超时用 `System.nanoTime()`。
- 契约演进：表结构变更走迁移脚本并保持向后兼容；消息和接口只加可选字段，不改语义、不删字段。这类改动仍需先问（硬性约束第 8 条）。

## 数据访问

- 禁止在循环里逐条查库或调用远程接口（N+1），改为按 ID 集合批量查询；需要时在端口上提供批量方法。
- 列表查询必须分页并设置上限；不允许无条件的全表查询。
- 批量写入用批量 API（如 `saveBatch`、批量 `INSERT`），不要循环单条写。
- 只查需要的列，不写 `select *`；大字段按需单独查询。
- 事务尽量小：只包住必须原子完成的写操作，查询、计算、远程调用放在事务外。

## 并发

- 禁止 `new Thread()`、匿名线程和默认 `ForkJoinPool.commonPool()`。使用有名字、有界队列、明确拒绝策略的 `ThreadPoolExecutor`。
- `@Async` 必须指定自定义线程池；`CompletableFuture` 必须有超时，禁止无超时的 `.get()`/`.join()`。
- 有界队列必须明确队列满时的策略（阻塞、丢弃、拒绝受理、熔断），写在代码注释中；不能丢的数据，队列满时只能拒绝受理或熔断，不能丢弃。
- 禁止在 IO 回调线程、`@EventListener` 同步链路中做 DB、HTTP、文件 IO 或 `Thread.sleep`。
- 共享可变状态用 `ConcurrentHashMap`、`Atomic*`、`ReentrantLock` 或不可变对象；禁止 `synchronized(this)`。
- 锁内不做 IO、不调用回调、不获取其他锁；需要多把锁时按固定顺序获取。
- 禁止共享 `SimpleDateFormat`，用不可变的 `DateTimeFormatter`。
- 捕获 `InterruptedException` 后必须 `Thread.currentThread().interrupt()`。

## 错误处理与事务

- 领域异常定义在领域层，应用异常和端口异常定义在应用层；出站适配器把第三方异常转换为端口约定的异常，入站适配器统一把内层异常转换为响应（如 `@RestControllerAdvice`）。
- 禁止空 `catch`、`catch (Exception e) { log.error(...) }` 后继续执行、`printStackTrace()`；失败必须明确采取以下一种处理：重试（有上限和退避）、死信、熔断、返回错误、停机。
- `@Transactional` 只加在应用层的 public、非 final 方法上。同类自调用不经过 Spring 代理，事务不生效，必须拆到另一个 Bean 或改用编程式事务。
- 受检异常默认不回滚，需要回滚时显式写 `rollbackFor = Exception.class`；传播行为显式声明。
- 事务内不做远程调用（第三方 HTTP、MQ 同步发送）；数据库与消息的一致性用 outbox 或 `afterCommit`，不做双写。
- 适配器和组装层不开事务。

## 日志、安全与配置

- 日志用 SLF4J 占位符：`log.info("order {} paid", orderId)`，不拼接字符串；异常作为最后一个参数传入（`log.error("charge failed, order={}", id, e)`），保留堆栈。
- 领域层不打日志，用返回值或领域异常表达结果；日志放在应用层和适配器。
- 链路标识通过 MDC 传递（如 `traceId`）；切换线程池时要把 MDC 一起传过去。
- 日志级别：error 只用于需要人处理的问题，可预期的业务失败用 warn 或 info；循环内和高频路径的日志必须限频。
- 队列积压、丢弃、重试、重连次数必须有指标或限频日志。
- 密钥、token、密码不硬编码，不写进日志和异常信息；从配置中心或环境变量读取。
- MyBatis 只用 `#{}`，禁止 `${}` 拼接用户输入；手写 SQL 一律参数化。
- 格式校验在入站适配器完成（`@Validated` + Bean Validation）；业务不变量由领域对象在构造和状态流转时校验。
- 配置用 `@ConfigurationProperties` + `@Validated` 绑定，只在外层使用；内层需要的参数通过构造器传入，不读配置。URL、超时、线程数、队列长度不硬编码。
- 所有外部调用（HTTP、RPC、DB、Redis、MQ）必须设置明确的连接超时和读超时。

## Java 编码细则

- Lombok 只用 `@RequiredArgsConstructor`、`@Getter`、`@Builder`、`@Slf4j`；Entity/PO 上禁止 `@Data`（它生成的 `equals`/`hashCode` 包含可变字段），禁止 `@SneakyThrows`。
- 值对象优先用 record，并在紧凑构造器中校验不变量；实体不暴露 setter，状态变化通过表达业务意图的方法完成。
- 方法不返回 null 集合，返回空集合；`Optional` 只用作返回值，不用作字段和参数。
- 注释写「为什么」，不写「做了什么」；公开方法的非显然前置条件、线程安全性写在 Javadoc 里。

## 测试与验证

- 领域计算、状态流转必须有纯 JUnit 5 + AssertJ 单元测试，不启动 Spring，覆盖正常、边界、非法输入。
- 应用层测试用 Mockito mock 端口或使用内存 fake；不连接真实数据库、Redis、第三方服务。
- 适配器用集成测试或契约测试验证协议、序列化、重连；不要 mock 掉被测的转换逻辑。
- 测试必须确定性：注入固定的 `Clock`；禁止用 `Thread.sleep` 等待，用 Awaitility 或带超时的 `CompletableFuture`。
- 禁止为了变绿修改断言、删除用例、加 `@Disabled`、放宽超时；测试只加严不放宽。
- 覆盖率不是目标，但核心业务分支必须有测试；新增分支不得没有测试。

## AI 常见劣化模式

| 劣化写法 | 正确做法 |
|---|---|
| domain import Spring/JPA/Jackson | 领域层保持纯净 |
| 入站适配器直接调用出站适配器或 Mapper | 走应用服务 + 端口；查询走 `application.query` |
| 端口签名里出现第三方请求类型 | 端口只用内层类型，转换放在适配器 |
| 单实现也建工厂、抽象类，为纯计算建端口，或加 `if (useNew)` 兼容层 | 只为 IO 边界建端口，其余用具体类，保持单一路径 |
| 应用服务里写金额计算和业务 if-else | 规则下沉到实体、值对象或领域服务 |
| 实体用 setter 改状态、`@Data` | 用表达业务意图的方法，`@Getter` |
| 一个事务里修改多个聚合 | 按 ID 引用，跨聚合用领域事件 |
| 远程调用放在事务内，或先调外部、后落库且没有幂等键 | 先落库「处理中」，以业务 ID 为幂等键调用外部，再落库结果 |
| `catch (Exception e) { log.error(...) }` 后继续 | 转换后抛出，或明确重试、死信、熔断 |
| 改断言、加 `@Disabled`，或只跑单个测试就宣称通过 | 修代码，跑完整检查并贴输出 |

## 检查方式与完成定义

### 检查

- `mvn test`：单元测试 + ArchUnit。
- `mvn dependency:tree -pl xxx-domain`：不应出现 Spring、数据库、消息依赖。
- ArchUnit 至少覆盖洋葱分层、领域层纯净、应用层纯净；下面的示例拷到项目后按实际包名调整：

```java
@AnalyzeClasses(packages = "com.xxx", importOptions = ImportOption.DoNotIncludeTests.class)
class OnionArchitectureTest {

    // 依赖由外向内、适配器之间互不依赖；ensureAll 让不属于任何层的新包无法绕过检查
    @ArchTest
    static final ArchRule onion = onionArchitecture()
            .domainModels("..domain.model..", "..domain.shared..").domainServices("..domain.service..")
            .applicationServices("..application..")
            .adapter("web", "..adapter.in.web..").adapter("ws", "..adapter.in.ws..").adapter("mq-in", "..adapter.in.mq..")
            .adapter("job", "..adapter.in.job..")
            .adapter("payment", "..adapter.out.payment..").adapter("persistence", "..adapter.out.persistence..")
            .adapter("mq-out", "..adapter.out.mq..")
            .ensureAllClassesAreContainedInArchitectureIgnoring("..boot..");

    @ArchTest
    static final ArchRule domainIsPure = classes().that().resideInAPackage("..domain..")
            .should().onlyDependOnClassesThat().resideInAnyPackage("..domain..", "java..", "lombok..");

    // java.. 白名单会放进 IO 包，单独禁止（应用层同理）
    @ArchTest
    static final ArchRule domainHasNoIo = noClasses().that().resideInAPackage("..domain..")
            .should().dependOnClassesThat().resideInAnyPackage("java.sql..", "java.net..", "java.nio.file..");

    @ArchTest
    static final ArchRule domainDoesNotReadClock = noClasses().that().resideInAPackage("..domain..")
            .should().callMethod(System.class, "currentTimeMillis")
            .orShould().callMethod(Instant.class, "now")
            .orShould().callMethod(LocalDateTime.class, "now");

    @ArchTest
    static final ArchRule applicationIsPure = classes().that().resideInAPackage("..application..")
            .should().onlyDependOnClassesThat().resideInAnyPackage("..domain..", "..application..", "java..", "lombok..",
                    "org.springframework.stereotype..", "org.springframework.transaction.annotation..");

    @ArchTest
    static final ArchRule applicationHasNoIo = noClasses().that().resideInAPackage("..application..")
            .should().dependOnClassesThat().resideInAnyPackage("java.sql..", "java.net..", "java.nio.file..");

    @ArchTest
    static final ArchRule transactionalOnlyInApplication = noClasses().that().resideOutsideOfPackage("..application..")
            .should().beAnnotatedWith(Transactional.class)
            .orShould().containAnyMethodsThat(annotatedWith(Transactional.class));

    @ArchTest static final ArchRule noFieldInjection = GeneralCodingRules.NO_CLASSES_SHOULD_USE_FIELD_INJECTION;
    @ArchTest static final ArchRule noStandardStreams = GeneralCodingRules.NO_CLASSES_SHOULD_ACCESS_STANDARD_STREAMS;
    @ArchTest static final ArchRule noJul = GeneralCodingRules.NO_CLASSES_SHOULD_USE_JAVA_UTIL_LOGGING;
    @ArchTest static final ArchRule noOldDateTime = GeneralCodingRules.OLD_DATE_AND_TIME_CLASSES_SHOULD_NOT_BE_USED;
}
```

- 以下规则 ArchUnit 拦不住，靠自查和 review：端口按业务设计而不是按外部 API 设计、应用层不写业务规则、一个事务只改一个聚合、金额构造与舍入、事务内远程调用、锁内 IO、日志中的敏感信息、队列满时的策略。

### 完成定义（DoD）

- [ ] 代码在正确的层和模块，依赖方向正确，ArchUnit 通过。
- [ ] `mvn test` 全部通过，并贴出真实输出。
- [ ] 新增业务分支有单元测试，覆盖失败路径；修 bug 有复现测试。
- [ ] 没有 TODO、注释掉的代码、调试日志、未使用的参数。
- [ ] 错误处理、日志、超时与中断、配置方式符合本文件。
- [ ] 没有新增依赖、工厂或兼容层；新增端口对应真实 IO 边界；如有例外，已在回复中说明理由。
- [ ] 已按「工作方式」的格式汇报。

## 已知的历史违规

> 记录现存不符合上述规则的代码，避免被模仿。只在本次需求本来就要改到的代码里顺带修正；超出这个范围的不动，在汇报中列出。没有则保持为空。

- （无）

## 附录：实时交易系统（可选）

> 非交易类项目删除本节。本节规则补充正文，冲突时以本节为准。

- 交易所报文、签名、限频（rate limit）、错误码只在出站适配器处理；端口按业务设计（`submit(HedgeOrder)`），禁止 `submit(BinanceOrderRequest)`。
- 下单使用确定性的 `clientOrderId` 作为幂等键，超时或网络错误后先按它查询订单状态再决定是否重下，禁止盲目重试。
- 行情、订单推送必须处理重复、乱序、gap、断线重连：出现 gap 时作废本地缓存并重拉快照，不能跳过继续用；重连后必须重新订阅。
- 背压分级：行情类可以丢弃并置 gap 标记；订单、回报、成交类不能丢，队列满时拒绝受理或熔断。
- WS/Netty 回调线程只做解析和投递，不查库、不发 HTTP、不 `Thread.sleep`。
- 热路径禁止逐笔日志、字符串拼接、JSON 序列化、反射、无界重试和不必要的对象分配。
- 价格和数量按交易对的 tickSize/stepSize 取整，取整规则写在领域值对象中（如 `Symbol.alignPrice(price, side)`），舍入方向由买卖方向决定，不依赖默认值。
