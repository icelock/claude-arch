// Package archtest 校验分层依赖规则，违反时 go test 失败。规则见 CLAUDE.md「分层架构」。
//
//	arch_test.go         本项目的分层定义（architecture）和检查入口，新增层或调整依赖只改这里
//	engine_test.go       通用分层检查框架（层间依赖、逐层循环依赖），与项目无关，可拷到其它项目使用
//	engine_self_test.go  框架自身的单元测试，随 engine_test.go 一起拷贝
package archtest

import "testing"

// architecture 是本项目的分层定义，与 CLAUDE.md「本项目的层级」保持一致。
//
// 新增的包落在已有模式下（如 internal/infra/xxx、internal/entry/xxx）时不用改这里；
// 新增层时加一个 Layer，写清它包含的包和允许依赖的层。
// 开启了 EnsureAllPackagesContained：模块内任何包不属于某一层（也没被 Ignored）时检查失败。
var architecture = Architecture{
	Layers: []Layer{
		{
			Name:         "cmd",
			Packages:     []string{"cmd/..."},
			MayAccessAll: true,
		},
		{
			Name:          "entry",
			Packages:      []string{"internal/entry/..."},
			MayOnlyAccess: []string{"service", "common"},
		},
		{
			Name:          "service",
			Packages:      []string{"internal/service/..."},
			MayOnlyAccess: []string{"infra", "common"},
			// 业务层不直接使用中间件/SDK，通过 infra 封装的类型访问
			DeniedImports: []string{"net/http", "database/sql", "github.com/gorilla/websocket", "gorm.io",
				"github.com/redis", "github.com/IBM/sarama", "github.com/twmb/franz-go",
				"github.com/apolloconfig", "github.com/aws"},
		},
		{
			Name:          "infra",
			Packages:      []string{"internal/infra/..."},
			MayOnlyAccess: []string{"common"},
		},
		{
			Name:          "common",
			Packages:      []string{"internal/common/..."},
			StdlibOnly:    true,
			DeniedImports: []string{"net", "database", "os", "io/fs", "io/ioutil"},
		},
	},
	Ignored:                    []string{"internal/archtest/..."},
	EnsureAllPackagesContained: true,
}

// TestLayerDependencies 检查各层之间的依赖是否符合 architecture。
func TestLayerDependencies(t *testing.T) { architecture.CheckLayers(t) }

// TestNoPackageCyclesAtEachLevel 按目录层级逐层检查包循环依赖，补上编译器管不到的子树之间的循环。
func TestNoPackageCyclesAtEachLevel(t *testing.T) { architecture.CheckCycles(t) }
