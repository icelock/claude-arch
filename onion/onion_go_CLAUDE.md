# CLAUDE.md

## 架构声明

本项目采用**洋葱架构**：从内到外分为**领域层（domain）→ 应用层（application）→ 外层（adapter + cmd）**，依赖只能由外向内。

```
外层：adapter/in（入站）、adapter/out（出站）、cmd（组装）
  └── 应用层：用例编排、出站端口
        └── 领域层：实体、值对象、领域服务、领域事件
依赖只能由外向内
```

六边形架构和整洁架构与本项目是同一条依赖规则的不同命名：六边形强调入站/出站端口对称，整洁架构强调用例输入/输出端口。**本项目统一按洋葱落法，不要引入第四种命名、分层或目录结构。**

## 对 AI 的硬性约束

本节是 MUST，优先于你的默认习惯和所谓的「最佳实践」。

1. **先定位再写**：动手前确定代码属于哪一层、哪个包。落不进「本项目的层级」表就停下来问，不要自创目录。
2. **先读后写**：找同层 1–2 个现有实现，沿用其结构、命名、错误处理和日志方式。不要引入仓库里没有的新风格。
3. **最小改动**：只改与需求相关的代码。不顺手重构、不重排 import、不升级依赖、不改格式化配置。
4. **抽象准入**：没有第二个实现、不属于真实 IO 边界时，不建端口、接口、工厂、基类、兼容层、fallback、feature flag。三行重复优于一个错误抽象。
5. **依赖准入**：不新增第三方库。确需新增时，先说明用途、体积、维护状态，等用户确认后再改 `go.mod`。
6. **不写占位**：不留 TODO、mock 返回值、假数据、注释掉的代码、未使用的参数或字段。
7. **验证真实**：完成前运行本节列出的全部检查，贴出命令和真实输出。失败就说失败并继续修，禁止改断言、跳过测试、更新快照、放宽 depguard 规则来让结果变绿。
8. **必须停下来问**：层级归属不明；新增包或层；新增依赖；修改公共类型、端口签名、对外接口、消息格式、表结构；需要放宽架构检查；修复历史违规会显著扩大改动范围。

## 分层与依赖

### 各层职责

| 层 | 职责 | 典型内容 |
|---|---|---|
| 领域层 | 业务概念和业务规则，不依赖框架和 IO | 实体、值对象、领域服务、领域事件、领域错误 |
| 应用层 | 编排用例：调用领域对象和端口，控制事务；定义出站端口 | 应用服务、端口接口、命令/结果类型、应用错误 |
| 外层·入站适配器 | 接收外部触发，转换为内层类型后调用应用层 | HTTP/gRPC handler、WS 推送接收、MQ 消费、定时任务 |
| 外层·出站适配器 | 实现应用层端口，访问外部系统 | 交易所下单客户端、数据库仓储、Redis、Kafka 发布 |
| 外层·组装 | 创建对象、注入依赖、启停 goroutine，只做接线 | `cmd/<app>/main.go` |

不设单独的 common 层：业务通用类型放领域层；与业务无关的纯函数放 `internal/domain/shared`，不得依赖 IO。

### 本项目的层级

> 下表为示例，按实际项目修改。新增包、拆分适配器时，同步更新本表和 depguard 配置。

| 层 | 包路径 | 可以依赖 | 外部依赖限制 |
|---|---|---|---|
| 领域层 | `internal/domain/...` | 无 | 只允许标准库非 IO 包和 `shopspring/decimal` |
| 应用层 | `internal/application/...`（端口在 `ports.go`） | domain | 禁 `net/http`、`database/sql`、gorm、redis、kafka |
| 入站适配器 | `internal/adapter/in/...` | application、domain | 不直接 import `adapter/out` |
| 出站适配器 | `internal/adapter/out/...` | application、domain | 不直接 import `adapter/in` |
| 组装 | `cmd/<app>` | 所有包 | 无 |

### 依赖规则

- **依赖只能由外向内**：domain 不 import 任何项目包；application 只 import domain；适配器 import application 和 domain；cmd import 所有包。
- **适配器之间互不依赖**：`adapter/in` 不 import `adapter/out`，不同出站适配器之间也不互相 import。只读查询也必须走应用层的查询服务，不得由入站适配器直连出站查询。
- **领域层保持纯净**：不 import `net/http`、`database/sql`、gorm、go-redis、sarama、`encoding/json`；领域类型不加 `json`/`gorm` tag，序列化格式由适配器决定。
- **应用层不做 IO**：只依赖 domain 和标准库中的 `context`、`errors`、`sync`、`time` 等。
- 禁止用包级变量保存服务对象、在 `init()` 中装配依赖；所有依赖在 cmd 通过 `NewXxx(...)` 传入。
- 所有跨层调用的第一个参数是 `context.Context`，一路传递；下层不得自行创建 `context.Background()`。

