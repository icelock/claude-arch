# CLAUDE.md

## 应用架构：分层架构

本项目采用分层架构，依赖只能自上而下单向流动。层级、Maven 模块和包名根据业务需要定义，但必须遵守下面的依赖规则。新增或修改代码前，先确认代码属于哪一层。

### 层的角色

每一层都必须归入以下某种角色。可以按业务拆出更多层或子包，但不能出现不属于任何角色的层。

| 角色 | 职责 | 典型内容 |
|---|---|---|
| 入口 | 接收外部触发，转换后调用业务层 | Controller、WS 推送接收、MQ Consumer、定时任务、启动事件监听 |
| 业务 | 业务规则与流程编排 | 计算、决策、状态流转、结果处理、事务边界 |
| 基础设施 | 技术细节 | 交易所/第三方客户端、Mapper/Repository、Redis、Kafka、文件、缓存加载 |
| 公共 | 各层共用的类型与工具，不依赖任何层 | 通用 DTO、枚举、常量、异常基类、工具类 |
| 组装 | Bean 装配、配置绑定、生命周期，只做接线 | `@SpringBootApplication` 启动类、`@Configuration` 配置类 |

### 本项目的层级

> 新增层、拆分模块或包时，同步更新本表和依赖图。下表为示例，按实际项目修改。

| 层 | Maven 模块 / 包 | 角色 | 可以依赖 |
|---|---|---|---|
| boot | `xxx-boot` / `com.xxx.boot`、`com.xxx.config` | 组装 | 所有层 |
| entry | `xxx-entry` / `com.xxx.controller`、`com.xxx.listener`、`com.xxx.schedule` | 入口 | service、common |
| service | `xxx-service` / `com.xxx.service` | 业务 | infra、common |
| infra | `xxx-infra` / `com.xxx.infra` | 基础设施 | common |
| common | `xxx-common` / `com.xxx.common` | 公共 | 无（只允许 JDK 和基础工具库） |

```
boot ──> entry ──> service ──> infra ──> common
  └────────┴──────────┴────────────────> common
```

### 依赖规则

- 上层可以依赖下层，**下层禁止依赖上层**：
  - Maven 多模块：下层模块的 `pom.xml` 不得声明上层模块依赖（由编译器保证）。
  - 单模块多包：下层包中不得 `import` 上层包的任何类（由 ArchUnit 测试保证，见「检查方式」）。
- 依赖图中不能有环，包括 Spring Bean 之间的循环依赖。新增层时，先确定它在图中的位置，再写代码。
- 禁止通过容器或静态持有绕过依赖规则，例如 `ApplicationContext.getBean(...)`、`SpringContextHolder.getBean(...)`、静态字段保存 Bean。所有依赖通过注入获得，优先使用构造器注入（`final` 字段 + `@RequiredArgsConstructor`）。
- 业务层不直接使用中间件或 SDK 的 API（`RedisTemplate`/`RedisCommands`、`KafkaTemplate`、Mapper、HTTP/WS 客户端等），必须通过基础设施层封装的类访问。
- 基础设施层只处理技术问题（协议、签名、序列化、连接、重连、持久化），不做业务判断。
- 入口层只做参数转换、校验和调用，不写业务逻辑，不直接调用基础设施层。
- 公共层不依赖 Spring、数据库、网络相关库，保持纯数据和纯函数。
- 数据库 PO（表结构对象）只在基础设施层内使用，返回给上层前转换为业务需要的 DTO。

### 下层通知上层：下层定义接口或事件，上层实现/消费

当下层需要把结果交给业务层（下单回报、WS 推送、MQ 消息等）时，**不能直接调用上层**。按以下两种方式之一处理。

#### 方式一：回调接口（单次结果、请求-回报型）

1. 接口定义在下层，与调用它的类放在一起。
2. 上层实现该接口。
3. 由 Spring 注入把实现交给下层，下层只持有接口类型。

