package archtest

import (
	"slices"
	"strings"
	"testing"
)

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		pattern, pkg string
		ok           bool
	}{
		{"internal/service/...", "internal/service", true},
		{"internal/service/...", "internal/service/sub/x", true},
		{"internal/service/...", "internal/servicex", false},
		{"internal/infra/*/...", "internal/infra", false},
		{"internal/infra/*/...", "internal/infra/binance", true},
		{"internal/infra/*/...", "internal/infra/binance/sub", true},
		{"internal/infra/*", "internal/infra/binance/sub", false},
		{"internal/infra/wsclient", "internal/infra/wsclient", true},
		{"internal/infra/wsclient", "internal/infra/wsclient/sub", false},
		{"cmd/...", ".", false},
	}
	for _, tt := range tests {
		if ok := matchPattern(tt.pattern, tt.pkg); ok != tt.ok {
			t.Errorf("matchPattern(%q, %q) = %v，期望 %v", tt.pattern, tt.pkg, ok, tt.ok)
		}
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name string
		arch Architecture
		want string // 期望错误中包含的内容，空表示无错误
	}{
		{"测试用定义", testArch, ""},
		{"重复层名", Architecture{Layers: []Layer{{Name: "a", Packages: []string{"a"}}, {Name: "a", Packages: []string{"b"}}}}, "重复"},
		{"没有包", Architecture{Layers: []Layer{{Name: "a"}}}, "没有定义包"},
		{"... 不在结尾", Architecture{Layers: []Layer{{Name: "a", Packages: []string{"a/.../b"}}}}, "无效"},
		{"依赖未定义的层", Architecture{Layers: []Layer{{Name: "a", Packages: []string{"a"}, MayOnlyAccess: []string{"b"}}}}, "不是其它已定义的层"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var msgs []string
			for _, err := range tt.arch.validate() {
				msgs = append(msgs, err.Error())
			}
			got := strings.Join(msgs, "\n")
			if (tt.want == "") != (got == "") || !strings.Contains(got, tt.want) {
				t.Errorf("validate() = %q，期望包含 %q", got, tt.want)
			}
		})
	}
}

// testArch 是引擎自测用的固定分层定义，与项目的 architecture 解耦，改项目配置不影响自测。
var testArch = Architecture{
	Layers: []Layer{
		{Name: "cmd", Packages: []string{"cmd/..."}, MayAccessAll: true},
		{Name: "entry", Packages: []string{"internal/entry/*/..."}, MayOnlyAccess: []string{"service", "common"}},
		{Name: "service", Packages: []string{"internal/service/..."}, MayOnlyAccess: []string{"infra", "common"}, DeniedImports: []string{"net/http"}},
		{Name: "infra", Packages: []string{"internal/infra/*/..."}, MayOnlyAccess: []string{"common"}},
		{Name: "common", Packages: []string{"internal/common/..."}, StdlibOnly: true, DeniedImports: []string{"os"}},
	},
	Ignored:                    []string{"internal/archtest/..."},
	EnsureAllPackagesContained: true,
}

// TestCheck 用 testArch 检查虚构的文件，确认每条规则都能命中、合法依赖不误报。
func TestCheck(t *testing.T) {
	tests := []struct {
		name    string
		file    goFile
		wantHit string // 期望违规中包含的内容，空表示合法
	}{
		{"cmd 依赖所有层", file("cmd/server", "m/internal/infra/binance", "m/internal/entry/httpapi"), ""},
		{"entry 依赖 service", file("internal/entry/httpapi", "m/internal/service", "net/http"), ""},
		{"entry 依赖 infra", file("internal/entry/httpapi", "m/internal/infra/binance"), "entry 层只能依赖"},
		{"entry 子包之间", file("internal/entry/httpapi", "m/internal/entry/scheduler"), ""},
		{"service 子包之间", file("internal/service/a", "m/internal/service/b"), ""},
		{"service 使用 SDK", file("internal/service", "net/http"), "禁止使用该包"},
		{"infra 子包之间", file("internal/infra/binance", "m/internal/infra/wsclient"), ""},
		{"infra 反向依赖 service", file("internal/infra/binance", "m/internal/service"), "不能依赖 service 层"},
		{"infra 依赖 common", file("internal/infra/wsclient", "m/internal/common/domain"), ""},
		{"common 使用 IO", file("internal/common/domain", "os"), "禁止使用该包"},
		{"common 使用第三方库", file("internal/common/domain", "github.com/shopspring/decimal"), "只能使用标准库"},
		{"忽略 archtest", file("internal/archtest", "m/internal/service"), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertHit(t, testArch.check("m", []goFile{tt.file}), tt.wantHit)
		})
	}
}

