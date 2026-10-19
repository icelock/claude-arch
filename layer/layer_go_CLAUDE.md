# CLAUDE.md

## 架构声明

本项目采用**经典分层架构**：依赖只能自上而下单向流动，层是封闭的，不能跳层。`cmd` 是组装根。

```
cmd ──> entry ──> service ──> infra ──> common
 └────────┴──────────┴────────────────> common
```

**这不是 Clean 架构、六边形架构或依赖倒置。** service 直接 import infra 的具体包，由 infra 提供能力。未经用户明确同意，不要：

- 在 service 定义端口接口、再由 infra 实现；
- 把 infra 类型抽象成接口层；
- 把分层改造成洋葱或整洁架构。

唯一例外是单元测试：service 可以在自己的包内定义 ≤3 个方法的窄接口，由 infra 的具体类型隐式满足，生产装配仍传具体类型。禁止系统性端口化、工厂、注册表。

## 对 AI 的硬性约束

本节是 MUST，优先于你的默认习惯和所谓的「最佳实践」。

1. **先定位再写**：动手前确定代码属于哪一层、哪个包。包路径必须落在「本项目的层级」表中；落不进去就停下来问，不要自创目录。
2. **先读后写**：找同层 1–2 个现有实现，沿用其结构、命名、错误处理和日志方式。不要引入仓库里没有的新风格。
3. **最小改动**：只改与需求相关的代码。不顺手重构、不重排 import、不升级依赖、不改格式化配置。
4. **抽象准入**：没有第二个实现、不属于真实 IO 边界时，不建接口、工厂、基类、泛型仓储、兼容层、fallback、feature flag。三行重复优于一个错误抽象。
5. **依赖准入**：不新增第三方库。确需新增时，先说明用途、体积、维护状态，等用户确认后再改 `go.mod`。
6. **不写占位**：不留 TODO、mock 返回值、假数据、注释掉的代码、未使用的参数或字段。
7. **验证真实**：完成前运行本节列出的全部检查，贴出命令和真实输出。失败就说失败并继续修，禁止改断言、跳过测试、更新快照、放宽架构规则来让结果变绿。
8. **必须停下来问**：层级归属不明；新增层或模块；新增依赖；修改公共类型、对外接口、消息格式、表结构；需要放宽架构检查；修复历史违规会显著扩大改动范围。

## 分层与依赖

### 层的角色

每一层必须归入以下某种角色。可以按业务拆子包，但不存在不属于任何角色的层。

| 角色 | 职责 | 典型内容 |
|---|---|---|
| 入口 | 接收外部触发，转换后调用业务层 | HTTP/gRPC handler、WS 推送接收、MQ 消费、定时任务、CLI |
| 业务 | 业务规则与流程编排 | 计算、决策、状态流转、结果处理、事务边界 |
| 基础设施 | 技术细节 | 交易所/第三方客户端、数据库、Redis、Kafka、文件、缓存加载 |
| 公共 | 各层共用的类型与工具，不依赖任何层 | 通用类型、枚举、哨兵错误、纯函数工具 |
| 组装 | 创建对象、注入依赖、启停 goroutine，只做接线 | `cmd/<app>/main.go`、`internal/bootstrap` |

### 本项目的层级

> 下表为示例，按实际项目修改。表与 `internal/archtest/arch_test.go` 中的 `architecture` 一一对应，两边必须同时改。在已有层下新增子包（如 `internal/infra/xxx`）不需要改。

| 层 | 包路径 | 可以依赖 | 外部包限制 |
|---|---|---|---|
| cmd | `cmd/...` | 所有层 | 无 |
| entry | `internal/entry/...` | service、common | 不直接用数据库、Redis、MQ、WS SDK |
| service | `internal/service/...` | infra、common | 禁 `net/http`、`database/sql`、gorm、redis、kafka、websocket |
| infra | `internal/infra/...` | common | 无 |
| common | `internal/common/...` | 无 | 只允许标准库，且禁 `net`、`database`、`os`、`io/fs` |

- 模块内每个包必须属于上表某一层，`internal/archtest/...` 除外。
- 同层内的包可以互相依赖，但目录之间不能形成循环。

### 依赖规则

- 上层可以依赖下层，**下层禁止 import 上层任何包**。Go 编译器只禁止循环 import，不禁止反向 import，必须靠 archtest 检查。
- 依赖图不能有环。编译器只禁止单包成环，不禁止子树互依；archtest 按目录层级逐层检查。
- 禁止通过包级变量、`init()`、反射或 `any` 绕过依赖规则。所有依赖在组装处通过构造函数传入。
- 业务层不直接使用中间件或 SDK 的 API（`database/sql`、gorm、go-redis、sarama/franz-go、websocket、HTTP 客户端），必须通过 infra 封装的类型访问。
- 基础设施层只处理技术问题（协议、签名、序列化、连接、重连、持久化），不做业务判断。
- 入口层只做解析、校验和调用，不写业务逻辑，不直接调用 infra。
- 公共层不依赖网络、数据库、IO 相关包，保持纯数据和纯函数。

