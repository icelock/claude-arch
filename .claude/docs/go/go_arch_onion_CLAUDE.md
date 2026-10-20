# CLAUDE.md

## 架构声明

本项目采用**洋葱架构**：从内到外分为**领域层（domain）→ 应用层（application）→ 外层（adapter + cmd）**，依赖只能由外向内。

```
外层：adapter/in（入站）、adapter/out（出站）、cmd（组装）
  └── 应用层：用例编排、事务边界、出站端口
        └── 领域层：实体、值对象、领域服务、领域事件
依赖只能由外向内
```

**组装层（`cmd`）可以依赖所有层**：它只做接线，是唯一同时知道所有具体实现的地方；任何层都不能反过来依赖组装层。

六边形架构和整洁架构与本项目是同一条依赖规则的不同命名：六边形强调入站/出站端口对称，整洁架构强调用例输入/输出端口。**本项目统一按洋葱落法，不要引入第四种命名、分层或目录结构。**

## 规则优先级与工作方式

规则冲突时按以下顺序取舍：**用户当次明确指令 > 正确性与安全 > 本文件的架构规则 > 仓库现有写法 > 通用最佳实践**。例外：安全类规则（密钥、注入、权限校验）即使用户要求放宽，也要先说明风险并得到确认。现有代码违反本文件时不要模仿，新代码按本文件写。

- 跨层改动或改动超过 3 个文件时，先用几行说明计划：落在哪层、改哪些包、是否新增端口、是否涉及「必须停下来问」的事项，再动手。
- 修 bug 先写一个能复现问题的失败测试，再修代码，让它变绿。
- 小的歧义（命名、放在同层哪个子包、日志措辞）不要打断用户：选仓库惯例或最保守的做法，在回复中写明假设。
- 完成时按以下格式汇报：
  1. 改动文件及所属层；
  2. 关键决策和假设；
  3. 执行的检查命令和真实输出（失败也照贴）；
  4. 遗留问题和风险，包括发现但没有修的历史违规。

## 对 AI 的硬性约束

本节是 MUST，优先于你的默认习惯和所谓的「最佳实践」。

1. **先定位再写**：动手前确定代码属于哪一层、哪个包。落不进「本项目的层级」表就停下来问，不要自创目录。
2. **先读后写**：找同层 1–2 个现有实现，沿用其结构、命名、错误处理和日志方式。不要引入仓库里没有的新风格。
3. **最小改动**：只改与需求相关的代码。不顺手重构、不重排 import、不升级依赖、不改格式化或 lint 配置。
4. **抽象准入**：真实 IO 边界的出站端口是架构要求，必须建；一个外部能力对应一个端口，不按方法拆。除此之外，没有第二个实现时，不建接口、工厂、泛型仓储、兼容层、fallback、feature flag；纯计算不建端口。三行重复优于一个错误抽象。
5. **依赖准入**：不新增第三方库。确需新增时，先说明用途、体积、维护状态，等用户确认后再改 `go.mod`。
6. **不写占位**：不留 TODO、`panic("not implemented")`、mock 返回值、假数据、注释掉的代码、未使用的参数或字段。
7. **验证真实**：完成前运行「检查」一节列出的全部检查，贴出命令和真实输出。失败就说失败并继续修，禁止改断言、`t.Skip`、放宽超时、更新快照来让结果变绿。
8. **必须停下来问**：层级归属不明；新增包或层；新增依赖；修改公共类型、端口签名、对外接口、消息格式、表结构；修复历史违规会超出本次需求的改动范围。

## 分层与依赖

### 各层职责

