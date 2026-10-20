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

## 架构工作方式

- 跨层改动时，先用几行说明计划：改哪些文件、每项落在哪一层、哪个模块和包，是否涉及「必须停下来问」的事项，再动手。

## 架构硬性约束

本节只保留与架构相关的约束；通用约束见根 `CLAUDE.md`。

1. **先定位再写**：动手前确定代码属于哪一层、哪个 Maven 模块和包。落不进「本项目的层级」表就停下来问，不要自创模块或包。
2. **必须停下来问**：层级归属不明；新增模块或层。

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

## 架构自查

- 代码在正确的层和模块，依赖方向符合本文件；entry 不直接依赖 infra。
