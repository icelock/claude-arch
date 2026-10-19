# CLAUDE.md

## 应用架构：洋葱架构（三层）

本项目采用洋葱架构，从内到外分为三层：**领域层（domain）→ 应用层（application）→ 外层（adapter + boot）**。依赖只能由外向内。模块和包名可以按业务调整，但必须遵守下面的依赖规则。新增或修改代码前，先确认代码属于哪一层。

```
┌──────────────────── 外层 ─────────────────────┐
│ adapter.in（入站）  adapter.out（出站）  boot    │
│   ┌────────────── 应用层 ──────────────┐       │
│   │ 用例编排、事务边界、出站端口接口       │       │
│   │   ┌────────── 领域层 ──────────┐   │       │
│   │   │ 实体、值对象、领域服务、领域事件 │   │       │
│   │   └────────────────────────────┘   │       │
│   └────────────────────────────────────┘       │
└────────────────────────────────────────────────┘
                 依赖只能由外向内
```

### 各层职责

| 层 | 职责 | 典型内容 |
|---|---|---|
| 领域层 | 业务概念和业务规则，不依赖任何框架和 IO | 实体、值对象、领域服务、领域事件、领域异常 |
| 应用层 | 编排用例：调用领域对象和端口，控制事务边界；定义出站端口接口 | 应用服务、出站端口（网关、仓储、发布器）、命令/结果对象 |
| 外层·入站适配器 | 接收外部触发，转换为内层类型后调用应用层 | Controller、WS 推送接收、MQ Consumer、定时任务 |
| 外层·出站适配器 | 实现应用层的端口，访问外部系统 | 交易所下单客户端、Mapper/Repository 实现、Redis、Kafka 发布 |
| 外层·组装 | Bean 装配、配置绑定、生命周期，只做接线 | `@SpringBootApplication` 启动类、`@Configuration` 配置类 |

不设单独的 common 层：业务通用类型放在领域层；与业务无关的纯工具类放在领域层的 `shared` 包中，不得依赖框架。

### 本项目的层级

> 新增模块、拆分适配器时，同步更新本表和 ArchUnit 配置。下表为示例，按实际项目修改。

| 层 | Maven 模块 / 包 | 可以依赖 |
|---|---|---|
| 领域层 | `xxx-domain` / `com.xxx.domain.model`、`com.xxx.domain.service` | 无（只允许 JDK、Lombok 等基础库） |
| 应用层 | `xxx-application` / `com.xxx.application.service`、`com.xxx.application.port` | 领域层 |
| 入站适配器 | `xxx-adapter` / `com.xxx.adapter.in.web`、`adapter.in.ws`、`adapter.in.job` | 应用层、领域层 |
| 出站适配器 | `xxx-adapter` / `com.xxx.adapter.out.exchange`、`adapter.out.persistence`、`adapter.out.mq` | 应用层、领域层 |
| 组装 | `xxx-boot` / `com.xxx.boot` | 所有层 |

### 依赖规则

- **依赖只能由外向内**：领域层不依赖任何层；应用层只依赖领域层；适配器依赖应用层和领域层；组装依赖所有层。
- **适配器之间互不依赖**：入站适配器只调用应用层，不能直接调用出站适配器；出站适配器之间也不互相调用。
- **领域层不依赖任何框架**：不出现 Spring、MyBatis、JPA、Jackson、Kafka、Redis 等任何注解和类型。
- **应用层只允许使用 `@Transactional`**：不使用 `@Service`、`@Autowired` 等注解，Bean 在组装层通过 `@Bean` 创建，依赖通过构造器传入。
- 禁止 `ApplicationContext.getBean(...)`、`SpringContextHolder`、静态字段保存 Bean 等绕过依赖规则的写法。
- 依赖图中不能有环，包括 Spring Bean 之间的循环依赖。

### 端口与跨边界类型

- **出站端口定义在应用层**的 `port` 包，由出站适配器实现。只有领域服务本身需要调用的端口才放在领域层。
- 端口按业务需要命名和设计（`HedgeOrderGateway.submit(HedgeOrder)`），不能按外部系统的 API 设计（禁止 `submit(BinanceOrderRequest)`）。
- **跨边界的类型归内层所有，转换由外层负责**：
  - 入站：适配器把 HTTP 请求、WS 报文、MQ 消息转换为应用层命令或领域类型，再调用应用层。
  - 出站：适配器把领域类型转换为交易所请求、PO、Kafka 消息，返回时再转换回内层类型。
- 交易所报文 DTO、数据库 PO、Kafka 消息体、HTTP 请求/响应对象只能出现在外层。
- 入站端口（用例接口）默认不定义，入站适配器直接调用应用服务；只有需要多种实现时才定义。