### 新代码放哪里

- 新的业务概念、业务规则 → domain。
- 新的业务流程 → application 的应用服务；需要外部能力时在 `ports.go` 新增接口。
- 新的外部系统接入 → `adapter/out`，实现端口。
- 新的触发方式 → `adapter/in`，只做转换和调用。
- 新增包：先更新层级表和 depguard 配置，再写代码。

## 端口与跨边界类型

- **出站端口定义在应用层**的 `ports.go`。Go 的接口由使用方定义：使用端口的是应用服务，所以接口在应用层。只有领域服务本身需要调用的端口才放领域层。
- 入站端口默认不定义，入站适配器直接调用应用服务；只有需要多种实现时才定义。
- 端口按业务需要设计（`Submit(ctx, domain.HedgeOrder)`），不按外部系统 API 设计（禁止 `Submit(ctx, BinanceOrderRequest)`）。
- **跨边界的类型归内层所有，转换由外层负责**：入站适配器把 HTTP/WS/MQ 报文转换为领域类型或应用命令；出站适配器把领域类型转换为交易所请求、数据库行、Kafka 消息，返回时再转回内层类型。
- 交易所报文、数据库模型、Kafka 消息体只能出现在适配器包中。
- 出站适配器用 `var _ application.HedgeOrderGateway = (*OrderGateway)(nil)` 在编译期确认实现。

```go
// internal/domain：业务规则，不依赖 IO
type HedgeOrder struct{ /* ... */ }

func NewHedgeOrder(t Trade, s Symbol) (HedgeOrder, error) { /* 计算方向、价格、数量 */ }

// internal/application/ports.go
type HedgeOrderGateway interface {
    Submit(ctx context.Context, o domain.HedgeOrder) (domain.OrderAck, error)
}

// internal/application：编排，不写业务计算
func (s *HedgeService) OnTrade(ctx context.Context, t domain.Trade) error {
    symbol, err := s.symbols.Get(ctx, t.Symbol)
    if err != nil { return fmt.Errorf("load symbol: %w", err) }
    order, err := domain.NewHedgeOrder(t, symbol)
    if err != nil { return fmt.Errorf("build hedge order: %w", err) }
    ack, err := s.gateway.Submit(ctx, order)
    if err != nil { return fmt.Errorf("submit hedge order: %w", err) }
    return s.publisher.Publish(ctx, order.Placed(ack))
}

// internal/adapter/out/binance：实现端口，在这里转换外部格式
var _ application.HedgeOrderGateway = (*OrderGateway)(nil)

func (g *OrderGateway) Submit(ctx context.Context, o domain.HedgeOrder) (domain.OrderAck, error) {
    resp, err := g.client.Place(ctx, toRequest(o))
    if err != nil { return domain.OrderAck{}, fmt.Errorf("place order: %w", err) }
    return toAck(resp), nil
}

// cmd/<app>/main.go：组装
gateway := binance.NewOrderGateway(cfg.Binance)
hedge := application.NewHedgeService(symbols, gateway, publisher)
```

## 数据、时间与一致性

- 价格、数量、金额统一使用 `shopspring/decimal`，禁止 `float64`/`float32`；同一模块不得混用两种表示。
- 除法必须指定精度和舍入方式；比较用 `Cmp`，不用 `==`。金额从字符串或整数构造，不从浮点构造；溢出和精度丢失必须显式处理。
- 下游写入使用确定性幂等键，数据库加唯一约束；重复消息不能产生重复副作用。
- 消息流必须处理重复、乱序、gap、断线重连：gap 时作废本地缓存并重拉快照，不能跳过继续用；重连后必须重订阅。
- **domain 不取时间**：`now` 由应用层作为参数传入；应用层需要时可定义 `Clock` 端口，但 domain 不得依赖。延迟和超时测量用单调时钟，不用系统墙钟。

## 并发与性能

- 每个 goroutine 都必须能通过 `ctx` 退出；组装处用 `errgroup` 等待，不能启动后不管。
- 入站适配器不在读取循环中直接处理业务；投递到有界 channel，由应用层消费。
- 从 channel 读取必须判断关闭：`v, ok := <-ch`，`ok == false` 时退出；不能忙等零值。
- channel 在 cmd 创建，**由拥有者（cmd）关闭**；接收方永不关闭；多发送者用 ctx 取消，不关闭 channel。
- 优先使用有界 channel 并定义背压策略。行情类可以丢弃并置 gap 标记；订单、回报、成交类不能丢，满了应拒绝受理、熔断或告警。
- 读循环中禁止 `time.After`，用 `time.NewTimer` + `Reset`；退出时 `Stop`。
- 热路径禁止逐笔日志、`encoding/json`、`fmt.Sprintf`、反射、无界重试和不必要的堆分配。
- 锁内不做 IO、不调用回调；`sync.Mutex` 不得按值拷贝；不并发读写普通 map。

