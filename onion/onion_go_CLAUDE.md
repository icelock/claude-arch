# CLAUDE.md

## 应用架构：洋葱架构（三层）

本项目采用洋葱架构，从内到外分为三层：**领域层（domain）→ 应用层（application）→ 外层（adapter + cmd）**。依赖只能由外向内。包名可以按业务调整，但必须遵守下面的依赖规则。新增或修改代码前，先确认代码属于哪一层。

```
┌──────────────────── 外层 ─────────────────────┐
│ adapter/in（入站）  adapter/out（出站）  cmd     │
│   ┌────────────── 应用层 ──────────────┐       │
│   │ 用例编排、出站端口接口               │       │
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
| 领域层 | 业务概念和业务规则，不依赖 IO | 实体、值对象、领域服务、领域事件、领域错误 |
| 应用层 | 编排用例：调用领域对象和端口；定义出站端口接口 | 应用服务、端口接口、命令/结果类型、应用错误 |
| 外层·入站适配器 | 接收外部触发，转换为内层类型后调用应用层 | HTTP/gRPC handler、WS 推送接收、MQ 消费、定时任务 |
| 外层·出站适配器 | 实现应用层的端口接口，访问外部系统 | 交易所下单客户端、数据库仓储、Redis、Kafka 发布 |
| 外层·组装 | 创建对象、注入依赖、启动和关闭 goroutine，只做接线 | `cmd/<app>/main.go` |

不设单独的 common 层：业务通用类型放在领域层；与业务无关的纯工具函数放在 `internal/domain/shared`，不得依赖 IO 相关的包。

### 本项目的层级

> 新增包、拆分适配器时，同步更新本表和 depguard 配置。下表为示例，按实际项目修改。

| 层 | 包路径 | 可以依赖 |
|---|---|---|
| 领域层 | `internal/domain/...` | 无（只允许标准库中非 IO 的包和 `shopspring/decimal` 等基础库） |
| 应用层 | `internal/application/...`（端口定义在 `ports.go`） | domain |
| 入站适配器 | `internal/adapter/in/...`（如 `in/http`、`in/ws`、`in/job`） | application、domain |
| 出站适配器 | `internal/adapter/out/...`（如 `out/binance`、`out/kafka`、`out/db`） | application、domain |
| 组装 | `cmd/<app>` | 所有包 |

### 依赖规则

- **依赖只能由外向内**：domain 不 import 任何项目包；application 只 import domain；适配器 import application 和 domain；cmd import 所有包。
  - 注意：Go 编译器只禁止循环 import，不禁止反向 import，必须用 depguard 检查（见「检查方式」）。
- **适配器之间互不依赖**：`adapter/in` 不 import `adapter/out`；不同出站适配器之间也不互相 import。
- **领域层保持纯净**：不 import `net/http`、`database/sql`、`gorm`、`go-redis`、`sarama`、`encoding/json` 等；领域类型不加 `json`/`gorm` tag，序列化格式由适配器决定。
- **应用层不做 IO**：只依赖 domain 和标准库中的 `context`、`errors`、`sync` 等。
- 禁止用包级变量保存服务对象、在 `init()` 中装配依赖；所有依赖在 cmd 中通过 `NewXxx(...)` 传入。
- 所有跨层调用的第一个参数传递 `context.Context`。

### 端口与跨边界类型

- **出站端口接口定义在应用层**。这符合 Go「接口由使用方定义」的惯例：使用端口的是应用服务，所以接口在应用层。只有领域服务本身需要调用的端口才放在领域层。
- 端口按业务需要设计（`Submit(ctx, domain.HedgeOrder)`），不能按外部系统的 API 设计。
- **跨边界的类型归内层所有，转换由外层负责**：
  - 入站：适配器把 HTTP 请求、WS 报文、MQ 消息转换为领域类型或应用命令，再调用应用层。
  - 出站：适配器把领域类型转换为交易所请求、数据库行、Kafka 消息，返回时再转换回内层类型。
- 交易所报文、数据库模型、Kafka 消息体只能出现在适配器包中。
- 出站适配器用 `var _ application.HedgeOrderGateway = (*OrderGateway)(nil)` 在编译期确认实现了端口。

```go
// ===== internal/domain：业务规则，不依赖 IO =====
type Symbol struct {
    Code     string
    TickSize decimal.Decimal
    QtyScale int32
}