### 新代码放哪里

- 新的外部系统接入 → infra，协议和报文转换在该层完成。
- 新的业务规则或流程 → service；计算尽量写成不依赖 IO 和 goroutine 的纯函数。
- 新的触发方式 → entry，只做转换和调用。
- 多层共用的只读类型 → common；只在一层内使用的类型留在该层，不要提前上移。
- 交易所报文、数据库模型、SDK 类型不得离开 infra；返回上层前转换为业务类型。

## 跨边界数据与下层通知上层

### 类型归属

- 数据库模型、表结构 struct、交易所原始报文、SDK 类型只在 infra 内使用。
- infra 返回给上层前，转换为 service 需要的类型；转换在 infra 内完成，不要在 service 里解析外部格式。
- common 只放跨层共用的只读类型、枚举、哨兵错误、纯函数工具；禁止放请求/响应 DTO、PO、带 `json`/`gorm` tag 的结构体。

### 方式一：回调接口或函数类型（单次结果、请求-回报型）

1. 接口定义在下层，与调用它的类型放在一起；签名只用下层或 common 的类型。
2. 上层类型隐式实现该接口。
3. 组装处把实现通过构造函数传给下层。

```go
// internal/infra/order：下层定义接口
type ResultListener interface {
    OnOrderResult(ctx context.Context, ev ResultEvent)
}

type Client struct{ listener ResultListener }

func NewClient(listener ResultListener) *Client { return &Client{listener: listener} }

// internal/service：上层实现（service -> infra，方向合法）
func (s *HedgeService) OnOrderResult(ctx context.Context, ev order.ResultEvent) { /* ... */ }

// cmd：组装
hedge := service.NewHedgeService(/* ... */)
client := order.NewClient(hedge)
```

只有一个方法时可用函数类型。只有一个调用方且无需解耦时，上层直接调用下层，不要为形式加接口。

### 方式二：channel（持续推送、流式数据）

1. 事件类型定义在下层或 common。
2. 下层持有只写 channel（`chan<- Event`），只负责发送。
3. 上层持有只读 channel（`<-chan Event`），在自己的 goroutine 中消费。
4. channel 在组装处创建；**由拥有者（组装处）关闭**。接收方永不关闭；多发送者时用 `ctx` 取消，不关闭 channel。

```go
events := make(chan ws.TradeEvent, 10_000)
wsClient := ws.NewTradeClient(cfg.WS, events) // chan<- ws.TradeEvent
hedge := service.NewHedgeService(/* ... */)

g, ctx := errgroup.WithContext(ctx)
g.Go(func() error { return wsClient.Run(ctx) })
g.Go(func() error { return hedge.Consume(ctx, events) }) // <-chan ws.TradeEvent
```

### 跨边界约束

- 接口、函数类型、事件类型的签名只能使用下层或 common 的类型，不能出现上层类型。
- 事件携带转换后的类型，不传交易所原始报文或 SDK 类型；参数多时封装成事件结构体。
- 回调运行在 IO/WS 读取 goroutine 上时，耗时处理必须交给其他 goroutine，不能阻塞读取循环。
- 有界 channel 必须明确满了的策略（阻塞、`select default` 丢弃、告警），并写在代码注释中。

## 数据、时间与一致性

- 价格、数量、金额统一使用 `shopspring/decimal`，禁止 `float64`/`float32`；同一模块不得混用两种表示。
- 除法必须指定精度和舍入方式；比较用 `Cmp`，不用 `==`。
- 金额从字符串或整数构造，不从浮点构造；溢出和精度丢失必须显式处理。
- 下游写入使用确定性幂等键，数据库加唯一约束；重复消息不能产生重复副作用。
- 消息流必须处理重复、乱序、gap、断线重连：收到 gap 时作废本地缓存并重拉快照，不能跳过继续用；重连后必须重订阅。
- service 及 common 中的纯函数不直接调用 `time.Now()`：`now` 通过参数注入。延迟和超时测量使用单调时钟，不用系统墙钟。

## 并发与性能