| 层 | 职责 | 典型内容 |
|---|---|---|
| 领域层 | 业务概念和业务规则，不依赖框架和 IO | 实体、值对象、领域服务、领域事件、领域错误 |
| 应用层 | 编排用例：加载、调用领域对象、调用端口、发布事件、控制事务；定义出站端口 | 应用服务、查询服务、端口接口、命令/结果类型、应用错误 |
| 外层·入站适配器 | 接收外部触发，转换为内层类型后调用应用层 | HTTP/gRPC handler、WS 推送接收、MQ 消费者、定时任务 |
| 外层·出站适配器 | 实现应用层端口，访问外部系统 | 第三方服务客户端、数据库仓储、事务执行器、Redis、Kafka 发布 |
| 外层·组装 | 读取配置、创建对象、注入依赖、启停 goroutine，只做接线 | `cmd/<app>/main.go` |

不设单独的 common 层：业务通用类型放领域层；与业务无关的纯函数放 `internal/domain/shared`，不得依赖 IO。

### 本项目的层级

> 下表为示例，按实际项目修改。在已有层下新增子包不需要改。

| 层 | 包路径 | 可以依赖 | 外部依赖限制 |
|---|---|---|---|
| 领域层 | `internal/domain/...`（含 `domain/shared`） | 无 | 只允许标准库非 IO 包和 `shopspring/decimal` |
| 应用层 | `internal/application/...`（端口在 `ports.go`，查询在 `application/query`） | domain | 只允许标准库非 IO 包、domain、`shopspring/decimal` |
| 入站适配器 | `internal/adapter/in/...`（`http`、`ws`、`mq`、`job`） | application、domain | 不 import `adapter/out` |
| 出站适配器 | `internal/adapter/out/...`（`payment`、`persistence`、`mq`） | application、domain | 不 import `adapter/in` |
| 组装 | `cmd/<app>` | 所有包 | 无 |

### 依赖规则

- **依赖只能由外向内**：domain 不 import 任何项目包；application 只 import domain；适配器 import application 和 domain；cmd import 所有包。
- **适配器之间互不依赖**：`adapter/in` 不 import `adapter/out`，不同出站适配器之间也不互相 import。只读查询走应用层的查询服务。
- **领域层保持纯净**：不 import `net/http`、`database/sql`、gorm、go-redis、sarama、`encoding/json`；领域类型不加 `json`/`db`/`gorm` tag，序列化格式由适配器决定。
- **应用层不做 IO**：只依赖 domain 和标准库中的 `context`、`errors`、`fmt`、`sync`、`time` 等非 IO 包。
- 禁止用包级变量保存服务对象、在 `init()` 中装配依赖；所有依赖在 cmd 通过 `NewXxx(...)` 传入。

### 新代码放哪里

- 新的业务概念、业务规则 → domain：规则优先写在实体和值对象的方法里，跨实体的规则写领域服务。
- 新的业务流程 → application 的应用服务；需要外部能力时在 `ports.go` 新增接口。
- 新的外部系统接入 → `adapter/out`，实现端口。
- 新的触发方式 → `adapter/in`，只做转换和调用。
- 新增包或适配器：先更新层级表，再写代码。

## 端口与跨边界类型

- **出站端口定义在应用层**的 `ports.go`。Go 的接口由使用方定义：使用端口的是应用服务，所以接口在应用层。只有领域服务本身需要调用的端口才放领域层。
- 入站端口默认不定义，入站适配器直接调用应用服务；只有需要多种实现时才定义。
- 端口按业务需要设计（`Charge(ctx, domain.Order)`），不按外部系统 API 设计（禁止 `Charge(ctx, StripeChargeRequest)`）。
- **查询侧**：只读查询放 `application/query` 的查询服务，查询端口放 `ports.go`，由持久化适配器实现；可以直接返回只读视图结构体，不必经过领域模型。入站适配器不得直接访问数据库。
- **跨边界的类型归内层所有，转换由外层负责**：入站适配器把 HTTP/WS/MQ 报文转换为领域类型或应用命令；出站适配器把领域类型转换为第三方请求、数据库行、Kafka 消息，返回时再转回内层类型。
- 出站适配器用 `var _ application.PaymentGateway = (*StripeGateway)(nil)` 在编译期确认实现。