// TestEnsureAllPackagesContained 对应 ArchUnit 的 ensureAllClassesAreContainedInArchitecture()。
func TestEnsureAllPackagesContained(t *testing.T) {
	loose := testArch
	loose.EnsureAllPackagesContained = false

	tests := []struct {
		name    string
		arch    Architecture
		files   []goFile
		wantHit string
	}{
		{"顶层目录不在架构内", testArch, []goFile{file("pkg/util")}, "包 pkg/util（如 pkg/util/x.go）不属于任何层"},
		{"internal 下未定义的目录", testArch, []goFile{file("internal/foo")}, "包 internal/foo"},
		{"层目录自身不匹配 *", testArch, []goFile{file("internal/infra")}, "包 internal/infra"},
		{"模块根目录", testArch, []goFile{file(".")}, "包 ."},
		{"只有测试文件的包", testArch, []goFile{{path: "internal/foo/x_test.go", pkg: "internal/foo", test: true}}, "包 internal/foo"},
		{"被 Ignored 的包", testArch, []goFile{file("internal/archtest")}, ""},
		{"架构内的包", testArch, []goFile{file("internal/service"), file("cmd/server")}, ""},
		{"关闭后不报告", loose, []goFile{file("pkg/util", "m/internal/service")}, ""},
		{"关闭后可被任何层依赖", loose, []goFile{file("internal/common/domain", "m/pkg/util")}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertHit(t, tt.arch.check("m", tt.files), tt.wantHit)
		})
	}

	// 同一个包的多个文件只报告一次
	got := testArch.check("m", []goFile{file("pkg/util"), {path: "pkg/util/y.go", pkg: "pkg/util"}})
	if len(got) != 1 {
		t.Errorf("同一个包期望只报告一次，实际：%v", got)
	}
}

// assertHit：wantHit 为空时期望没有违规，否则期望某条违规包含 wantHit。
func assertHit(t *testing.T, got []string, wantHit string) {
	t.Helper()
	if wantHit == "" {
		if len(got) > 0 {
			t.Errorf("期望合法，实际违规：%v", got)
		}
		return
	}
	if !slices.ContainsFunc(got, func(v string) bool { return strings.Contains(v, wantHit) }) {
		t.Errorf("期望违规包含 %q，实际：%v", wantHit, got)
	}
}

func file(pkg string, imports ...string) goFile {
	return goFile{path: pkg + "/x.go", pkg: pkg, imports: imports}
}

// TestCycles 用虚构的文件验证逐层循环检查：截断、合并、找环和过滤规则。
func TestCycles(t *testing.T) {
	tests := []struct {
		name  string
		files []goFile
		depth int
		want  []string // 期望的环（报告的第一行），空表示无环
	}{
		{"无环", []goFile{file("a/x", "m/b/y"), file("b/y", "m/c")}, 1, nil},
		{"子树之间的环", []goFile{file("a/x", "m/b/y"), file("b/z", "m/a/w")}, 1, []string{"a -> b -> a"}},
		{"子树之间的环在更深层消失", []goFile{file("a/x", "m/b/y"), file("b/z", "m/a/w")}, 2, nil},
		{"同一子树内合并后不算环", []goFile{file("a/x", "m/a/y"), file("a/y", "m/a/x/sub")}, 1, nil},
		{"比深度浅的包保留原路径", []goFile{file("a", "m/b/c/d"), file("b/c/e", "m/a")}, 2, []string{"a -> b/c -> a"}},
		{"三个节点的环", []goFile{file("a/x", "m/b/x"), file("b/x", "m/c/x"), file("c/x", "m/a/x")}, 1, []string{"a -> b -> c -> a"}},
		{"两个独立的环", []goFile{file("a/x", "m/b/x"), file("b/x", "m/a/y"), file("c/x", "m/d/x"), file("d/x", "m/c/y")}, 1,
			[]string{"a -> b -> a", "c -> d -> c"}},
		{"模块根目录", []goFile{file(".", "m/a"), file("a", "m")}, 1, []string{". -> a -> ."}},
		{"忽略测试文件", []goFile{file("a/x", "m/b/y"), {path: "b/z/z_test.go", pkg: "b/z", test: true, imports: []string{"m/a/w"}}}, 1, nil},
		{"忽略 Ignored 的包", []goFile{file("internal/archtest", "m/internal/service"), file("internal/service", "m/internal/archtest")}, 1, nil},
		{"忽略外部包", []goFile{file("a", "github.com/x/b"), file("b", "github.com/x/a")}, 1, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []string
			for _, c := range testArch.importGraph("m", tt.files).truncate(tt.depth).cycles() {
				got = append(got, strings.SplitN(c, "\n", 2)[0])
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("depth=%d 的环 = %q，期望 %q", tt.depth, got, tt.want)
			}
		})
	}
}

// TestCycleReport 环的报告中附带每条边的示例来源，便于定位要改的 import。
func TestCycleReport(t *testing.T) {
	g := testArch.importGraph("m", []goFile{file("a/x", "m/b/y"), file("b/z", "m/a/w")})
	if got := g.maxDepth(); got != 2 {
		t.Errorf("maxDepth() = %d，期望 2", got)
	}
	want := "a -> b -> a\n\ta -> b：a/x/x.go 引用了 m/b/y\n\tb -> a：b/z/x.go 引用了 m/a/w"
	if got := g.truncate(1).cycles(); !slices.Equal(got, []string{want}) {
		t.Errorf("cycles() = %q，期望 %q", got, want)
	}
}
