// Copyright 2022 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package generate

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"maps"

	"golang.org/x/tools/go/packages"

	"github.com/jun3372/weaver/internal/files"
)

// TODO(rgrandl): Modify the generator code to use only the types package. Right
// now we are doing code generation relying both on the types and ast packages,
// which can be confusing and also we might do unnecessary work.

const generatedCodeFile = "weaver_gen.go"

// Options controls the operation of Generate.
type Options struct {
	Warn      func(error) // If non-nil, use the specified function to report warnings
	BuildTags string
}

// Generate generates weaver code for the specified packages.
// The list of supplied packages are treated similarly to the arguments
// passed to "go build" (see "go help packages" for details).
func Generate(dir string, pkgs []string, opt Options) error {
	if opt.Warn == nil {
		opt.Warn = func(err error) { fmt.Fprintln(os.Stderr, err) }
	}
	fset := token.NewFileSet()
	cfg := &packages.Config{
		Mode:      packages.NeedName | packages.NeedSyntax | packages.NeedImports | packages.NeedTypes | packages.NeedTypesInfo,
		Dir:       dir,
		Fset:      fset,
		ParseFile: parseNonWeaverGenFile,
	}
	if len(opt.BuildTags) > 0 {
		cfg.BuildFlags = []string{"-tags", opt.BuildTags}
	}
	pkgList, err := packages.Load(cfg, pkgs...)
	if err != nil {
		return fmt.Errorf("packages.Load: %w", err)
	}

	var errs []error
	for _, pkg := range pkgList {
		if noGoFiles(pkg) {
			// 目录下没有 Go 文件(如 init 模板把 main.go 放在 ./cmd 后对项目根
			// 执行 generate),跳过即可,不应让整条命令失败。
			opt.Warn(fmt.Errorf("跳过 %s: 无 Go 文件", pkg.PkgPath))
			continue
		}
		g, err := newGenerator(opt, pkg, fset)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if err := g.generate(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// noGoFiles 报告 pkg 是否仅因目录下没有 Go 文件而加载失败。
func noGoFiles(pkg *packages.Package) bool {
	if len(pkg.Syntax) > 0 {
		return false
	}
	for _, err := range pkg.Errors {
		if !strings.Contains(err.Msg, "no Go files") {
			return false
		}
	}
	return true
}

// parseNonWeaverGenFile parses a Go file, except for weaver_gen.go files whose
// contents are ignored since those contents may reference types that no longer
// exist.
func parseNonWeaverGenFile(fset *token.FileSet, filename string, src []byte) (*ast.File, error) {
	if filepath.Base(filename) == generatedCodeFile {
		return parser.ParseFile(fset, filename, src, parser.PackageClauseOnly)
	}
	return parser.ParseFile(fset, filename, src, parser.ParseComments|parser.DeclarationErrors)
}

type generator struct {
	pkg        *packages.Package
	tset       *typeSet
	fileset    *token.FileSet
	components []*component
}

// errorf is like fmt.Errorf but prefixes the error with the provided position.
func errorf(fset *token.FileSet, pos token.Pos, format string, args ...any) error {
	// Rewrite the position's filename relative to the current directory. This
	// replaces long filenames like "/home/foo/weaver/weaver.go"
	// with much shorter filenames like "./weaver.go".
	position := fset.Position(pos)
	if cwd, err := filepath.Abs("."); err == nil {
		if filename, err := filepath.Rel(cwd, position.Filename); err == nil {
			position.Filename = filename
		}
	}

	prefix := position.String()
	return fmt.Errorf("%s: %w", prefix, fmt.Errorf(format, args...))
}

func newGenerator(opt Options, pkg *packages.Package, fset *token.FileSet) (*generator, error) {
	// Abort if there were any errors loading the package.
	var errs []error
	for _, err := range pkg.Errors {
		errs = append(errs, err)
	}
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}

	tset := newTypeSet(pkg)

	// Find and process all components.
	components := map[string]*component{}
	for _, file := range pkg.Syntax {
		filename := fset.Position(file.Package).Filename
		if filepath.Base(filename) == generatedCodeFile {
			// Ignore weaver_gen.go files.
			continue
		}

		fileComponents, err := findComponents(opt, pkg, file, tset)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		for _, c := range fileComponents {
			// Check for component duplicates, two components that embed the
			// same weaver.Implements[T].
			//
			// TODO(mwhittaker): This code relies on the fact that a component
			// interface and component implementation have to be in the same
			// package. If we lift this requirement, then this code will break.
			if existing, ok := components[c.fullIntfName()]; ok {
				errs = append(errs, errorf(pkg.Fset, c.impl.Obj().Pos(),
					"Duplicate implementation for component %s, other declaration: %v",
					c.fullIntfName(), fset.Position(existing.impl.Obj().Pos())))
				continue
			}
			components[c.fullIntfName()] = c
		}
	}

	// Find method attributes.
	for _, file := range pkg.Syntax {
		filename := fset.Position(file.Package).Filename
		if filepath.Base(filename) == generatedCodeFile {
			// Ignore weaver_gen.go files.
			continue
		}
		if err := findMethodAttributes(pkg, file, components); err != nil {
			errs = append(errs, err)
		}
	}

	if err := errors.Join(errs...); err != nil {
		return nil, err
	}

	return &generator{
		pkg:        pkg,
		tset:       tset,
		fileset:    fset,
		components: slices.Collect(maps.Values(components)),
	}, nil
}

// findComponents returns the components in the provided file. For example,
// findComponents will find and return the following component.
//
//	type something struct {
//	    weaver.Implements[SomeComponentType]
//	    ...
//	}
func findComponents(opt Options, pkg *packages.Package, f *ast.File, tset *typeSet) ([]*component, error) {
	var components []*component
	var errs []error
	for _, d := range f.Decls {
		gendecl, ok := d.(*ast.GenDecl)
		if !ok || gendecl.Tok != token.TYPE {
			continue
		}
		for _, spec := range gendecl.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			component, err := extractComponent(opt, pkg, f, tset, ts)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			if component != nil {
				components = append(components, component)
			}
		}
	}
	return components, errors.Join(errs...)
}

func findMethodAttributes(pkg *packages.Package, f *ast.File, components map[string]*component) error {
	// Look for declarations of the form:
	//	var _ weaver.NotRetriable = Component.Method
	var errs []error
	for _, decl := range f.Decls {
		gendecl, ok := decl.(*ast.GenDecl)
		if !ok || gendecl.Tok != token.VAR {
			continue
		}
		for _, spec := range gendecl.Specs {
			valspec, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			typeAndValue, ok := pkg.TypesInfo.Types[valspec.Type]
			if !ok {
				continue
			}
			t := typeAndValue.Type
			if !isWeaverNotRetriable(t) {
				continue
			}
			for _, val := range valspec.Values {
				// We allow non-blank vars for uniformity.
				comp, method, ok := findComponentMethod(pkg, components, val)
				if !ok {
					errs = append(errs, errorf(pkg.Fset, valspec.Pos(), "weaver.NonRetriable should only be assigned a value that identifies a method of a component implemented by this package"))
					continue
				}
				if comp.noretry == nil {
					comp.noretry = map[string]struct{}{}
				}
				comp.noretry[method] = struct{}{}
			}
		}
	}
	return errors.Join(errs...)
}

// findComponentMethod returns the component and method if val is an expression of
// the form C.M where C is a component listed in components and C has a method named M.
func findComponentMethod(pkg *packages.Package, components map[string]*component, val ast.Expr) (*component, string, bool) {
	sel, ok := val.(*ast.SelectorExpr)
	if !ok {
		return nil, "", false
	}
	ctypeval, ok := pkg.TypesInfo.Types[sel.X]
	if !ok {
		return nil, "", false
	}
	ctype, ok := ctypeval.Type.(*types.Named)
	if !ok {
		return nil, "", false
	}
	// components 以组件接口全名为键，而 val 形如 "Impl.Method" 中的 Impl 是
	// 实现类型，因此这里按实现类型匹配，而不是用实现类型全名查接口名键。
	// 同一实现类型挂多个组件接口时按接口名取最小者，保证结果确定。
	var c *component
	for _, comp := range components {
		if comp.impl != ctype {
			continue
		}
		if c == nil || comp.intfName() < c.intfName() {
			c = comp
		}
	}
	if c == nil {
		return nil, "", false
	}
	method := sel.Sel.Name
	for _, m := range c.methods() {
		if m.Name() == method {
			return c, method, true
		}
	}
	return nil, "", false
}

// extractComponent attempts to extract a component from the provided TypeSpec.
// It returns a nil component if the TypeSpec doesn't define a component.
// warnNonConformingSignatures 检查组件接口方法的签名形状：第一个参数必须是
// context.Context，最后一个返回值必须是 error。不符合时通过 opt.Warn 告警。
func warnNonConformingSignatures(opt Options, pkg *packages.Package, impl, intf *types.Named) {
	if opt.Warn == nil {
		return
	}

	iface, ok := intf.Underlying().(*types.Interface)
	if !ok {
		return
	}

	var ctxType types.Type
	for _, imp := range pkg.Imports {
		if imp.PkgPath == "context" {
			if v := imp.Types.Scope().Lookup("Context"); v != nil {
				ctxType = v.Type()
			}
			break
		}
	}
	errType := types.Universe.Lookup("error").Type()

	for m := range iface.Methods() {
		sig, ok := m.Type().(*types.Signature)
		if !ok {
			continue
		}

		var problems []string
		if ctxType == nil || sig.Params().Len() == 0 || !types.Identical(sig.Params().At(0).Type(), ctxType) {
			problems = append(problems, "第一个参数必须是 context.Context")
		}
		if sig.Results().Len() == 0 || !types.Identical(sig.Results().At(sig.Results().Len()-1).Type(), errType) {
			problems = append(problems, "最后一个返回值必须是 error")
		}
		if len(problems) > 0 {
			opt.Warn(errorf(pkg.Fset, m.Pos(),
				"component %s 的方法 %s 不符合组件方法约定: %s",
				formatType(pkg, impl), m.Name(), strings.Join(problems, "；")))
		}
	}
}

func extractComponent(opt Options, pkg *packages.Package, file *ast.File, tset *typeSet, spec *ast.TypeSpec) (*component, error) {
	// Check that the type spec is of the form `type t struct {...}`.
	s, ok := spec.Type.(*ast.StructType)
	if !ok {
		// This type declaration does not involve a struct. For example, it
		// might look like `type t int`. These non-struct type declarations
		// cannot be components.
		return nil, nil
	}
	def, ok := pkg.TypesInfo.Defs[spec.Name]
	if !ok {
		return nil, errorf(pkg.Fset, spec.Pos(), "name %v not found", spec.Name)
	}
	impl, ok := def.Type().(*types.Named)
	if !ok {
		// For type aliases like `type t = struct{}`, t has type *types.Struct
		// and not type *types.Named. We ignore these.
		return nil, nil
	}

	// Find any weaver.Implements[T] or weaver.WithRouter[T] embedded fields.
	var intf *types.Named   // The component interface type
	var router *types.Named // Router type (if any)
	var isMain bool         // Is intf weaver.Main?
	var refs []*types.Named // T for which weaver.Ref[T] exists in struct
	var listeners []string  // Names of all listener fields declared in struct
	for _, f := range s.Fields.List {
		typeAndValue, ok := pkg.TypesInfo.Types[f.Type]
		if !ok {
			panic(errorf(pkg.Fset, f.Pos(), "type %v not found", f.Type))
		}
		t := typeAndValue.Type

		if isWeaverRef(t) {
			// The field f has type weaver.Ref[T].
			arg := t.(*types.Named).TypeArgs().At(0)
			if isWeaverMain(arg) {
				return nil, errorf(pkg.Fset, f.Pos(),
					"components cannot contain a reference to weaver.Main")
			}
			named, ok := arg.(*types.Named)
			if !ok {
				return nil, errorf(pkg.Fset, f.Pos(),
					"weaver.Ref argument %s is not a named type.",
					formatType(pkg, arg))
			}
			refs = append(refs, named)
		} else if isWeaverListener(t) {
			lis, err := getListenerNamesFromStructField(pkg, f)
			if err != nil {
				return nil, err
			}
			listeners = append(listeners, lis...)
		}

		if len(f.Names) != 0 {
			// Ignore unembedded fields.
			//
			// TODO(mwhittaker): Warn the user about unembedded
			// weaver.Implements, weaver.WithConfig, or weaver.WithRouter?
			continue
		}

		switch {
		// The field f is an embedded weaver.Implements[T].
		case isWeaverImplements(t):
			// Check that T is a named interface type inside the package.
			arg := t.(*types.Named).TypeArgs().At(0)
			named, ok := arg.(*types.Named)
			if !ok {
				return nil, errorf(pkg.Fset, f.Pos(),
					"weaver.Implements argument %s is not a named type.",
					formatType(pkg, arg))
			}
			isMain = isWeaverMain(arg)
			if !isMain && named.Obj().Pkg() != pkg.Types {
				return nil, errorf(pkg.Fset, f.Pos(),
					"weaver.Implements argument %s is a type outside the current package. A component interface and implementation must be in the same package. If you can't move them into the same package, you can add `type %s %v` to the implementation's package and embed `weaver.Implements[%s]` instead of `weaver.Implements[%s]`.",
					formatType(pkg, named), named.Obj().Name(), formatType(pkg, named), named.Obj().Name(), formatType(pkg, named))
			}
			if _, ok := named.Underlying().(*types.Interface); !ok {
				return nil, errorf(pkg.Fset, f.Pos(),
					"weaver.Implements argument %s is not an interface.",
					formatType(pkg, named))
			}
			intf = named

		// The field f is an embedded weaver.WithRouter[T].
		case isWeaverWithRouter(t):
			// Check that T is a named type inside the package.
			arg := t.(*types.Named).TypeArgs().At(0)
			named, ok := arg.(*types.Named)
			if !ok {
				return nil, errorf(pkg.Fset, f.Pos(),
					"weaver.WithRouter argument %s is not a named type.",
					formatType(pkg, arg))
			}
			if named.Obj().Pkg() != pkg.Types {
				return nil, errorf(pkg.Fset, f.Pos(),
					"weaver.WithRouter argument %s is a type outside the current package.",
					formatType(pkg, named))
			}
			router = named
		}
	}

	if intf == nil {
		// TODO(mwhittaker): Warn the user if they embed weaver.WithRouter or
		// weaver.WithConfig but don't embed weaver.Implements.
		return nil, nil
	}

	// Check that that the component implementation implements the component
	// interface.
	if !types.Implements(types.NewPointer(impl), intf.Underlying().(*types.Interface)) {
		return nil, errorf(pkg.Fset, spec.Pos(),
			"type %s embeds weaver.Implements[%s] but does not implement interface %s.",
			formatType(pkg, impl), formatType(pkg, intf), formatType(pkg, intf))
	}

	// 校验组件方法签名形状（首参 context.Context、末返回 error）。生成产物
	// 不依赖该形状，因此不符合时仅告警。
	warnNonConformingSignatures(opt, pkg, impl, intf)

	// Disallow generic component implementations.
	if spec.TypeParams != nil && spec.TypeParams.NumFields() != 0 {
		return nil, errorf(pkg.Fset, spec.Pos(),
			"component implementation %s is generic. Component implements cannot be generic.",
			formatType(pkg, impl))
	}

	// Check that listener names are unique.
	seenLis := map[string]struct{}{}
	for _, lis := range listeners {
		if _, ok := seenLis[lis]; ok {
			return nil, errorf(pkg.Fset, spec.Pos(),
				"component implementation %s declares multiple listeners with name %s. Please disambiguate.", formatType(pkg, impl), lis)
		}
		seenLis[lis] = struct{}{}
	}

	comp := &component{
		intf:      intf,
		impl:      impl,
		router:    router,
		isMain:    isMain,
		refs:      refs,
		listeners: listeners,
	}

	// Find routing information if needed.
	if comp.router != nil {
		var err error
		comp.routingKey, comp.routedMethods, err = routerMethods(pkg, intf, router)
		if err != nil {
			return nil, errorf(pkg.Fset, spec.Pos(), "%w", err)
		}
	}

	return comp, nil
}

// getListenerNamesFromStructField extracts listener names from the given
// weaver.Listener field in the component implementation struct.
func getListenerNamesFromStructField(pkg *packages.Package, f *ast.Field) ([]string, error) {
	// Try to get the listener name from the struct tag.
	if f.Tag != nil {
		tag := reflect.StructTag(strings.TrimPrefix(
			strings.TrimSuffix(f.Tag.Value, "`"), "`"))
		if len(f.Names) > 1 {
			return nil, errorf(pkg.Fset, f.Pos(),
				"Tag %s repeated for multiple fields", tag)
		}
		if name, ok := tag.Lookup("weaver"); ok {
			if !token.IsIdentifier(name) {
				return nil, errorf(pkg.Fset, f.Pos(),
					"Listener tag %s is not a valid Go identifier", tag)
			}
			return []string{name}, nil
		}
		// fallthrough
	}

	// Get the listener name(s) from the struct field name(s).
	if f.Names == nil { // embedded field
		return []string{"Listener"}, nil
	}
	var ret []string
	for _, fname := range f.Names {
		ret = append(ret, fname.Name)
	}
	return ret, nil
}

// component represents a weaver component.
//
// A component is divided into an interface and implementation. For example, in
// the following code, Adder is the component interface, and adder is the
// component implementation. router is the router type.
//
//	type Adder interface{}
//	type adder struct {
//	    weaver.Implements[Adder]
//	    weaver.WithRouter[router]
//	}
//	type router struct{}
type component struct {
	intf          *types.Named        // component interface
	impl          *types.Named        // component implementation
	router        *types.Named        // router, or nil if there is no router
	routingKey    types.Type          // routing key, or nil if there is no router
	routedMethods map[string]bool     // the set of methods with a routing function
	isMain        bool                // intf is weaver.Main
	refs          []*types.Named      // List of T where a weaver.Ref[T] field is in impl struct
	listeners     []string            // Names of listener fields declared in impl struct
	noretry       map[string]struct{} // Methods that should not be retried
}

func fullName(t *types.Named) string {
	return path.Join(t.Obj().Pkg().Path(), t.Obj().Name())
}

// intfName returns the component interface name.
func (c *component) intfName() string {
	return c.intf.Obj().Name()
}

// implName returns the component implementation name.
func (c *component) implName() string {
	return c.impl.Obj().Name()
}

// fullIntfName returns the full package-prefixed component interface name.
func (c *component) fullIntfName() string {
	return fullName(c.intf)
}

// methods returns the component interface's methods.
func (c *component) methods() []*types.Func {
	underlying := c.intf.Underlying().(*types.Interface)
	methods := slices.Collect(underlying.Methods())

	// Sort the component's methods deterministically. This allows a developer
	// to re-order the interface methods without the generated code changing.
	slices.SortFunc(methods, func(a, b *types.Func) int {
		return strings.Compare(a.Name(), b.Name())
	})
	return methods
}

// routerMethods returns the routing key and the set of routed methods for comp.
//
// A developer can annotate a weaver component with a router, like this:
//
//	type foo struct {
//		weaver.Implements[Foo]
//		weaver.WithRouter[fooRouter]
//		...
//	}
//	func (*foo) A(context.Context) error {...}
//	func (*foo) B(context.Context, int) error {...}
//	func (*foo) C(context.Context, int) error {...}
//
//	type fooRouter struct{}
//	func (fooRouter) A(context.Context) int {...}
//	func (fooRouter) B(context.Context, int) int {...}
func routerMethods(pkg *packages.Package, intf, router *types.Named) (types.Type, map[string]bool, error) {
	underlying := intf.Underlying().(*types.Interface)
	componentMethods := map[string]*types.Signature{}
	for m := range underlying.Methods() {
		componentMethods[m.Name()] = m.Type().(*types.Signature)
	}

	// Verify that every router method corresponds to a component method. This
	// is so we avoid errors where one has been renamed but not the other.
	// Also check that they all have the same return type.
	var routingKey types.Type
	routedMethods := map[string]bool{}
	for i, n := 0, router.NumMethods(); i < n; i++ {
		m := router.Method(i)
		pos := m.Origin().Pos()
		componentMethod, ok := componentMethods[m.Name()]
		if !ok {
			return nil, nil, errorf(pkg.Fset, pos,
				"Routing function %q does not match any method of %q.",
				m.Name(), intf.Obj().Name())
		}
		mt := m.Type().(*types.Signature)

		// Router method args must match component method args.
		if !types.Identical(mt.Params(), componentMethod.Params()) {
			return nil, nil, errorf(pkg.Fset, pos,
				"Component %q method arguments %s do not match router method arguments %s",
				intf.Obj().Name(), formatType(pkg, componentMethod.Params()), formatType(pkg, mt.Params()))
		}

		// All router methods must have the same routable return type.
		if mt.Results().Len() != 1 {
			return nil, nil, errorf(pkg.Fset, pos,
				"Routing function %q must return exactly one value (it returns %d)",
				m.Name(), mt.Results().Len())
		}
		ret := mt.Results().At(0).Type()
		if i == 0 {
			if !isValidRouterType(ret) {
				return nil, nil, errorf(pkg.Fset, pos,
					"Router method %q has invalid routing key type %q. A routing key type should be an integer, float, string, or a struct with every field being an integer, float, or string.",
					m.Name(), formatType(pkg, ret))
			}
			routingKey = ret
		} else if !types.Identical(ret, routingKey) {
			return nil, nil, errorf(pkg.Fset, pos,
				"Return type of %q (%s) does not match previously seen routing key type (%s)",
				m.Name(), formatType(pkg, ret), formatType(pkg, routingKey))
		}
		routedMethods[m.Name()] = true
	}

	if routingKey == nil {
		return nil, nil, errorf(pkg.Fset, router.Obj().Pos(),
			"No routing methods found on declarated router type (%s) for component %q",
			router.Obj().Name(), intf.Obj().Name())
	}
	return routingKey, routedMethods, nil
}

type printFn func(format string, args ...any)

// TODO(mwhittaker): Have generate return an error.
func (g *generator) generate() error {
	if len(g.components) == 0 {
		// There's nothing to generate. Remove a stale weaver_gen.go left behind
		// by an earlier run (e.g., the package's last component was deleted);
		// it may reference types that no longer exist and break the build.
		if len(g.pkg.Syntax) > 0 {
			stale := filepath.Join(g.pkgDir(), generatedCodeFile)
			if err := os.Remove(stale); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
		return nil
	}

	// Process components in deterministic order.
	slices.SortFunc(g.components, func(a, b *component) int {
		return strings.Compare(a.intfName(), b.intfName())
	})

	// Generate the file body.
	var body bytes.Buffer
	{
		fn := func(format string, args ...any) {
			fmt.Fprintln(&body, fmt.Sprintf(format, args...))
		}
		g.generateRegisteredComponents(fn)
	}

	// Generate the file header. This should be done at the very end to ensure
	// that all types added to the body have been imported.
	var header bytes.Buffer
	{
		fn := func(format string, args ...any) {
			fmt.Fprintln(&header, fmt.Sprintf(format, args...))
		}
		g.generateImports(fn)
	}

	// 生成器按组件逐段输出，段间补空行会导致文件末尾多一个空行，不符合 gofmt。
	bodyTrimmed := bytes.TrimRight(body.Bytes(), "\n")
	body.Reset()
	body.Write(bodyTrimmed)
	body.WriteString("\n")

	// Create a generated file.
	filename := filepath.Join(g.pkgDir(), generatedCodeFile)
	dst := files.NewWriter(filename)
	defer dst.Cleanup()

	fmtAndWrite := func(buf bytes.Buffer) error {
		// Format the code.
		b := buf.Bytes()
		formatted, err := format.Source(b)
		if err != nil {
			return fmt.Errorf("format.Source: %w", err)
		}
		b = formatted

		// Write to dst.
		_, err = io.Copy(dst, bytes.NewReader(b))
		return err
	}

	if err := fmtAndWrite(header); err != nil {
		return err
	}
	if err := fmtAndWrite(body); err != nil {
		return err
	}
	return dst.Close()
}

// pkgDir returns the directory of the package.
func (g *generator) pkgDir() string {
	if len(g.pkg.Syntax) == 0 {
		panic(fmt.Errorf("package %v has no source files", g.pkg))
	}
	f := g.pkg.Syntax[0]
	fname := g.fileset.Position(f.Package).Filename
	return filepath.Dir(fname)
}

// componentRef returns the string to use to refer to the interface
// implemented by a component in generated code.
func (g *generator) componentRef(comp *component) string {
	if comp.isMain {
		return g.weaver().qualify("Main")
	}
	return comp.intfName() // We already checked that interface is in the same package.
}

// generateImports generates code to import all the dependencies.
func (g *generator) generateImports(p printFn) {
	p(`// Code generated by "weaver generate". DO NOT EDIT.`)
	p("//go:build !ignoreWeaverGen")
	p("")
	p("package %s", g.pkg.Name)
	p("")
	p(`import (`)
	for _, imp := range g.tset.imports() {
		switch {
		case imp.local:
			// Already inside desired package
		case imp.alias == "":
			p(`	%s`, strconv.Quote(imp.path))
		default:
			p(`	%s %s`, imp.alias, strconv.Quote(imp.path))
		}
	}
	p(`)`)
}

// generateRegisteredComponents 生成把组件注册到运行时注册表的代码。
//
// 产物形如:
//
//	func init() {
//		codegen.Register(codegen.Registration{
//			Name:      "github.com/foo/bar/Baz",
//			Interface: reflect.TypeFor[Baz](),
//			Impl:      reflect.TypeFor[baz](),
//		})
//	}
func (g *generator) generateRegisteredComponents(p printFn) {
	if len(g.components) == 0 {
		return
	}

	p(``)
	p(`func init() {`)
	for _, comp := range g.components {
		// 为接口类型取 reflect.Type:先取指针类型再解析其元素类型。
		// 见 https://pkg.go.dev/reflect#example-TypeOf
		reflect := g.tset.importPackage("reflect", "reflect")
		myName := comp.fullIntfName()

		p(`	%s(%s{`, g.codegen().qualify("Register"), g.codegen().qualify("Registration"))
		p(`		Name: %q,`, myName)
		p(`		Interface: %s[%s](),`, reflect.qualify("TypeFor"), g.componentRef(comp))
		p(`		Impl: %s[%s](),`, reflect.qualify("TypeFor"), comp.implName())
		p(`	})`)
	}
	p(`}`)

	// 断言实现类型确实实现了组件接口。组件接口变更后若忘记重新执行
	// weaver generate,这条断言会让编译失败,而不是把问题留到运行期。
	for _, comp := range g.components {
		p(``)
		p(`// Check that %s implements the %s interface.`, g.tset.genTypeString(comp.impl), g.tset.genTypeString(comp.intf))
		p(`var _ %s = (*%s)(nil)`, g.tset.genTypeString(comp.intf), g.tset.genTypeString(comp.impl))
	}
}

// weaver imports and returns the weaver package.
func (g *generator) weaver() importPkg {
	return g.tset.importPackage(weaverPackagePath, "weaver")
}

// codegen imports and returns the codegen package.
func (g *generator) codegen() importPkg {
	path := fmt.Sprintf("%s/runtime/codegen", weaverPackagePath)
	return g.tset.importPackage(path, "codegen")
}

// formatType pretty prints the provided type, encountered in the provided
// currentPackage.
func formatType(currentPackage *packages.Package, t types.Type) string {
	qualifier := func(pkg *types.Package) string {
		if pkg == currentPackage.Types {
			return ""
		}
		return pkg.Name()
	}
	return types.TypeString(t, qualifier)
}
