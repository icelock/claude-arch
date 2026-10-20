// Package archtest 校验洋葱架构的依赖规则，违反时 go test 失败。由人工定期运行：go test ./internal/archtest
//
//	arch_test.go         本项目的分层定义（architecture）和检查入口，与 CLAUDE.md「本项目的层级」表一一对应
//	engine_test.go       通用检查框架，从 layer/archtest_go 拷贝
//	engine_self_test.go  框架自身的单元测试，从 layer/archtest_go 拷贝
//
// 内层的 DeniedImports 是 deny 列表：引入新的框架、驱动或客户端库时，必须同时加进 innerDenied。
package archtest

import "testing"

// 层按顺序匹配，包归入第一个匹配的层。每个出站适配器单独成层且互不在 MayOnlyAccess 中，以此保证彼此不依赖。
// DeniedImports 命中该包及其子包；不要写 "net"，否则会连纯计算的 net/url、net/netip 一起禁掉。
var innerDenied = []string{"net/http", "net/rpc", "net/smtp", "database/sql", "os", "io/fs", "encoding/json",
	"gorm.io", "github.com/redis", "github.com/IBM/sarama", "github.com/twmb/franz-go", "github.com/gorilla/websocket"}

var architecture = Architecture{
	Layers: []Layer{
		{Name: "cmd", Packages: []string{"cmd/..."}, MayAccessAll: true},
		{Name: "adapter-in", Packages: []string{"internal/adapter/in/..."}, MayOnlyAccess: []string{"application", "domain"}},
		{Name: "out-payment", Packages: []string{"internal/adapter/out/payment/..."}, MayOnlyAccess: []string{"application", "domain"}},
		{Name: "out-persistence", Packages: []string{"internal/adapter/out/persistence/..."}, MayOnlyAccess: []string{"application", "domain"}},
		{Name: "out-mq", Packages: []string{"internal/adapter/out/mq/..."}, MayOnlyAccess: []string{"application", "domain"}},
		{Name: "application", Packages: []string{"internal/application/..."}, MayOnlyAccess: []string{"domain"}, DeniedImports: innerDenied},
		{Name: "domain", Packages: []string{"internal/domain/..."}, DeniedImports: innerDenied},
	},
	Ignored:                    []string{"internal/archtest/..."},
	EnsureAllPackagesContained: true,
}

func TestLayerDependencies(t *testing.T)          { architecture.CheckLayers(t) }
func TestNoPackageCyclesAtEachLevel(t *testing.T) { architecture.CheckCycles(t) }
