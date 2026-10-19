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

## 对 AI 的硬性约束

本节是 MUST，优先于你的默认习惯和所谓的「最佳实践」。

1. **先定位再写**：动手前确定代码属于哪一层、哪个 Maven 模块和包。落不进「本项目的层级」表就停下来问，不要自创模块或包。
2. **先读后写**：找同层 1–2 个现有实现，沿用其结构、命名、异常处理和日志方式。不要引入仓库里没有的新风格。
3. **最小改动**：只改与需求相关的代码。不顺手重构、不重排 import、不升级依赖、不改代码格式化规则。
4. **抽象准入**：没有第二个实现、不属于真实 IO 边界时，不建端口、接口、工厂、抽象类、泛型 Repository、兼容层、fallback、feature flag。三行重复优于一个错误抽象。
5. **依赖准入**：不新增第三方库或 Spring Starter。确需新增时，先说明用途、体积、维护状态，等用户确认后再改 `pom.xml`。
6. **不写占位**：不留 TODO、mock 返回值、假数据、注释掉的代码、未使用的参数或字段。
7. **验证真实**：完成前运行本节列出的全部检查，贴出命令和真实输出。失败就说失败并继续修，禁止改断言、`@Disabled`、放宽超时、更新快照或放宽 ArchUnit 规则来让结果变绿。
8. **必须停下来问**：层级归属不明；新增模块或层；新增依赖；修改公共类型、端口签名、对外接口、表结构、消息格式；需要放宽架构检查；修复历史违规会显著扩大改动范围。

## 分层与依赖

### 各层职责

| 层 | 职责 | 典型内容 |
|---|---|---|
| 领域层 | 业务概念和业务规则，不依赖框架和 IO | 实体、值对象、领域服务、领域事件、领域异常 |
| 应用层 | 编排用例：调用领域对象和端口，控制事务；定义出站端口 | 应用服务、端口接口、命令/结果对象、应用异常 |
| 外层·入站适配器 | 接收外部触发，转换为内层类型后调用应用层 | Controller、WS 推送接收、MQ Consumer、定时任务 |
| 外层·出站适配器 | 实现应用层端口，访问外部系统 | 交易所下单客户端、Mapper/Repository、Redis、Kafka 发布 |
| 外层·组装 | Bean 装配、配置绑定、生命周期，只做接线 | `@SpringBootApplication`、`@Configuration` |

不设单独的 common 层：业务通用类型放领域层；与业务无关的纯工具放 `domain.shared`，不得依赖框架。

### 本项目的层级

> 下表为示例，按实际项目修改。新增模块、拆分适配器时，同步更新本表和 ArchUnit 配置。

| 层 | Maven 模块 / 包 | 可以依赖 | 外部依赖限制 |
|---|---|---|---|
| 领域层 | `xxx-domain` / `com.xxx.domain.model`、`domain.service` | 无 | 只允许 JDK、Lombok 等基础库 |
| 应用层 | `xxx-application` / `com.xxx.application.service`、`application.port` | 领域层 | 仅可用 `@Service`、`@Transactional`，其余框架依赖禁止 |
| 入站适配器 | `xxx-adapter` / `com.xxx.adapter.in.web`、`in.ws`、`in.job` | 应用层、领域层 | 不依赖出站适配器 |
| 出站适配器 | `xxx-adapter` / `com.xxx.adapter.out.exchange`、`out.persistence`、`out.mq` | 应用层、领域层 | 不依赖入站适配器 |
| 组装 | `xxx-boot` / `com.xxx.boot` | 所有层 | 无 |

### 依赖规则

- **依赖只能由外向内**：领域层不依赖任何层；应用层只依赖领域层；适配器依赖应用层和领域层；组装依赖所有层。
- **适配器之间互不依赖**：入站适配器只调用应用层，不能直接调用出站适配器或查询类；出站适配器之间也不互相调用。只读查询走应用层的查询服务。
- **领域层不依赖任何框架**：不出现 Spring、MyBatis、JPA、Jackson、Kafka、Redis 的注解和类型。
- **应用层允许 `@Service` 和 `@Transactional`**：用构造器注入，字段 `final`；禁止字段注入、`ApplicationContext.getBean`、静态 holder。
- 依赖图不能有环，包括 Spring Bean 之间的循环依赖。
- 所有跨边界调用必须有明确的超时；禁止无超时地调用外部系统。

### 新代码放哪里

- 新的业务概念、业务规则 → 领域层。
- 新的业务流程 → 应用服务；需要外部能力时在 `port` 新增接口。
- 新的外部系统接入 → 出站适配器，实现已有或新增端口。
- 新的触发方式 → 入站适配器，只做转换和调用。
- 新增模块或适配器：先更新层级表和 ArchUnit 配置，再写代码。

## 端口与跨边界类型