```java
// infra：下层定义接口，签名只用 infra 或 common 中的类型
public interface OrderResultListener {
    void onOrderResult(OrderResultEvent event);
}

// infra：下层只依赖自己定义的接口
@Component
@RequiredArgsConstructor
public class OrderClient {
    private final OrderResultListener listener;
}

// service：上层实现下层接口（service -> infra，方向合法）
@Service
public class HedgeService implements OrderResultListener {
    @Override
    public void onOrderResult(OrderResultEvent event) { /* ... */ }
}
```

多个实现需要同时接收时，下层注入 `List<OrderResultListener>` 逐个调用。

#### 方式二：事件或队列（持续推送、流式数据）

- **低频事件**：事件类定义在下层或 common，下层通过 `ApplicationEventPublisher` 发布，上层用 `@EventListener` 消费。Spring 事件默认同步执行，需要异步时配合 `@Async` 和指定的线程池。
- **高频推送**（行情、成交等）：下层把转换后的事件投递到 `BlockingQueue` / Disruptor，上层在自己的线程中消费。队列在组装处创建，两端分别注入。

```java
// boot：组装
@Bean
public BlockingQueue<TradeEvent> tradeEventQueue() {
    return new ArrayBlockingQueue<>(10_000);
}

// infra：只负责投递
@Component
@RequiredArgsConstructor
public class TradeWsClient {
    private final BlockingQueue<TradeEvent> tradeEventQueue;

    void onMessage(String raw) {
        if (!tradeEventQueue.offer(toEvent(raw))) {
            // 队列满的处理策略：告警 / 丢弃 / 降级，必须明确
        }
    }
}

// service：在自己的线程中消费
```

#### 约束

- 接口和事件类的签名只能使用下层或 common 中的类型，不能出现上层的类。
- 事件携带的是转换后的类型，不能直接传递交易所原始报文或 SDK 类型。
- 参数较多时封装成事件对象，不要堆叠参数。
- 不要在上层把自己注册给下层（如 `@PostConstruct` 中 `client.setListener(this)`），由注入完成装配。
- 回调运行在 IO/WS/Netty 线程上时，耗时处理必须投递到业务线程池，不能阻塞回调线程。
- 有界队列需要明确满了以后的策略（等待、丢弃、告警），并写在代码注释中。
- 只有一个调用方且无需解耦时，上层直接调用下层即可，不要为了形式增加接口。

### 异常与事务

- 基础设施层把第三方异常（`SQLException`、`IOException`、SDK 异常等）转换为本层定义的异常，不直接抛给上层。
- 业务异常定义在业务层或 common，入口层统一转换为响应（如 `@RestControllerAdvice`）。
- `@Transactional` 只加在业务层的方法上，不加在入口层和基础设施层。
- 不要吞掉异常；必须捕获时记录日志并带上 traceId 等上下文。

### 新代码放哪里

- 新的外部系统接入（交易所、第三方 API、存储）→ 基础设施层，协议和报文转换在该层内完成。
- 新的业务规则或流程 → 业务层；计算逻辑尽量写成不依赖 Spring 和 IO 的普通类，并配单元测试。
- 新的触发方式（新接口、新定时任务、新消费者、新的 WS 推送接收）→ 入口层，只做转换和调用。
- 多个层都用到的类型 → common；只在一层内使用的类型留在该层，不要提前放到 common。
- 交易所报文 DTO 与业务 DTO 分开命名和存放。
- 不属于现有任何层的代码：先在「本项目的层级」表中新增层、明确角色和依赖，再写代码。

### 检查方式

- Maven 多模块：`mvn dependency:tree -pl <下层模块>` 的输出中不应出现上层模块。
- 单模块多包：维护 ArchUnit 测试，示例如下（包名按「本项目的层级」表调整）：

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

    @ArchTest
    static final ArchRule noGetBean = noClasses().that().resideOutsideOfPackage("..config..")
            .should().callMethod(ApplicationContext.class, "getBean", Class.class);
}
```

- 提交前运行 `mvn test`。

### 已知的历史违规

> 记录现存不符合上述规则的代码，避免被模仿；修改相关代码时顺手修正。没有则保持为空。

- （无）