## 错误处理与事务

- domain 定义 `DomainError`，application 定义 `AppError` 和端口错误；适配器把第三方错误在本层转换，不直接暴露给内层。
- 包装错误用 `fmt.Errorf("...: %w", err)`，上层用 `errors.Is`/`errors.As` 判断。
- 不得忽略错误。确实需要忽略时用 `_ =` 并写注释说明原因。
- 失败不能只记日志后继续：必须明确重试（有上限和退避）、进入死信、熔断、返回错误或停机中的一种。
- 事务边界在 application；出站适配器只执行，不开事务；入站适配器不开事务。
- 事务内不做远程调用（交易所、HTTP、MQ 同步发送）；数据库与消息的一致性用 outbox 或提交后发送，不做双写。
- 业务流程中不使用 `panic`；只有 cmd 启动阶段遇到无法恢复的配置错误才允许 `log.Fatal`。

## 测试与验证

- 领域计算、状态流转必须有表驱动单元测试，覆盖正常、边界、非法输入；不依赖 IO 和 goroutine。
- 应用层测试使用端口的内存实现或 fake，不连接真实数据库、Redis、交易所。
- 适配器用集成测试或契约测试验证协议、序列化、重连；不要 mock 掉被测的转换逻辑。
- 测试必须确定性：不依赖当前时间、随机数、真实网络、执行顺序；时间通过参数或 fake clock 注入。
- 禁止 `time.Sleep` 等待异步结果，用 channel、条件变量或 `require.Eventually` 这类有上限的轮询。
- 禁止为了变绿修改断言、删除用例、加 `t.Skip`、放宽超时；测试只加严不放宽。
- 覆盖率不是目标，但核心业务分支必须有测试；新增分支不得无测试。

## AI 常见劣化模式

| 劣化写法 | 正确做法 |
|---|---|
| domain/application import 框架、DB、HTTP | 依赖方向只能由外向内 |
| 入站适配器直接调用出站适配器或查询包 | 走应用服务 + 端口 |
| 把类型挪到 `domain/shared` 让各层都能用 | 类型归真正拥有它的层，边界做转换 |
| 单实现也建端口 + 工厂 + 构造函数包装 | 直接用具体类型，出现第二实现再抽象 |
| 加 `if useNew { ... } else { ... }` 兼容层 | 删旧路径，保持单一路径 |
| 改断言、加 `t.Skip`、放宽超时让测试变绿 | 修代码；测试只加严 |
| 只跑单个测试就宣称全部通过 | 跑完整检查并贴输出 |
| `if err != nil { return nil }` / 只 log 继续 | 包装或转换后返回，或明确失败策略 |
| 金额用 `float64`、domain 里 `time.Now()` | `decimal.Decimal`；`now` 参数注入 |
| `go func(){}` 无 ctx、无等待，或 channel 满时阻塞读取 | `errgroup` + ctx；有界队列 + 背压策略 |

## 检查方式与完成定义

### 检查

- `go vet ./...`
- `golangci-lint run`：至少启用 `errcheck`、`govet`、`staticcheck`、`depguard`、`bodyclose`。
- `go test -race ./...`
- depguard 必须用 `list-mode: strict` + 白名单，只写 deny 列表会漏掉未列出的 SDK：

```yaml
# .golangci.yml
version: "2"
linters:
  enable: [depguard]
  settings:
    depguard:
      rules:
        domain:
          files: ["**/internal/domain/**"]
          list-mode: strict
          allow: ["$gostd", "github.com/shopspring/decimal"]
        application:
          files: ["**/internal/application/**"]
          list-mode: strict
          allow: ["$gostd", "example.com/project/internal/domain", "github.com/shopspring/decimal"]
        adapter-in:
          files: ["**/internal/adapter/in/**"]
          deny: ["example.com/project/internal/adapter/out"]
        adapter-out:
          files: ["**/internal/adapter/out/**"]
          deny: ["example.com/project/internal/adapter/in"]
```

- `go list -f '{{.ImportPath}}: {{join .Imports " "}}' ./internal/domain/...`：不应出现项目内其他层。

### 完成定义（DoD）

- [ ] 代码在正确的层，依赖方向正确，depguard 和适配器独立性检查通过。
- [ ] `go vet`、`golangci-lint`、`go test -race` 全部通过，并贴出真实输出。
- [ ] 新增业务分支有单元测试，覆盖失败路径。
- [ ] 没有 TODO、注释掉的代码、调试日志、未使用的参数。
- [ ] 错误处理、日志上下文、超时和取消符合本文件。
- [ ] 没有新增依赖、端口、工厂或兼容层；如确有必要，已在回复中说明理由。

## 已知的历史违规

> 记录现存不符合上述规则的代码，避免被模仿；修改相关代码时顺手修正。没有则保持为空。

- （无）