```go
// internal/domain：业务规则，不依赖 IO
func (o *Order) StartPayment(now time.Time) error { /* 只有「待支付」可以发起，否则返回领域错误 */ }
func (o *Order) MarkPaid(r PaymentReceipt, now time.Time) (OrderPaid, error) { /* 只有「支付中」可以确认；生成领域事件 */ }

// internal/application/ports.go
type PaymentGateway interface {
    // Charge 的实现必须以订单号作为幂等键，重复调用不会重复扣款。
    Charge(ctx context.Context, o *domain.Order) (domain.PaymentReceipt, error)
}

type TxManager interface {
    // WithinTx 在一个数据库事务中执行 fn；fn 内的仓储调用必须使用传入的 ctx，才能落在同一个事务里。
    WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// internal/application：只编排，不写业务计算；远程调用在事务外
func (s *PayOrderService) Pay(ctx context.Context, id domain.OrderID) error {
    var order *domain.Order
    // 事务 1：待支付 → 支付中，提交后才调用外部
    err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
        o, err := s.orders.Get(ctx, id)
        if err != nil { return fmt.Errorf("load order: %w", err) }
        if err := o.StartPayment(s.clock.Now()); err != nil { return err }
        order = o
        return s.orders.Save(ctx, o)
    })
    if err != nil { return fmt.Errorf("start payment: %w", err) }

    // 事务外；超时可安全重试。失败或进程崩溃时订单停在「支付中」，由对账任务按订单号补偿
    receipt, err := s.gateway.Charge(ctx, order)
    if err != nil { return fmt.Errorf("charge: %w", err) }

    // 事务 2：支付中 → 已支付，领域事件写入 outbox，与订单在同一事务提交
    return s.tx.WithinTx(ctx, func(ctx context.Context) error {
        o, err := s.orders.Get(ctx, id)
        if err != nil { return fmt.Errorf("load order: %w", err) }
        ev, err := o.MarkPaid(receipt, s.clock.Now())
        if err != nil { return err }
        if err := s.orders.Save(ctx, o); err != nil { return fmt.Errorf("save order: %w", err) }
        return s.outbox.Append(ctx, ev)
    })
}

// internal/adapter/out/payment：实现端口，在这里转换外部格式
var _ application.PaymentGateway = (*StripeGateway)(nil)

func (g *StripeGateway) Charge(ctx context.Context, o *domain.Order) (domain.PaymentReceipt, error) {
    resp, err := g.client.Charge(ctx, toRequest(o))
    if err != nil { return domain.PaymentReceipt{}, fmt.Errorf("charge order %s: %w", o.ID(), err) }
    return toReceipt(resp), nil
}

// cmd/<app>/main.go：组装
tx := persistence.NewTxManager(db)
pay := application.NewPayOrderService(tx, persistence.NewOrderRepo(db), payment.NewStripeGateway(cfg.Stripe), persistence.NewOutbox(db), clock)
```

### 层级内部规则

- 领域层：不直接取当前时间、随机数、全局 ID，这些都通过参数传入；必须有纯单元测试。
- 聚合：一个事务只修改一个聚合；聚合之间按 ID 引用，不持有对方对象；跨聚合的一致性用领域事件实现最终一致。Repository 按聚合定义，一个聚合一个。
- 应用层：只做加载、调用领域对象、调用端口、发布事件、控制事务；不写金额计算，也不写承载业务规则的条件分支。幂等、去重、并发控制属于应用层，具体实现通过端口交给外层。
- 外层：入站只转换和调用；出站只处理协议、签名、序列化、连接、重连、持久化，不做业务判断。回调运行在 IO 读取 goroutine 上时，耗时处理必须交给其他 goroutine。

## AI 常见劣化模式