- **出站端口定义在应用层**的 `port` 包，由出站适配器实现。只有领域服务本身需要调用的端口才放领域层。
- 入站端口默认不定义，入站适配器直接调用应用服务；只有需要多种实现时才定义。
- 端口按业务需要设计（`submit(HedgeOrder)`），不按外部系统 API 设计（禁止 `submit(BinanceOrderRequest)`）。
- **跨边界的类型归内层所有，转换由外层负责**：入站适配器把 HTTP/WS/MQ 报文转换为应用命令或领域类型；出站适配器把领域类型转换为交易所请求、PO、Kafka 消息，返回时再转回内层类型。
- 交易所报文 DTO、数据库 PO、Kafka 消息体、HTTP 请求/响应对象只能出现在外层。组装在 boot 用 `@Bean` 或构造器注入完成，不要在应用层写 `new` 装配链。

```java
// domain：业务规则，不依赖框架
public record Symbol(String code, BigDecimal tickSize, int qtyScale) {
    public BigDecimal alignPrice(BigDecimal price, Side side) { /* 按 tickSize 取整 */ }
}

public class HedgeOrder {
    public static HedgeOrder from(Trade trade, Symbol symbol) { /* 计算方向、价格、数量 */ }
    public HedgeOrderPlaced placed(OrderAck ack) { /* 生成领域事件 */ }
}

// application.port：出站端口，签名只用内层类型
public interface HedgeOrderGateway { OrderAck submit(HedgeOrder order); }

// application.service：只编排，不写业务计算
public class HedgeAppService {
    private final SymbolRepository symbols;
    private final HedgeOrderGateway gateway;
    private final HedgeEventPublisher publisher;

    public HedgeAppService(SymbolRepository symbols, HedgeOrderGateway gateway, HedgeEventPublisher publisher) { /* 赋值 */ }

    public void onTrade(Trade trade) {
        HedgeOrder order = HedgeOrder.from(trade, symbols.get(trade.symbol()));
        publisher.publish(order.placed(gateway.submit(order)));
    }
}

// adapter.out：实现端口，在这里转换外部格式
@Component
@RequiredArgsConstructor
public class BinanceHedgeOrderGateway implements HedgeOrderGateway {
    private final BinanceClient client;
    public OrderAck submit(HedgeOrder order) { return toAck(client.place(toRequest(order))); }
}

// adapter.in：转换后调用应用层
@Component
@RequiredArgsConstructor
public class TradeWsListener {
    private final HedgeAppService hedgeAppService;
    void onMessage(String raw) { hedgeAppService.onTrade(toTrade(parse(raw))); }
}
```

### 层级内部规则

- 领域层：规则优先写在实体和值对象中；跨实体规则写领域服务。不直接取当前时间、随机数、全局 ID，`now` 通过参数传入；金额用 `BigDecimal`；必须有纯单元测试。
- 应用层：只做加载、调用领域对象、调用端口、发布事件、控制事务；不写价格计算和条件分支业务规则。幂等、去重、并发控制属于应用层，具体实现通过端口交给外层。
- 外层：入站只转换和调用；出站只处理协议、签名、序列化、连接、重连、持久化，不做业务判断。回调运行在 IO/WS/Netty 线程时，耗时处理必须投递到业务线程池。

## 数据、时间与一致性

- 价格、数量、金额统一使用 `BigDecimal`，禁止 `double`/`float`/`Double`，禁止在金额路径调用 `doubleValue()`/`floatValue()`；上游 double 先转字符串。
- 禁止 `new BigDecimal(0.1)`；用 `new BigDecimal("0.1")`、`BigDecimal.valueOf(long)` 或 `BigDecimal.valueOf(String)`。金额比较用 `compareTo`，不用 `equals`。
- 除法必须指定 scale 和 `RoundingMode`；任何隐式舍入都视为错误。
- 下游写入使用确定性幂等键并加唯一约束；消息流必须处理重复、乱序、gap、断线重连：gap 时作废缓存并重拉快照，重连后重订阅；重复消息不能产生重复副作用。
- 领域层不调用 `Instant.now()`/`System.currentTimeMillis()`：`now` 作为参数传入；应用层需要时可注入 `java.time.Clock`，但时钟端口属于应用层。延迟和超时用 `System.nanoTime()`。

## 并发与性能

- 禁止 `new Thread()`、匿名线程和默认 `ForkJoinPool.commonPool()`。使用有名字、有界队列、明确拒绝策略的 `ThreadPoolExecutor`。
- `@Async` 必须指定自定义线程池；`CompletableFuture` 必须有超时，禁止无超时 `.get()`/`.join()`。
- 禁止在 Netty/WS 回调线程、`@EventListener` 同步链路中做 DB、HTTP、文件 IO 或 `Thread.sleep`。
- 有界队列必须明确满了的策略；行情类可丢弃并置 gap 标记，订单/回报/成交类不能丢，满则拒绝受理或熔断。
- 共享可变状态用 `ConcurrentHashMap`、`Atomic*`、`ReentrantLock` 或不可变对象；禁止 `synchronized(this)`。
- 锁内不做 IO、不调用回调；多锁按固定顺序获取。禁止共享 `SimpleDateFormat`，用 `DateTimeFormatter`。
- 热路径禁止逐笔日志、字符串拼接、JSON 序列化、反射、无界重试和不必要的对象分配。
- 捕获 `InterruptedException` 后必须 `Thread.currentThread().interrupt()`。

