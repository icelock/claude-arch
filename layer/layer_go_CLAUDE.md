# CLAUDE.md

## 应用架构：分层架构

本项目采用分层架构，依赖只能自上而下单向流动。层级和包名根据业务需要定义，但必须遵守下面的依赖规则。新增或修改代码前，先确认代码属于哪一层。

架构风格是**经典分层**：层是封闭的，不能跳层；common 是各层共用的公共层；`cmd` 是组装根。它不是 Clean 架构或六边形架构，service 直接 import infra 的具体包。未经用户同意，不要改成依赖倒置的写法（由 service 定义端口接口、infra 实现）。

### 写代码的强制要求

本节和下面的所有规则对人和 AI 同样适用，不是建议。

1. **先定位，再写代码**：动手前确定代码属于哪一层、放在哪个包。包路径必须落在「本项目的层级」表的某个模式下。落不进去时停下来问用户，确认后按「新增层」处理：同步更新层级表、依赖图和 `internal/archtest/arch_test.go` 中的 `architecture`，再写代码。
2. **不许通过改规则让检查通过**：archtest 失败，说明代码放错了层或依赖方向错了，应该改代码。下面这些做法一律禁止：
   - 扩大 `MayOnlyAccess`，或删减 `DeniedImports`、`StdlibOnly`；
   - 加 `Ignored`，或调整包路径模式；
   - 修改 `engine_test.go` 的检查逻辑来放行。

   确实需要调整规则时，先向用户说明原因，用户同意后再改，并同步更新本文件。
3. **不许绕过规则**，包括：
   - 用全局变量、`init()`、反射或 `any` 传递上层对象；
   - 只为了让上下层都能引用，就把类型挪进 common；
   - 把上层逻辑复制到下层。
4. **archtest 查不到的规则同样必须遵守**，写代码时逐条自查：
   - 跨层调用的第一个参数是 `context.Context`，下层不得自行创建 `context.Background()`/`context.TODO()`；
   - 依赖只通过构造函数注入，不用包级可变状态，也不在 `init()` 中装配；
   - entry 不直接使用数据库、Redis、MQ、WebSocket 等 SDK（`net/http` 服务端只用于 HTTP 入口）；
   - infra 不做业务判断；数据库模型、交易所原始报文、SDK 类型不离开 infra；
   - 回调和事件的签名中不出现上层类型；
   - 错误在本层转换，第三方错误不暴露给上层；
   - entry 子包之间、infra 子包之间不互相依赖。
5. **完成前必须验证**：运行 `go vet ./...`、`go test ./internal/archtest`、`go test -race ./...`，如实报告结果。只要 archtest 没通过，就不能说任务已完成。

### 层的角色

每一层都必须归入以下某种角色。可以按业务拆出更多层或子包，但不能出现不属于任何角色的层。

| 角色 | 职责 | 典型内容 |
|---|---|---|
| 入口 | 接收外部触发，转换后调用业务层 | HTTP/gRPC handler、WS 推送接收、MQ 消费、定时任务、CLI |
| 业务 | 业务规则与流程编排 | 计算、决策、状态流转、结果处理 |
| 基础设施 | 技术细节 | 交易所/第三方客户端、数据库、Redis、Kafka、文件、缓存加载 |
| 公共 | 各层共用的类型与工具，不依赖任何层 | 通用类型、枚举常量、错误基础定义、工具函数 |
| 组装 | 创建对象、注入依赖、启动和关闭 goroutine，只做接线 | `cmd/<app>/main.go`、`internal/bootstrap` |

### 本项目的层级

> 本表与 `internal/archtest/arch_test.go` 中的 `architecture` 一一对应，两边必须同时修改。在已有层下新增子包（如 `internal/infra/xxx`）不需要改。

| 层 | 包路径模式 | 角色 | 可以依赖的层 | 外部包限制（archtest 检查） |
|---|---|---|---|---|
| cmd | `cmd/...` | 组装 | 所有层 | 无 |
| entry | `internal/entry/...` | 入口 | service、common | 无（SDK 限制见「写代码的强制要求」第 4 条，需人工遵守） |
| service | `internal/service/...` | 业务 | infra、common | 禁止 `net/http`、`database/sql`、`github.com/gorilla/websocket`、`gorm.io`、`github.com/redis`、`github.com/IBM/sarama`、`github.com/twmb/franz-go`、`github.com/apolloconfig`、`github.com/aws` 及其子包 |
| infra | `internal/infra/...` | 基础设施 | common | 无 |
| common | `internal/common/...` | 公共 | 无（只有本层） | 只允许标准库，且禁止 `net`、`database`、`os`、`io/fs`、`io/ioutil` 及其子包 |