| 劣化写法 | 正确做法 |
|---|---|
| domain/application import `net/http`、`database/sql`、gorm | 依赖方向只能由外向内 |
| 入站适配器直接调用出站适配器或访问数据库 | 走应用服务 + 端口；查询走 `application/query` |
| 端口签名里出现第三方请求类型 | 端口只用内层类型，转换放在适配器 |
| 单实现建工厂、为纯计算建端口，或加 `if useNew` 兼容层 | 只为 IO 边界建端口，其余用具体类型，保持单一路径 |
| 应用服务里写金额计算和业务 if-else | 规则下沉到实体、值对象或领域服务 |
| 一个事务里修改多个聚合 | 按 ID 引用，跨聚合用领域事件 |
| 远程调用放在 `WithinTx` 里，或先调外部、后落库且没有幂等键 | 先落库「处理中」，以业务 ID 为幂等键调用外部，再落库结果 |
| `if err != nil { return nil }`、只打日志后继续 | 包装后返回，或明确重试、死信、熔断 |
| `go func(){}` 无 ctx、无等待，或 channel 满时阻塞读取循环 | `errgroup` + ctx；有界 channel + 背压策略 |
| 改断言、加 `t.Skip`，或只跑单个测试就宣称通过 | 修代码，跑完整检查并贴输出 |

## 检查方式与完成定义

### 检查

- `go vet ./...`
- `golangci-lint run --enable bodyclose,rowserrcheck,sqlclosecheck,noctx`：默认已含 `errcheck`、`govet`、`staticcheck`。
- `go test -race ./...`
- 以下规则靠自查和 review：内层使用 `net` 包做 socket IO、端口按业务设计而不是按外部 API 设计、应用层不写业务规则、一个事务只改一个聚合、金额构造与舍入、事务内远程调用、锁内 IO、日志中的敏感信息、channel 满时的策略。

### 完成定义（DoD）

- [ ] 代码在正确的层，依赖方向符合本文件。
- [ ] `go vet`、`golangci-lint`、`go test -race` 全部通过，并贴出真实输出。
- [ ] 新增业务分支有单元测试，覆盖失败路径；修 bug 有复现测试。
- [ ] 没有 TODO、注释掉的代码、调试日志、未使用的参数。
- [ ] 错误处理、日志、超时与 ctx 取消、配置方式符合本文件。
- [ ] 没有新增依赖、工厂或兼容层；新增端口对应真实 IO 边界；如有例外，已在回复中说明理由。
- [ ] 已按「工作方式」的格式汇报。

## 已知的历史违规

> 记录现存不符合上述规则的代码，避免被模仿。只在本次需求本来就要改到的代码里顺带修正；超出这个范围的不动，在汇报中列出。没有则保持为空。

- （无）

## 附录：实时交易系统（可选）

> 非交易类项目删除本节。本节规则补充正文，冲突时以本节为准。

- 交易所报文、签名、限频（rate limit）、错误码只在出站适配器处理；端口按业务设计（`Submit(ctx, domain.HedgeOrder)`），禁止 `Submit(ctx, BinanceOrderRequest)`。
- 下单使用确定性的 `clientOrderId` 作为幂等键，超时或网络错误后先按它查询订单状态再决定是否重下，禁止盲目重试。
- 行情、订单推送必须处理重复、乱序、gap、断线重连：出现 gap 时作废本地缓存并重拉快照，不能跳过继续用；重连后必须重新订阅。
- 背压分级：行情类 channel 满了可以丢弃并置 gap 标记；订单、回报、成交类不能丢，满了拒绝受理或熔断。
- WS 读取循环只做解析和投递，不写数据库、不 `time.Sleep`。
- 热路径禁止逐笔日志、`encoding/json`、`fmt.Sprintf`、反射、无界重试和不必要的堆分配。
- 价格和数量按交易对的 tickSize/stepSize 取整，取整规则写在领域值对象中（如 `Symbol.AlignPrice(price, side)`），舍入方向由买卖方向决定，不依赖默认值。
