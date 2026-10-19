package archtest

import (
	"bufio"
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// 分层架构检查框架，参考 ArchUnit，只依赖标准库，与具体项目无关。
//
// 提供两项检查：
//
//	CheckLayers  类似 layeredArchitecture()：先定义每层包含哪些包，再定义层与层之间允许的依赖
//	CheckCycles  类似 slices().should().beFreeOfCycles()：按目录层级逐层检查包循环依赖
//
// 移植到其它项目：把 engine_test.go、engine_self_test.go 拷到目标项目的任意目录（包名 archtest，
// 或把两个文件的 package 改成目录名），再新建一个 _test.go 定义 Architecture 并调用检查，例如：
//
//	var architecture = archtest.Architecture{...} // 同包内直接写 Architecture{...}
//
//	func TestLayerDependencies(t *testing.T) { architecture.CheckLayers(t) }
//	func TestNoPackageCycles(t *testing.T)   { architecture.CheckCycles(t) }
//
// 包路径模式相对模块根（go.mod 所在目录）：
//
//	internal/service/...    internal/service 及其所有子包（结尾的 /... 匹配零或多段）
//	internal/infra/*/...    internal/infra 下的每个子包及其子包（* 匹配一段），不含 internal/infra 自身
//	internal/infra/wsclient 精确匹配

// Architecture 描述分层：包属于哪一层、层之间允许哪些依赖。
type Architecture struct {
	// Layers 按顺序匹配，包归入第一个匹配的层，因此更具体的模式要放在前面。
	Layers []Layer
	// Ignored 不参与检查的包（模式），如 archtest 自身。
	Ignored []string
	// EnsureAllPackagesContained 对应 ArchUnit 的 ensureAllClassesAreContainedInArchitecture()：
	// 模块内每个含 .go 文件（含只有 _test.go）的包都必须属于某一层或被 Ignored，否则检查失败。
	// 关闭时，不属于任何层的包既不受检查，也可以被任何层依赖。
	EnsureAllPackagesContained bool
}

// Layer 是一层的定义。同层内的包总是可以互相依赖。
type Layer struct {
	Name     string
	Packages []string // 属于该层的包路径模式

	MayOnlyAccess []string // 允许依赖的其它层，空表示不能依赖其它任何层
	MayAccessAll  bool     // 可以依赖所有层（组装层）

	StdlibOnly    bool     // 只能使用标准库，禁止第三方库
	DeniedImports []string // 禁止引用的外部包，等于或是其子包即命中
}

// CheckLayers 校验分层定义本身，再检查模块内所有 .go 文件（含 _test.go）的 import 是否符合分层定义。
func (a Architecture) CheckLayers(t *testing.T) {
	t.Helper()
	a.mustValidate(t)
	root, module := moduleRoot(t)
	for _, v := range a.check(module, scanModule(t, root)) {
		t.Error(v)
	}
}

// CheckCycles 按目录层级逐层检查模块内包之间的循环依赖，每个深度一个子测试。
//
// Go 编译器只禁止单个包之间的循环 import，不管子树之间的循环：例如 internal/entry/x -> internal/service/y
// 且 internal/service/z -> internal/entry/w，编译能通过，但 entry 和 service 两个子树已经互相依赖。
// 因此每一层把包路径截断到该深度作为一个节点（比该深度浅的包保留原路径），在合并后的依赖图中找环。
// 深度 1 检查顶层目录之间，最深一层等同于逐包检查，深度上限按项目中最深的包自动计算。
// 只看非 _test.go 文件：测试代码不进入正式构建，不会产生实际的循环；Ignored 的包不参与。
// 不要求包属于某一层，没有定义 Layers 时也可以单独使用。
func (a Architecture) CheckCycles(t *testing.T) {
	t.Helper()
	a.mustValidate(t)
	root, module := moduleRoot(t)
	g := a.importGraph(module, scanModule(t, root))
	for depth := 1; depth <= g.maxDepth(); depth++ {
		t.Run(fmt.Sprintf("depth=%d", depth), func(t *testing.T) {
			for _, cycle := range g.truncate(depth).cycles() {
				t.Errorf("包循环依赖：%s", cycle)
			}
		})
	}
}

func (a Architecture) mustValidate(t *testing.T) {
	t.Helper()
	if errs := a.validate(); len(errs) > 0 {
		for _, err := range errs {
			t.Errorf("分层定义有误：%v", err)
		}
		t.FailNow()
	}
}

// goFile 是一个 Go 源文件的 import 信息，路径都相对模块根。
type goFile struct {
	path    string // 如 internal/infra/binance/manager.go
	pkg     string // 所在目录，如 internal/infra/binance；模块根为 "."
	test    bool   // 是否为 _test.go
	imports []string
}

// validate 检查分层定义本身是否有误。
func (a Architecture) validate() []error {
	var errs []error
	names := map[string]bool{}
	for _, l := range a.Layers {
		if l.Name == "" || names[l.Name] {
			errs = append(errs, fmt.Errorf("层名 %q 为空或重复", l.Name))
		}
		names[l.Name] = true
	}
	for _, l := range a.Layers {
		if len(l.Packages) == 0 {
			errs = append(errs, fmt.Errorf("%s 层没有定义包", l.Name))
		}
		for _, p := range l.Packages {
			if err := validatePattern(p); err != nil {
				errs = append(errs, fmt.Errorf("%s 层：%w", l.Name, err))
			}
		}
		if l.MayAccessAll && len(l.MayOnlyAccess) > 0 {
			errs = append(errs, fmt.Errorf("%s 层同时设置了 MayAccessAll 和 MayOnlyAccess", l.Name))
		}
		for _, n := range l.MayOnlyAccess {
			if !names[n] || n == l.Name {
				errs = append(errs, fmt.Errorf("%s 层的 MayOnlyAccess 中 %q 不是其它已定义的层", l.Name, n))
			}
		}
	}
	for _, p := range a.Ignored {
		if err := validatePattern(p); err != nil {
			errs = append(errs, fmt.Errorf("Ignored：%w", err))
		}
	}
	return errs
}

func validatePattern(pattern string) error {
	segs := strings.Split(pattern, "/")
	for i, s := range segs {
		if s == "" || (s == "..." && i != len(segs)-1) {
			return fmt.Errorf("包模式 %q 无效：不能有空段，... 只能出现在结尾", pattern)
		}
	}
	return nil
}

// matchPattern 判断 pkg 是否匹配 pattern。
func matchPattern(pattern, pkg string) bool {
	ps, ks := strings.Split(pattern, "/"), strings.Split(pkg, "/")
	for i, p := range ps {
		if p == "..." {
			break
		}
		if i >= len(ks) || (p != "*" && p != ks[i]) {
			return false
		}
	}
	return ps[len(ps)-1] == "..." || len(ps) == len(ks)
}

func (a Architecture) ignored(pkg string) bool {
	return slices.ContainsFunc(a.Ignored, func(p string) bool {
		return matchPattern(p, pkg)
	})
}

// layerOf 返回 pkg 所属的层。
func (a Architecture) layerOf(pkg string) (*Layer, bool) {
	for i := range a.Layers {
		for _, p := range a.Layers[i].Packages {
			if matchPattern(p, pkg) {
				return &a.Layers[i], true
			}
		}
	}
	return nil, false
}

// uncontained 返回既不属于任何层、也没有被 Ignored 的包，按包路径排序，每个包附一个示例文件。
func (a Architecture) uncontained(files []goFile) []string {
	example := map[string]string{}
	for _, f := range files {
		if _, seen := example[f.pkg]; seen || a.ignored(f.pkg) {
			continue
		}
		if _, ok := a.layerOf(f.pkg); !ok {
			example[f.pkg] = f.path
		}
	}
	var violations []string
	for _, pkg := range slices.Sorted(maps.Keys(example)) {
		violations = append(violations, fmt.Sprintf("包 %s（如 %s）不属于任何层：先在分层定义中把它归入某一层，或加入 Ignored", pkg, example[pkg]))
	}
	return violations
}

// check 按分层定义检查所有文件的 import，返回违规描述。
func (a Architecture) check(module string, files []goFile) []string {
	var violations []string
	if a.EnsureAllPackagesContained {
		violations = a.uncontained(files)
	}
	for _, f := range files {
		if a.ignored(f.pkg) {
			continue
		}
		from, ok := a.layerOf(f.pkg)
		if !ok {
			continue // 不在架构内，由 uncontained 报告
		}
		for _, imp := range f.imports {
			target, inModule := modulePackage(module, imp)
			if !inModule {
				switch {
				case isDenied(imp, from.DeniedImports):
					violations = append(violations, fmt.Sprintf("%s 引用了 %s：%s 层禁止使用该包", f.path, imp, from.Name))
				case from.StdlibOnly && !isStdlib(imp):
					violations = append(violations, fmt.Sprintf("%s 引用了第三方库 %s：%s 层只能使用标准库", f.path, imp, from.Name))
				}
				continue
			}
			if a.ignored(target) {
				continue
			}
			to, ok := a.layerOf(target)
			switch {
			case !ok:
				// 目标不在架构内，不限制；开启 EnsureAllPackagesContained 时由 uncontained 报告
			case to == from, from.MayAccessAll || slices.Contains(from.MayOnlyAccess, to.Name):
			default:
				violations = append(violations, fmt.Sprintf("%s 引用了 %s：%s 层只能依赖%s，不能依赖 %s 层",
					f.path, imp, from.Name, accessDesc(from), to.Name))
			}
		}
	}
	return violations
}

// modulePackage 把 import 路径转换为相对模块根的包路径，不是本模块的包时返回 false。
func modulePackage(module, imp string) (string, bool) {
	if imp == module {
		return ".", true
	}
	return strings.CutPrefix(imp, module+"/")
}

func accessDesc(l *Layer) string {
	if len(l.MayOnlyAccess) == 0 {
		return "本层"
	}
	return "本层和 " + strings.Join(l.MayOnlyAccess, "、")
}

// isDenied：p 等于 denied 中某项，或是其子包。
func isDenied(p string, denied []string) bool {
	return slices.ContainsFunc(denied, func(d string) bool {
		return p == d || strings.HasPrefix(p, d+"/")
	})
}

// isStdlib：标准库路径的第一段不含点。
func isStdlib(p string) bool { return !strings.Contains(strings.Split(p, "/")[0], ".") }

// ---- 包循环依赖 ----

// pkgGraph 项目内包的依赖图，包用相对模块根的路径表示，如 internal/infra/binance。
type pkgGraph struct {
	pkgs  map[string]bool
	edges map[string]map[string]string // from -> to -> 示例来源（文件 引用了 包），用于报错
}

func newPkgGraph() *pkgGraph {
	return &pkgGraph{pkgs: map[string]bool{}, edges: map[string]map[string]string{}}
}

func (g *pkgGraph) addEdge(from, to, example string) {
	if from == to {
		return
	}
	if g.edges[from] == nil {
		g.edges[from] = map[string]string{}
	}
	if _, ok := g.edges[from][to]; !ok {
		g.edges[from][to] = example
	}
}

// importGraph 用非测试文件的 import 构建模块内包的依赖图，跳过 Ignored 的包。
func (a Architecture) importGraph(module string, files []goFile) *pkgGraph {
	g := newPkgGraph()
	for _, f := range files {
		if f.test || a.ignored(f.pkg) {
			continue
		}
		g.pkgs[f.pkg] = true
		for _, imp := range f.imports {
			to, ok := modulePackage(module, imp)
			if !ok || a.ignored(to) {
				continue // 标准库、第三方库、Ignored 的包
			}
			g.pkgs[to] = true
			g.addEdge(f.pkg, to, f.path+" 引用了 "+imp)
		}
	}
	return g
}

// maxDepth 返回图中最深的包的段数。
func (g *pkgGraph) maxDepth() int {
	depth := 0
	for pkg := range g.pkgs {
		depth = max(depth, depthOf(pkg))
	}
	return depth
}

// truncate 把每个包截断到前 depth 段后合并成新图，比 depth 浅的包保留原路径。
func (g *pkgGraph) truncate(depth int) *pkgGraph {
	out := newPkgGraph()
	for pkg := range g.pkgs {
		out.pkgs[truncatePath(pkg, depth)] = true
	}
	for from, tos := range g.edges {
		for to, example := range tos {
			out.addEdge(truncatePath(from, depth), truncatePath(to, depth), example)
		}
	}
	return out
}

// cycles 返回图中每个强连通分量里的一条环，格式为 a -> b -> a，并附上每条边的示例来源。
func (g *pkgGraph) cycles() []string {
	var result []string
	for _, scc := range g.sccs() {
		inSCC := map[string]bool{}
		for _, p := range scc {
			inSCC[p] = true
		}
		path := g.cycleIn(scc[0], inSCC)
		var b strings.Builder
		b.WriteString(strings.Join(path, " -> "))
		for i := 0; i+1 < len(path); i++ {
			fmt.Fprintf(&b, "\n\t%s -> %s：%s", path[i], path[i+1], g.edges[path[i]][path[i+1]])
		}
		result = append(result, b.String())
	}
	return result
}

// sccs 用 Tarjan 算法找出节点数大于 1 的强连通分量，每个分量内的节点按字典序排列。
func (g *pkgGraph) sccs() [][]string {
	index, low := map[string]int{}, map[string]int{}
	onStack := map[string]bool{}
	var stack []string
	var result [][]string
	next := 0

	var visit func(v string)
	visit = func(v string) {
		index[v], low[v] = next, next
		next++
		stack = append(stack, v)
		onStack[v] = true
		for _, w := range g.sortedTargets(v) {
			if _, seen := index[w]; !seen {
				visit(w)
				low[v] = min(low[v], low[w])
			} else if onStack[w] {
				low[v] = min(low[v], index[w])
			}
		}
		if low[v] == index[v] {
			var scc []string
			for {
				w := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				onStack[w] = false
				scc = append(scc, w)
				if w == v {
					break
				}
			}
			if len(scc) > 1 {
				slices.Sort(scc)
				result = append(result, scc)
			}
		}
	}
	for _, v := range slices.Sorted(maps.Keys(g.pkgs)) {
		if _, seen := index[v]; !seen {
			visit(v)
		}
	}
	return result
}

// cycleIn 在强连通分量内用 BFS 找一条从 start 出发回到 start 的最短环。
func (g *pkgGraph) cycleIn(start string, inSCC map[string]bool) []string {
	prev := map[string]string{}
	queue := []string{start}
	for len(queue) > 0 {
		v := queue[0]
		queue = queue[1:]
		for _, w := range g.sortedTargets(v) {
			if !inSCC[w] {
				continue
			}
			if w == start {
				path := []string{start}
				for u := v; u != start; u = prev[u] {
					path = append(path, u)
				}
				slices.Reverse(path[1:])
				return append(path, start)
			}
			if _, seen := prev[w]; !seen {
				prev[w] = v
				queue = append(queue, w)
			}
		}
	}
	return []string{start} // 强连通分量内必然有环，不会走到这里
}

func (g *pkgGraph) sortedTargets(v string) []string {
	return slices.Sorted(maps.Keys(g.edges[v]))
}

// depthOf：包路径的段数，模块根目录下的包为 0。
func depthOf(pkg string) int {
	if pkg == "." {
		return 0
	}
	return strings.Count(pkg, "/") + 1
}

// truncatePath 保留包路径的前 depth 段。
func truncatePath(pkg string, depth int) string {
	if depthOf(pkg) <= depth {
		return pkg
	}
	return strings.Join(strings.Split(pkg, "/")[:depth], "/")
}

// ---- 源码扫描 ----

// scanModule 解析模块内所有 .go 文件的 import。
func scanModule(t *testing.T, root string) []goFile {
	t.Helper()
	var files []goFile
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			// 与 go 工具一致，跳过 . 和 _ 开头的目录、testdata、vendor
			if path != root && (strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") || name == "testdata" || name == "vendor") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(name, ".go") {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		gf := goFile{path: rel, pkg: filepath.ToSlash(filepath.Dir(rel)), test: strings.HasSuffix(name, "_test.go")}
		for _, imp := range f.Imports {
			p, _ := strconv.Unquote(imp.Path.Value)
			gf.imports = append(gf.imports, p)
		}
		files = append(files, gf)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// moduleRoot 向上查找 go.mod，返回模块根目录和模块名。
func moduleRoot(t *testing.T) (string, string) {
	t.Helper()
	dir, _ := os.Getwd()
	for {
		f, err := os.Open(filepath.Join(dir, "go.mod"))
		if err == nil {
			defer f.Close()
			sc := bufio.NewScanner(f)
			for sc.Scan() {
				if m, ok := strings.CutPrefix(strings.TrimSpace(sc.Text()), "module "); ok {
					return dir, strings.TrimSpace(m)
				}
			}
			t.Fatal("go.mod 中未找到 module")
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("未找到 go.mod")
		}
		dir = parent
	}
}