func (s Symbol) AlignPrice(price decimal.Decimal, side Side) decimal.Decimal { /* 按 TickSize 取整 */ }

func NewHedgeOrder(t Trade, s Symbol) (HedgeOrder, error) { /* 计算对冲方向、价格、数量 */ }

func (o HedgeOrder) Placed(ack OrderAck) HedgeOrderPlaced { /* 生成领域事件 */ }

// ===== internal/application/ports.go：出站端口 =====
type SymbolRepository interface {
    Get(ctx context.Context, code string) (domain.Symbol, error)
}

type HedgeOrderGateway interface {
    Submit(ctx context.Context, o domain.HedgeOrder) (domain.OrderAck, error)
}

type HedgeEventPublisher interface {
    Publish(ctx context.Context, e domain.HedgeOrderPlaced) error
}

// ===== internal/application：用例编排，不写业务计算 =====
type HedgeService struct {
    symbols   SymbolRepository
    gateway   HedgeOrderGateway
    publisher HedgeEventPublisher
}

func (s *HedgeService) OnTrade(ctx context.Context, t domain.Trade) error {
    symbol, err := s.symbols.Get(ctx, t.Symbol)
    if err != nil {
        return fmt.Errorf("load symbol: %w", err)
    }
    order, err := domain.NewHedgeOrder(t, symbol)
    if err != nil {
        return err
    }
    ack, err := s.gateway.Submit(ctx, order)
    if err != nil {
        return fmt.Errorf("submit hedge order: %w", err)
    }
    return s.publisher.Publish(ctx, order.Placed(ack))
}

func (s *HedgeService) Consume(ctx context.Context, trades <-chan domain.Trade) error {
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case t := <-trades:
            if err := s.OnTrade(ctx, t); err != nil {
                slog.ErrorContext(ctx, "处理成交失败", "err", err)
            }
        }
    }
}

// ===== internal/adapter/out/binance：实现端口，在这里做外部格式转换 =====
var _ application.HedgeOrderGateway = (*OrderGateway)(nil)

func (g *OrderGateway) Submit(ctx context.Context, o domain.HedgeOrder) (domain.OrderAck, error) {
    resp, err := g.client.Place(ctx, toRequest(o))
    if err != nil {
        return domain.OrderAck{}, fmt.Errorf("binance place order: %w", err)
    }
    return toAck(resp), nil
}

// ===== cmd/<app>/main.go：组装 =====
symbols := db.NewSymbolRepository(pool)
gateway := binance.NewOrderGateway(cfg.Binance)
publisher := kafka.NewEventPublisher(cfg.Kafka)
hedge := application.NewHedgeService(symbols, gateway, publisher)

trades := make(chan domain.Trade, 10000)
wsReceiver := ws.NewTradeReceiver(cfg.WS, trades) // 入站适配器：报文转为 domain.Trade 后发送