## 错误处理与事务

- 领域异常定义在领域层，应用异常和端口异常定义在应用层；出站适配器把第三方异常转换为端口约定的异常，入站适配器统一把内层异常转换为响应（如 `@RestControllerAdvice`）。
- 禁止空 `catch`、`catch (Exception e) { log.error(...) }` 后继续执行、`printStackTrace()`；失败必须明确重试、死信、熔断、返回错误或停机中的一种。
- `@Transactional` 只加在应用层的 public、非 final 方法上；同类自调用不经过代理，事务不生效，必须拆到另一个 Bean 或改用编程式事务。
- 受检异常默认不回滚，需要回滚时显式 `rollbackFor = Exception.class`；传播行为显式声明。
- 事务内不做远程调用（交易所、HTTP、MQ 同步发送）；数据库与消息的一致性用 outbox 或 `afterCommit`，不做双写。
- 适配器不开事务；入站适配器不管理事务。

## 测试与验证

- 领域计算、状态流转必须有纯 JUnit 5 + AssertJ 单元测试，不启动 Spring，覆盖正常、边界、非法输入。
- 应用层测试用 Mockito mock 端口或使用内存 fake；不连接真实数据库、Redis、交易所。
- 适配器用集成测试或契约测试验证协议、序列化、重连；不要 mock 掉被测的转换逻辑。
- 测试必须确定性：注入固定 `Clock`；禁止 `Thread.sleep`，用 Awaitility 或有界 `CompletableFuture` 等待。
- 禁止为了变绿修改断言、删除用例、加 `@Disabled`、放宽超时；测试只加严不放宽。
- 覆盖率不是目标，但核心业务分支必须有测试；新增分支不得无测试。

## AI 常见劣化模式

| 劣化写法 | 正确做法 |
|---|---|
| domain import Spring/JPA/Jackson | 领域层保持纯净 |
| 入站适配器直接调用出站适配器或查询类 | 走应用服务 + 端口 |
| 把类型挪到 `domain.shared` 让各层都能用 | 类型归真正拥有它的层，边界做转换 |
| 单实现也建端口 + 工厂 + 抽象类 | 直接用具体类，出现第二实现再抽象 |
| 加 `if (useNew) ... else ...` 兼容层 | 删旧路径，保持单一路径 |
| 改断言、加 `@Disabled`、放宽超时让测试变绿 | 修代码；测试只加严 |
| 只跑单个测试就宣称全部通过 | 跑完整检查并贴输出 |
| `catch (Exception e) { log.error(...) }` 后继续 | 转换抛出，或明确重试/死信/熔断 |
| `new BigDecimal(0.1)`、`equals` 比金额、`divide` 不指定精度 | 字符串构造、`compareTo`、显式 `RoundingMode` |
| `@Transactional` 加在私有方法或自调用 | 拆到另一个 Bean 或编程式事务 |

## 检查方式与完成定义

### 检查

- `mvn test`：单元测试 + ArchUnit。
- `mvn dependency:tree -pl xxx-domain`：不应出现 Spring、数据库、消息依赖。
- ArchUnit 至少覆盖洋葱分层、领域层纯净、应用层纯净：

```java
@AnalyzeClasses(packages = "com.xxx")
class OnionArchitectureTest {
    @ArchTest
    static final ArchRule onion = onionArchitecture()
            .domainModels("..domain.model..").domainServices("..domain.service..")
            .applicationServices("..application..")
            .adapter("web", "..adapter.in.web..").adapter("ws", "..adapter.in.ws..")
            .adapter("exchange", "..adapter.out.exchange..").adapter("persistence", "..adapter.out.persistence..");

    @ArchTest
    static final ArchRule domainIsPure = classes().that().resideInAPackage("..domain..")
            .should().onlyDependOnClassesThat().resideInAnyPackage("..domain..", "java..", "lombok..");
}
```

### 完成定义（DoD）

- [ ] 代码在正确的层和模块，依赖方向正确，ArchUnit 通过。
- [ ] `mvn test` 全部通过，并贴出真实输出。
- [ ] 新增业务分支有单元测试，覆盖失败路径。
- [ ] 没有 TODO、注释掉的代码、调试日志、未使用的参数。
- [ ] 错误处理、日志上下文、超时和取消符合本文件。
- [ ] 没有新增依赖、端口、工厂或兼容层；如确有必要，已在回复中说明理由。

## 已知的历史违规

> 记录现存不符合上述规则的代码，避免被模仿；修改相关代码时顺手修正。没有则保持为空。

- （无）
