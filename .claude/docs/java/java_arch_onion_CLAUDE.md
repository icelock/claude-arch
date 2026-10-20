# CLAUDE.md

## 架构声明

本项目采用**洋葱架构**：从内到外分为**领域层（domain）→ 应用层（application）→ 外层（adapter + boot）**，依赖只能由外向内。

```
外层：adapter.in（入站）、adapter.out（出站）、boot（组装）
  └── 应用层：用例编排、事务边界、出站端口
        └── 领域层：实体、值对象、领域服务、领域事件
依赖只能由外向内
```

**组装层（`boot`）可以依赖所有层**：它只做接线，是唯一同时知道所有具体实现的地方；任何层都不能反过来依赖组装层。

六边形架构和整洁架构与本项目是同一条依赖规则的不同命名：六边形强调入站/出站端口对称，整洁架构强调用例输入/输出端口。**本项目统一按洋葱落法，不要引入第四种命名、分层或目录结构。**

## 架构工作方式

- 跨层改动时，先用几行说明计划：改哪些文件、每项落在哪一层、哪个模块和包，是否新增端口，是否涉及「必须停下来问」的事项，再动手。

## 架构硬性约束

本节只保留与架构相关的约束；通用约束见根 `CLAUDE.md`。

1. **先定位再写**：动手前确定代码属于哪一层、哪个 Maven 模块和包。落不进「本项目的层级」表就停下来问，不要自创模块或包。
2. **端口准入**：真实 IO 边界的出站端口是架构要求，必须建；一个外部能力对应一个端口，不按方法拆；纯计算不建端口。
3. **必须停下来问**：层级归属不明；新增模块或层；修改端口签名。

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

> 下表为示例，按实际项目修改。新增模块、拆分适配器时同步更新本表。

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
- 新增模块或适配器：先更新层级表，再写代码。

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

## 架构自查

- 代码在正确的层和模块，依赖方向由外向内；适配器之间互不依赖。
- 端口按业务设计而不是按外部 API 设计；新增端口对应真实 IO 边界。
- 应用层不写业务规则；一个事务只修改一个聚合。