g, ctx := errgroup.WithContext(ctx)
g.Go(func() error { return wsReceiver.Run(ctx) })
g.Go(func() error { return hedge.Consume(ctx, trades) })
```

### 领域层规则

- 业务规则优先写在实体和值对象的方法中；跨多个实体的规则写成领域服务（普通函数）。
- 用类型表达约束：用自定义类型区分 `Price`、`Qty`、`AccountID` 等；通过构造函数校验，避免创建非法对象。
- 不直接调用 `time.Now()`、`rand`、全局 ID 生成器：通过参数传入，或在应用层定义 `Clock`、`IDGenerator` 端口。
- 金额、价格、数量使用 `decimal.Decimal`，不使用 `float64`。
- 领域层必须有表驱动单元测试，覆盖核心计算和状态流转。

### 应用层规则

- 只做编排：加载数据、调用领域对象、调用端口、发布事件。不写价格计算、条件分支判断等业务规则。
- 幂等、去重、并发控制属于应用层，具体实现（如 Redis 去重）通过端口交给外层。
- 应用服务的参数和返回值使用领域类型或应用层定义的命令/结果类型。
- 应用层测试使用端口的内存实现或 fake，不连接真实外部系统。

### 外层规则

- 入站适配器只做转换和调用，不写业务逻辑；持续推送通过 channel 交给应用层，不在读取循环中直接处理。
- 出站适配器只处理技术问题（协议、签名、序列化、连接、重连、持久化），不做业务判断。
- 带缓冲的 channel 需要明确满了以后的策略（阻塞、`select default` 丢弃、告警），并写在代码注释中。
- channel 由发送方关闭；所有 goroutine 都必须能通过 `ctx` 退出，cmd 负责等待它们结束。
- **只读查询例外**：报表、列表等不涉及业务规则的只读场景，可以由入站适配器调用外层的查询包直接返回 DTO，不经过领域层。

### 错误处理

- 领域层和应用层定义自己的错误（哨兵错误或错误类型）。
- 出站适配器把第三方错误（`sql.ErrNoRows`、`redis.Nil` 等）转换为应用层约定的错误，不直接暴露给内层。
- 包装错误使用 `fmt.Errorf("...: %w", err)`，上层用 `errors.Is` / `errors.As` 判断。
- 业务流程中不使用 `panic`；只在 cmd 启动阶段遇到无法恢复的配置错误时允许 `log.Fatal`。

### 新代码放哪里

- 新的业务概念、业务规则 → domain。
- 新的业务流程（收到什么、做什么、通知谁）→ application 的应用服务；需要外部能力时在 `ports.go` 中新增接口。
- 新的外部系统接入（交易所、存储、消息）→ `adapter/out` 下新建或扩展包，实现端口接口。
- 新的触发方式（新接口、新定时任务、新消费者、新的 WS 推送接收）→ `adapter/in`。
- 新增包：先更新「本项目的层级」表和 depguard 配置，再写代码。

### 检查方式

- 使用 golangci-lint 的 depguard 检查依赖（以 golangci-lint v2 配置为例，模块路径按 `go.mod` 修改）：

```yaml
# .golangci.yml
version: "2"
linters:
  enable:
    - depguard
  settings:
    depguard:
      rules:
        domain:
          files: ["**/internal/domain/**"]
          deny:
            - pkg: "example.com/project/internal/application"
              desc: "domain 不能依赖 application"
            - pkg: "example.com/project/internal/adapter"
              desc: "domain 不能依赖 adapter"
            - pkg: "net/http"
              desc: "domain 不能做 IO"
            - pkg: "database/sql"
              desc: "domain 不能做 IO"
            - pkg: "encoding/json"
              desc: "序列化由适配器负责"
        application:
          files: ["**/internal/application/**"]
          deny:
            - pkg: "example.com/project/internal/adapter"
              desc: "application 不能依赖 adapter"
            - pkg: "net/http"
              desc: "application 不能做 IO"
            - pkg: "database/sql"
              desc: "application 不能做 IO"
        adapter-in:
          files: ["**/internal/adapter/in/**"]
          deny:
            - pkg: "example.com/project/internal/adapter/out"
              desc: "入站适配器不能直接调用出站适配器"
        adapter-out:
          files: ["**/internal/adapter/out/**"]
          deny:
            - pkg: "example.com/project/internal/adapter/in"
              desc: "出站适配器不能依赖入站适配器"
```

- 快速查看依赖：`go list -f '{{.ImportPath}}: {{join .Imports " "}}' ./internal/domain/...`，不应出现项目内其他层的包。
- 提交前运行 `go vet ./...`、`golangci-lint run` 和 `go test -race ./...`。

### 已知的历史违规

> 记录现存不符合上述规则的代码，避免被模仿；修改相关代码时顺手修正。没有则保持为空。

- （无）