- 模块内每个包都必须属于上表某一层，`internal/archtest/...` 除外。不属于任何层时检查失败。
- 同层内的包可以互相依赖，但目录之间不能形成循环（archtest 逐层检查）。

各层已有子包的职责：

- entry：`httpapi`（HTTP）、`scheduler`（定时任务）等
- service：项目业务逻辑
- infra：基础设施，例如
  - `wsclient`：通用 WS 客户端，负责建连、心跳、重连，是基础设施公共库；
  - `config`：Apollo + AWS Secrets Manager，只在 cmd 中使用。
- common：`domain`（Account、Bot、Credentials、OrderUpdate、BBO 等共用类型）

```
cmd ──> entry ──> service ──> infra ──> common
 └────────┴──────────┴────────────────> common
```

### 依赖规则

- 上层可以依赖下层，**下层禁止 import 上层的任何包**。
  - 注意：Go 编译器只禁止循环 import，不禁止反向 import。例如 `infra/ws` import `service`，而 `service` 只 import 了 `infra/order`，没有形成环，编译仍然通过。因此必须用 `internal/archtest` 检查（见「检查方式」）。
- 依赖图中不能有环。新增层时，先确定它在图中的位置，再写代码。
  - 注意：编译器只禁止单个包之间的循环，不禁止子树之间的循环。例如 `common/x` import `infra/y`，而 `infra/z` import `common/w`，每个包都没有成环，编译通过，但 common 和 infra 已经互相依赖。`internal/archtest` 中的 `TestNoPackageCyclesAtEachLevel` 按目录层级逐层检查这种情况。
- 禁止通过包级变量绕过依赖规则，例如在下层用全局 `var` 保存上层对象再调用，或在 `init()` 中做依赖装配。所有依赖在组装处通过构造函数（`NewXxx(...)`）传入。
- 业务层不直接使用中间件或 SDK 的 API（`database/sql`、`gorm`、`go-redis`、`sarama`/`franz-go`、`gorilla/websocket`、`net/http` 客户端等），必须通过基础设施层封装的类型访问。
- 基础设施层只处理技术问题（协议、签名、序列化、连接、重连、持久化），不做业务判断。
- 入口层只做参数解析、校验和调用，不写业务逻辑，不直接调用基础设施层。
- 公共层不依赖网络、数据库、IO 相关的包，保持纯数据和纯函数。
- 数据库模型（gorm model、表结构 struct）只在基础设施层内使用，返回给上层前转换为业务需要的类型。
- 所有跨层调用的第一个参数传递 `context.Context`，不要在下层自行创建 `context.Background()` 切断取消和超时。

### 下层通知上层：下层定义接口、函数类型或 channel，上层实现/消费

当下层需要把结果交给业务层（下单回报、WS 推送、MQ 消息等）时，**不能直接调用上层**。按以下两种方式之一处理。

#### 方式一：回调接口或函数类型（单次结果、请求-回报型）

1. 接口定义在下层，与调用它的类型放在一起。这也符合 Go「接口由使用方定义」的惯例：调用回调的是下层，所以接口在下层。
2. 上层的类型实现该接口。Go 的接口是隐式实现的，上层不需要声明 `implements`。
3. 组装处把实现通过构造函数传给下层。

```go
// internal/infra/order：下层定义接口，签名只用 infra 或 common 中的类型
type ResultListener interface {
    OnOrderResult(ctx context.Context, ev ResultEvent)
}

type Client struct {
    listener ResultListener
}

func NewClient(listener ResultListener) *Client {
    return &Client{listener: listener}
}

// internal/service：上层实现下层接口（service -> infra，方向合法）
func (s *HedgeService) OnOrderResult(ctx context.Context, ev order.ResultEvent) { /* ... */ }

// cmd/<app>/main.go：组装
hedge := service.NewHedgeService(/* ... */)
client := order.NewClient(hedge)
```

只有一个方法时，也可以使用函数类型：`type ResultHandler func(ctx context.Context, ev ResultEvent)`。

#### 方式二：channel（持续推送、流式数据，优先使用）

1. 事件类型定义在下层或 common。
2. 下层持有只写 channel（`chan<- Event`），只负责发送。
3. 上层持有只读 channel（`<-chan Event`），在自己的 goroutine 中消费。
4. channel 在组装处创建，两端分别交给下层和上层。