- 所有跨层调用的第一个参数是 `context.Context`，一路传递；下层不得自行创建 `context.Background()`/`context.TODO()`。
- 每个 goroutine 都必须能通过 `ctx` 退出；组装处用 `errgroup` 等待，不能启动后不管。
- 优先使用有界 channel 并定义背压策略。行情类可以丢弃并置 gap 标记；订单、回报、成交类不能丢，满了应拒绝受理、熔断或告警。
- 从 channel 读取必须判断关闭：`v, ok := <-ch`；`ok == false` 时退出循环，不能忙等零值。
- 读循环中禁止 `time.After`（每次分配定时器），用 `time.NewTimer` + `Reset`；退出时 `Stop`。
- 热路径禁止逐笔日志、`encoding/json`、`fmt.Sprintf`、反射、无界重试和不必要的堆分配。
- 锁内不做 IO、不调用回调；`sync.Mutex` 不得按值拷贝；不并发读写普通 map。
- 共享状态要么用 channel 传递，要么用锁保护，不要两者混用。

## 错误处理与事务

- 每层定义自己的错误（`var ErrXxx = errors.New(...)` 或错误类型）；第三方的 `sql.ErrNoRows`、`redis.Nil` 等在本层内转换，不直接暴露给上层。
- 包装错误用 `fmt.Errorf("...: %w", err)`，上层用 `errors.Is`/`errors.As` 判断。
- 不得忽略错误。确实需要忽略时用 `_ =` 并写注释说明原因。
- 失败不能只记日志后继续：必须明确重试（有上限和退避）、进入死信、熔断、返回错误或停机中的一种。
- 事务边界在 service；infra 只负责执行，不在 infra 开事务，不在 entry 开事务。
- 事务内不做远程调用（交易所、HTTP、MQ 同步发送）；数据库与消息的一致性用 outbox 或提交后发送，不做双写。
- 业务流程中不使用 `panic`；只有组装阶段遇到无法启动的配置错误才允许 `log.Fatal`。

## 测试与验证

- 领域计算、状态流转必须有表驱动单元测试，覆盖正常、边界、非法输入。
- service 测试使用 infra 的内存 fake，不连接真实数据库、Redis、交易所。
- infra 用集成测试或契约测试验证协议、序列化、重连；不要 mock 掉被测的转换逻辑。
- 测试必须确定性：不依赖当前时间、随机数、真实网络、执行顺序；时间通过参数或 fake clock 注入。
- 禁止 `time.Sleep` 等待异步结果，用 channel、条件变量或 `require.Eventually` 这类有上限的轮询。
- 禁止为了变绿修改断言、删除用例、加 `t.Skip`、放宽超时；测试只加严不放宽。
- 覆盖率不是目标，但核心业务分支必须有测试；新增分支不得无测试。

## AI 常见劣化模式

| 劣化写法 | 正确做法 |
|---|---|
| service 直接 import gorm/redis/kafka | 通过 infra 封装访问 |
| 把类型挪到 common 让两层都能用 | 类型归真正拥有它的层，边界做转换 |
| 单实现也建 `XxxService` 接口 + `NewXxxFactory` | 直接用具体类型，出现第二实现再抽象 |
| 加 `if useNew { ... } else { ... }` 兼容层 | 删旧路径，保持单一路径 |
| 改断言、加 `t.Skip`、放宽超时让测试变绿 | 修代码；测试只加严 |
| 只跑单个测试就宣称全部通过 | 跑完整检查并贴输出 |
| `if err != nil { return nil }` / `_ = err` | 包装或转换后返回，或明确失败策略 |
| 金额用 `float64` 或字符串拼 SQL | `decimal.Decimal` + 参数化查询 |
| WS 读取循环里做 DB 写入或 `time.Sleep` | 投递到有界 channel，异步处理 |
| `go func() {}` 无 ctx、无等待 | `errgroup` + ctx，组装处等待退出 |

## 检查方式与完成定义

### 检查

- `go vet ./...`
- `go test ./internal/archtest`：层间依赖、同层子树循环、外部包限制。
- `golangci-lint run`：至少启用 `errcheck`、`govet`、`staticcheck`、`depguard`、`bodyclose`。
- `go test -race ./...`
- 外部包限制用 depguard 的 `list-mode: strict` + 白名单，不要只写 deny 列表，否则未列出的 SDK 仍能进 service。
- archtest 的 `architecture` 与层级表一一对应；`engine_test.go` 是通用框架，修改时必须补自测。

### 完成定义（DoD）

- [ ] 代码在正确的层，依赖方向正确，archtest 通过。
- [ ] `go vet`、`golangci-lint`、`go test -race` 全部通过，并贴出真实输出。
- [ ] 新增业务分支有单元测试，覆盖失败路径。
- [ ] 没有 TODO、注释掉的代码、调试日志、未使用的参数。
- [ ] 错误处理、日志上下文、超时和取消符合本文件。
- [ ] 没有新增依赖、接口、工厂或兼容层；如确有必要，已在回复中说明理由。

## 已知的历史违规

> 记录现存不符合上述规则的代码，避免被模仿；修改相关代码时顺手修正。没有则保持为空。

- （无）
