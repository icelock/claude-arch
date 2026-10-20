# CLAUDE.md

## 架构声明

本项目采用**经典分层架构**：依赖只能自上而下单向流动，层是封闭的，不能跳层。`cmd` 是组装根。

```
cmd ──> entry ──> service ──> infra ──> common
 └────────┴──────────┴──────────┴─────> common
```

**组装层（`cmd`）可以依赖所有层**：它只做接线，是唯一同时知道所有具体实现的地方；任何层都不能反过来依赖组装层。

common 是共享内核，各层依赖 common 不算跳层；除此之外，entry 不能直接依赖 infra。

**这不是 Clean 架构、六边形架构或依赖倒置。** service 直接 import infra 的具体包，由 infra 提供能力。未经用户明确同意，不要：

- 在 service 定义端口接口、再由 infra 实现；
- 把 infra 类型抽象成接口层；
- 把分层改造成洋葱或整洁架构。

唯一例外是单元测试：service 可以在自己的包内定义 ≤3 个方法的窄接口，由 infra 的具体类型隐式满足，生产装配仍传具体类型。禁止系统性端口化、工厂、注册表。

## 架构工作方式

- 跨层改动时，先用几行说明计划：改哪些文件、每项落在哪一层、哪个包，是否涉及「必须停下来问」的事项，再动手。

## 架构硬性约束

本节只保留与架构相关的约束；通用约束见根 `CLAUDE.md`。

1. **先定位再写**：动手前确定代码属于哪一层、哪个包。包路径必须落在「本项目的层级」表中；落不进去就停下来问，不要自创目录。
2. **必须停下来问**：层级归属不明；新增层或模块。

## 分层与依赖

### 层的角色

每一层必须归入以下某种角色。可以按业务拆子包，但不存在不属于任何角色的层。

| 角色 | 职责 | 典型内容 |
|---|---|---|
| 组装 | 读取配置、创建对象、注入依赖、启停 goroutine，只做接线 | `cmd/<app>/main.go` |
| 入口 | 接收外部请求或定时触发，转换和校验后调用业务层 | HTTP/gRPC handler、MQ 消费者、定时任务、CLI 命令 |
| 业务 | 业务规则与流程编排 | 计算、决策、状态流转、结果处理、事务边界 |
| 基础设施 | 技术细节 | 第三方客户端、WebSocket 客户端、数据库仓储、Redis、Kafka、文件、缓存加载 |
| 公共 | 各层共用的类型与工具，不依赖任何层 | 通用类型、枚举、哨兵错误、纯函数工具 |

### 本项目的层级

> 下表为示例，按实际项目修改。在已有层下新增子包（如 `internal/infra/xxx`）不需要改。

| 层 | 包路径 | 可以依赖 | 外部包限制 |
|---|---|---|---|
| cmd | `cmd/...` | 所有层 | 无 |
| entry | `internal/entry/...`（如 `entry/http`、`entry/mq`、`entry/job`） | service、common | 不直接用数据库、Redis、MQ、WS SDK |
| service | `internal/service/...` | infra、common | 禁 `net/http`、`database/sql`、gorm、redis、kafka、websocket |
| infra | `internal/infra/...` | common | 无 |
| common | `internal/common/...` | 无 | 只允许标准库，且禁 `net`、`database`、`os`、`io/fs` |

- 模块内每个包必须属于上表某一层，新目录落不进去就停下来问。
- 同层内的包可以互相依赖，但目录之间不能形成循环。

### 依赖规则

- 上层可以依赖下层，**下层禁止 import 上层任何包**。Go 编译器只禁止循环 import，不禁止反向 import，写代码时自行保证。
- 依赖图不能有环。编译器只禁止单包成环，不禁止子树互依，按目录层级逐层看也不能有环。
- 禁止通过包级变量、`init()`、反射或 `any` 绕过依赖规则。所有依赖在 cmd 通过构造函数传入。
- 业务层不直接使用中间件或 SDK 的 API（`database/sql`、gorm、go-redis、sarama/franz-go、websocket、HTTP 客户端），必须通过 infra 封装的类型访问。
- 基础设施层只处理技术问题（协议、签名、序列化、连接、重连、持久化），不做业务判断。
- 入口层只做解析、校验和调用，不写业务逻辑，不直接调用 infra；定时任务放 `internal/entry/job`，只负责触发并调用 service。
- 公共层不依赖网络、数据库、IO 相关包，保持纯数据和纯函数。

### 新代码放哪里

- 新的外部系统接入 → infra，协议和报文转换在该层完成。
- 新的业务规则或流程 → service；计算尽量写成不依赖 IO 和 goroutine 的纯函数。
- 新的 HTTP 接口、消息消费或定时任务 → `internal/entry/...` 的对应子包，只做转换和调用。
- 新的启动、预热、生命周期管理 → `cmd/<app>`，只做接线和启停，不写业务逻辑。
- 多层共用的只读类型 → common；只在一层内使用的类型留在该层，不要提前上移。

## 跨边界数据与下层通知上层