```java
// ===== domain：业务规则，不依赖框架 =====
public record Symbol(String code, BigDecimal tickSize, int qtyScale) {
    public BigDecimal alignPrice(BigDecimal price, Side side) { /* 按 tickSize 取整 */ }
}

public class HedgeOrder {
    public static HedgeOrder from(Trade trade, Symbol symbol) { /* 计算对冲方向、价格、数量 */ }
    public HedgeOrderPlaced placed(OrderAck ack) { /* 生成领域事件 */ }
}

// ===== application.port：出站端口，签名只用内层类型 =====
public interface HedgeOrderGateway {
    OrderAck submit(HedgeOrder order);
}

public interface HedgeEventPublisher {
    void publish(HedgeOrderPlaced event);
}

// ===== application.service：用例编排，不写业务计算 =====
public class HedgeAppService {
    private final SymbolRepository symbolRepository;
    private final HedgeOrderGateway gateway;
    private final HedgeEventPublisher publisher;

    public HedgeAppService(SymbolRepository symbolRepository, HedgeOrderGateway gateway,
                           HedgeEventPublisher publisher) { /* 赋值 */ }

    public void onTrade(Trade trade) {
        Symbol symbol = symbolRepository.get(trade.symbol());
        HedgeOrder order = HedgeOrder.from(trade, symbol);
        OrderAck ack = gateway.submit(order);
        publisher.publish(order.placed(ack));
    }
}

// ===== adapter.out：实现端口，在这里做外部格式转换 =====
@Component
@RequiredArgsConstructor
public class BinanceHedgeOrderGateway implements HedgeOrderGateway {
    private final BinanceClient client;

    @Override
    public OrderAck submit(HedgeOrder order) {
        BinanceOrderResp resp = client.place(toRequest(order));
        return toAck(resp);
    }
}

// ===== adapter.in：转换后调用应用层 =====
@Component
@RequiredArgsConstructor
public class TradeWsListener {
    private final HedgeAppService hedgeAppService;

    void onMessage(String raw) {
        hedgeAppService.onTrade(toTrade(parse(raw)));
    }
}

// ===== boot：组装 =====
@Configuration
class HedgeWiring {
    @Bean
    HedgeAppService hedgeAppService(SymbolRepository symbols, HedgeOrderGateway gateway,
                                    HedgeEventPublisher publisher) {
        return new HedgeAppService(symbols, gateway, publisher);
    }
}
```

### 领域层规则

- 业务规则优先写在实体和值对象中；跨多个实体的规则写成领域服务。不要让实体只剩 getter/setter，而规则散落在应用服务里。
- 不直接获取当前时间、随机数、全局 ID：通过方法参数传入，或在应用层定义 `Clock`、`IdGenerator` 端口。
- 金额、价格、数量使用 `BigDecimal`，舍入方式显式指定。
- 领域层必须有不依赖 Spring 的单元测试，覆盖核心计算和状态流转。

### 应用层规则

- 只做编排：加载数据、调用领域对象、调用端口、发布事件、控制事务。不写价格计算、条件分支判断等业务规则。
- 幂等、去重、并发控制属于应用层，具体实现（如 Redis 去重）通过端口交给外层。
- 应用服务的参数和返回值使用领域类型或应用层定义的命令/结果对象。

### 外层规则

- 入站适配器只做转换和调用，不写业务逻辑。
- 出站适配器只处理技术问题（协议、签名、序列化、连接、重连、持久化），不做业务判断。
- 回调运行在 IO/WS/Netty 线程上时，耗时处理必须投递到业务线程池或队列，不能阻塞回调线程。
- 有界队列需要明确满了以后的策略（等待、丢弃、告警），并写在代码注释中。
- **只读查询例外**：报表、列表、后台查询等不涉及业务规则的只读场景，可以由入站适配器调用外层的查询类直接返回 DTO，不经过领域层。

### 异常处理

- 领域异常定义在领域层，应用异常定义在应用层。
- 出站适配器把第三方异常（`SQLException`、`IOException`、SDK 异常等）转换为端口约定的异常，不直接抛给内层。
- 入站适配器统一把内层异常转换为响应（如 `@RestControllerAdvice`）。
- 不要吞掉异常；必须捕获时记录日志并带上 traceId 等上下文。

### 新代码放哪里

- 新的业务概念、业务规则 → 领域层。
- 新的业务流程（收到什么、做什么、通知谁）→ 应用层的应用服务；需要外部能力时在 `port` 中新增接口。
- 新的外部系统接入（交易所、存储、消息）→ 出站适配器，实现已有或新增的端口。
- 新的触发方式（新接口、新定时任务、新消费者、新的 WS 推送接收）→ 入站适配器。
- 新增模块或适配器：先更新「本项目的层级」表和 ArchUnit 配置，再写代码。

### 检查方式

- Maven 多模块：`xxx-domain` 的 `pom.xml` 中不得出现 Spring、数据库、消息相关依赖；`mvn dependency:tree -pl xxx-domain` 可确认。
- 维护 ArchUnit 测试（包名按「本项目的层级」表调整）：

```java
@AnalyzeClasses(packages = "com.xxx")
class OnionArchitectureTest {

    @ArchTest
    static final ArchRule onion = onionArchitecture()
            .domainModels("..domain.model..")
            .domainServices("..domain.service..")
            .applicationServices("..application..")
            .adapter("web", "..adapter.in.web..")
            .adapter("ws", "..adapter.in.ws..")
            .adapter("job", "..adapter.in.job..")
            .adapter("exchange", "..adapter.out.exchange..")
            .adapter("persistence", "..adapter.out.persistence..")
            .adapter("mq", "..adapter.out.mq..");

    @ArchTest
    static final ArchRule domainIsPure = classes().that().resideInAPackage("..domain..")
            .should().onlyDependOnClassesThat()
            .resideInAnyPackage("..domain..", "java..", "lombok..");

    @ArchTest
    static final ArchRule applicationIsPure = classes().that().resideInAPackage("..application..")
            .should().onlyDependOnClassesThat()
            .resideInAnyPackage("..domain..", "..application..", "java..", "lombok..",
                    "org.springframework.transaction.annotation..");
}
```

- 提交前运行 `mvn test`。

### 已知的历史违规

> 记录现存不符合上述规则的代码，避免被模仿；修改相关代码时顺手修正。没有则保持为空。

- （无）
