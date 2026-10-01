package make

import (
	"fmt"
	"go/types"
	"sort"
	"strings"
)

// renderImpl 渲染接口实现结构体的 Go 源码,实现与接口位于同一包。
func renderImpl(pkg *types.Package, ifaceName string, iface *types.Interface) ([]byte, error) {
	g := &gen{
		pkg:       pkg,
		imports:   map[string]string{"github.com/jun3372/weaver": "weaver"},
		ifaceName: ifaceName,
		implName:  unexport(ifaceName),
	}
	return g.render(iface)
}

type gen struct {
	pkg       *types.Package
	imports   map[string]string // path -> alias
	ifaceName string
	implName  string
}

func (g *gen) render(iface *types.Interface) ([]byte, error) {
	var sb strings.Builder

	for i := 0; i < iface.NumMethods(); i++ {
		sb.WriteString(g.method(iface.Method(i)))
	}

	var imports strings.Builder
	paths := make([]string, 0, len(g.imports))
	for path := range g.imports {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		alias := g.imports[path]
		if alias != lastSegment(path) {
			_, _ = imports.WriteString("\t" + alias + " \"" + path + "\"\n")
		} else {
			_, _ = imports.WriteString("\t\"" + path + "\"\n")
		}
	}

	out := fmt.Sprintf(`package %s

import (
%s)

// %s 是 %s 的组件实现骨架,由 weaver make 生成。
type %s struct {
	weaver.Implements[%s]
}

%s`, g.pkg.Name(), imports.String(), g.implName, g.ifaceName, g.implName, g.ifaceName, sb.String())

	return []byte(out), nil
}

// method 渲染单个方法的 stub:参数缺名补 pN,返回值通过 var 声明零值后返回。
func (g *gen) method(m *types.Func) string {
	sig := m.Type().(*types.Signature)
	params := g.params(sig)
	results := g.results(sig)

	var body strings.Builder
	body.WriteString("\t// TODO: implement\n")
	if sig.Results().Len() > 0 {
		body.WriteString("\tvar (\n")
		for i := 0; i < sig.Results().Len(); i++ {
			_, _ = body.WriteString("\t\tr" + fmt.Sprint(i) + " " + g.typeString(sig.Results().At(i).Type()) + "\n")
		}
		body.WriteString("\t)\n")
		_, _ = body.WriteString("\treturn " + joinNames(sig.Results().Len(), "r") + "\n")
	}

	return fmt.Sprintf("func (i *%s) %s(%s) (%s) {\n%s}\n\n",
		g.implName, m.Name(), params, results, body.String())
}

func (g *gen) params(sig *types.Signature) string {
	tup := sig.Params()
	var parts []string
	for i := 0; i < tup.Len(); i++ {
		name := "p" + fmt.Sprint(i)
		t := tup.At(i).Type()
		if sig.Variadic() && i == tup.Len()-1 {
			parts = append(parts, name+" ..."+g.typeString(t.(*types.Slice).Elem()))
			continue
		}
		parts = append(parts, name+" "+g.typeString(t))
	}
	return strings.Join(parts, ", ")
}

func (g *gen) results(sig *types.Signature) string {
	tup := sig.Results()
	var parts []string
	for i := 0; i < tup.Len(); i++ {
		parts = append(parts, g.typeString(tup.At(i).Type()))
	}
	return strings.Join(parts, ", ")
}

// typeString 渲染类型:同包类型裸名引用,外部类型经 qualifier 注册 import。
func (g *gen) typeString(t types.Type) string {
	return types.TypeString(t, func(p *types.Package) string {
		if p == g.pkg {
			return ""
		}
		if alias, ok := g.imports[p.Path()]; ok {
			return alias
		}
		alias := uniqueAlias(p.Name(), g.imports)
		g.imports[p.Path()] = alias
		return alias
	})
}

func joinNames(n int, prefix string) string {
	var parts []string
	for i := 0; i < n; i++ {
		parts = append(parts, prefix+fmt.Sprint(i))
	}
	return strings.Join(parts, ", ")
}

func lastSegment(path string) string {
	if i := strings.LastIndexByte(path, '/'); i >= 0 {
		return path[i+1:]
	}
	return path
}

func uniqueAlias(name string, taken map[string]string) string {
	if _, ok := taken[name]; !ok {
		return name
	}
	for i := 2; ; i++ {
		alias := fmt.Sprintf("%s%d", name, i)
		if _, ok := taken[alias]; !ok {
			return alias
		}
	}
}