### 类型归属

- 数据库模型、表结构 struct、外部系统原始报文、SDK 类型只在 infra 内使用；返回上层前转换为 service 需要的类型，转换在 infra 内完成。
- common 只放跨层共用的只读类型、枚举、哨兵错误、纯函数工具；禁止放带 `json`/`db`/`gorm` tag 的结构体。
- 只服务于某个 HTTP 接口的请求/响应结构体留在 entry，不要传进 service。

### 方式一：回调接口或函数类型（单次结果、请求-回报型）

1. 接口定义在下层，与调用它的类型放在一起；签名只用下层或 common 的类型。
2. 上层类型隐式实现该接口。
3. cmd 把实现通过构造函数传给下层。

```go
// internal/infra/payment：下层定义回调接口
type ResultListener interface {
    OnPaymentResult(ctx context.Context, ev ResultEvent)
}

type Client struct{ listener ResultListener }

func NewClient(listener ResultListener) *Client { return &Client{listener: listener} }

func (c *Client) Pay(ctx context.Context, orderNo string, amount decimal.Decimal) error { /* 发起支付；结果异步到达后调用 listener */ }

// internal/service：发起支付，只持有 *payment.Client
type OrderService struct{ payments *payment.Client }

// internal/service：单独的类型实现回调（service -> infra，方向合法），不持有 *payment.Client
type PaymentResultHandler struct{ orders *orderdb.Repo }

func (h *PaymentResultHandler) OnPaymentResult(ctx context.Context, ev payment.ResultEvent) { /* 更新订单状态 */ }

// cmd：组装，先建回调实现，再建客户端，最后建使用客户端的服务
handler := service.NewPaymentResultHandler(orderRepo)
client := payment.NewClient(handler)
orders := service.NewOrderService(client)
```

- **实现回调接口的类型不能再持有那个下层客户端**：上例若让 `OrderService` 同时实现 `ResultListener`，`NewClient` 需要先有 `OrderService`，`NewOrderService` 又需要先有 `Client`，cmd 里无法按顺序构造，只能先建半成品再 `SetListener` 回填，这正是下面禁止的「上层把自己注册给下层」。包依赖是合法的，但装配上会成环。
- 只有一个方法时可用函数类型。只有一个调用方且无需解耦时，上层直接调用下层，不要为形式加接口。

### 方式二：channel（持续推送、流式数据）

1. 事件类型定义在下层或 common。
2. 下层持有只写 channel（`chan<- Event`），只负责发送。
3. 上层持有只读 channel（`<-chan Event`），在自己的 goroutine 中消费。
4. channel 在 cmd 创建；**由拥有者（cmd）关闭**。接收方永不关闭；多发送者时用 `ctx` 取消，不关闭 channel。

```go
events := make(chan stream.Event, 1024)
streamClient := stream.NewClient(cfg.Stream, events) // chan<- stream.Event
consumer := service.NewEventConsumer(/* ... */)

g, ctx := errgroup.WithContext(ctx)
g.Go(func() error { return streamClient.Run(ctx) })
g.Go(func() error { return consumer.Consume(ctx, events) }) // <-chan stream.Event
```

### 跨边界约束

- 接口、函数类型、事件类型的签名只能使用下层或 common 的类型，不能出现上层类型。
- 事件携带转换后的类型，不传原始报文或 SDK 类型；参数多时封装成事件结构体。
- 不要提供 `SetListener` 之类的方法让上层在运行时把自己注册给下层，依赖一律在构造时传入。
- 回调运行在 IO 读取 goroutine 上时，耗时处理必须交给其他 goroutine，不能阻塞读取循环。

## AI 常见劣化模式

| 劣化写法 | 正确做法 |
|---|---|
| service 直接 import gorm/redis/kafka/`net/http` | 通过 infra 封装访问 |
| 把类型挪到 common 让两层都能用 | 类型归真正拥有它的层，边界做转换 |
| 单实现也建 `XxxService` 接口 + `NewXxxFactory`，或加 `if useNew` 兼容层 | 直接用具体类型，保持单一路径 |
| 同一个类型既实现下层回调接口，又持有该下层客户端，再用 `SetListener` 回填 | 回调拆到单独的类型，构造时传入 |
| 循环里逐条查库、列表不分页 | 批量查询，分页并设上限 |
| `if err != nil { return nil }`、只打日志后继续 | 包装后返回，或明确重试、死信、熔断 |
| `go func() {}` 无 ctx、无等待 | `errgroup` + ctx，cmd 等待退出 |
| `http.Get`、`http.DefaultClient`、不关 `resp.Body` | 设置了 `Timeout` 的 client + `NewRequestWithContext` + `defer Close` |
| 改断言、加 `t.Skip`、放宽超时让测试变绿 | 修代码；测试只加严 |
| 只跑单个测试就宣称全部通过 | 跑完整检查并贴输出 |

## 架构自查

- 代码在正确的层，依赖方向符合本文件；entry 不直接使用数据库、Redis、MQ、WS SDK。