```go
// cmd/<app>/main.go：组装
events := make(chan ws.TradeEvent, 10000)
wsClient := ws.NewTradeClient(cfg.WS, events)   // 参数类型 chan<- ws.TradeEvent
hedge := service.NewHedgeService(/* ... */)

g, ctx := errgroup.WithContext(ctx)
g.Go(func() error { return wsClient.Run(ctx) })
g.Go(func() error { return hedge.Consume(ctx, events) }) // 参数类型 <-chan ws.TradeEvent
```

#### 约束

- 接口、函数类型和事件类型的签名只能使用下层或 common 中的类型，不能出现上层类型。
- 事件携带的是转换后的类型，不能直接传递交易所原始报文或 SDK 类型。
- 参数较多时封装成事件结构体，不要堆叠参数。
- 回调运行在 IO/WS 读取 goroutine 上时，耗时处理必须交给其他 goroutine，不能阻塞读取循环。
- 带缓冲的 channel 需要明确满了以后的策略（阻塞、`select default` 丢弃、告警），并写在代码注释中。
- channel 由发送方关闭，接收方不关闭；所有 goroutine 都必须能通过 `ctx` 退出，组装处负责等待它们结束。
- 只有一个调用方且无需解耦时，上层直接调用下层即可，不要为了形式增加接口。
- 业务层为了单元测试，可以在自己的包中定义只包含所需方法的小接口，由 infra 的具体类型隐式满足。这不违反依赖规则。

### 错误处理

- 每层定义自己的错误（哨兵错误 `var ErrXxx = errors.New(...)` 或错误类型），下层的第三方错误（`sql.ErrNoRows`、`redis.Nil` 等）在本层内转换，不直接暴露给上层。
- 包装错误使用 `fmt.Errorf("...: %w", err)`，上层用 `errors.Is` / `errors.As` 判断。
- 不要忽略错误；确实需要忽略时，用 `_ =` 显式忽略并写注释说明原因。
- 业务流程中不使用 `panic`；只在组装阶段遇到无法启动的配置错误时允许 `panic` 或 `log.Fatal`。

### 新代码放哪里

- 新的外部系统接入（交易所、第三方 API、存储）→ 基础设施层，协议和报文转换在该层内完成。
- 新的业务规则或流程 → 业务层；计算逻辑尽量写成不依赖 IO 和 goroutine 的纯函数，并配表驱动单元测试。
- 新的触发方式（新接口、新定时任务、新消费者、新的 WS 推送接收）→ 入口层，只做转换和调用。
- 多个层都用到的类型 → common；只在一层内使用的类型留在该层，不要提前放到 common。
- 交易所报文结构与业务类型分开定义和存放。
- 不属于现有任何层的代码：先在「本项目的层级」表中新增层、明确角色和依赖，再写代码。

### 检查方式

- 层间依赖由 `internal/archtest` 在 `go test` 中检查，违反时测试失败，不需要额外安装工具。
- `arch_test.go` 中的 `TestLayerDependencies`：
  - `architecture` 声明式定义分层，类似 ArchUnit：先用包路径模式定义每层包含哪些包（`*` 匹配一段，结尾的 `/...` 匹配零或多段），再用 `MayOnlyAccess` 定义层之间的依赖；
  - 同层内的包总是可以互相依赖；`DeniedImports`、`StdlibOnly` 限制外部包；
  - `EnsureAllPackagesContained`（对应 ArchUnit 的 `ensureAllClassesAreContainedInArchitecture()`）要求模块内每个包都属于某一层或被 `Ignored`；
  - 一个包按 `Layers` 的顺序匹配，归入第一个匹配的层；
  - 检查范围包含 `_test.go`，跳过 `testdata`、`vendor`，以及 `.`、`_` 开头的目录。
- `arch_test.go` 中的 `TestNoPackageCyclesAtEachLevel`：把包路径依次截断到第 1 层到最深层，每层检查一次循环依赖（只看非测试文件），补上编译器管不到的子树之间的循环。报告会列出构成每条边的文件和 import。
- `engine_test.go` 是通用检查框架（`CheckLayers`、`CheckCycles`），只依赖标准库，与项目无关；`engine_self_test.go` 是框架自测。修改框架时必须补充自测用例，并保证自测通过。其它项目拷贝这两个文件，再写自己的 `arch_test.go` 就能使用。
- 单独运行：`go test ./internal/archtest`。
- 快速查看某一层的依赖：`go list -f '{{.ImportPath}}: {{join .Imports " "}}' ./internal/infra/... | grep internal/service`，结果应为空。
- 提交前运行 `go vet ./...` 和 `go test -race ./...`（含 `internal/archtest`）。

### 已知的历史违规

> 记录现存不符合上述规则的代码，避免被模仿；修改相关代码时顺手修正。没有则保持为空。

- （无）
