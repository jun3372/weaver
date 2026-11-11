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
	"fmt"
	"go/types"
	"slices"
	"strings"

	"golang.org/x/tools/go/packages"
)

const weaverPackagePath = "github.com/jun3372/weaver"

// typeSet holds type information needed by the code generator.
type typeSet struct {
	pkg            *packages.Package
	imported       []importPkg          // imported packages
	importedByPath map[string]importPkg // imported, indexed by path
	importedByName map[string]importPkg // imported, indexed by name
}

// importPkg is a package imported by the generated code.
type importPkg struct {
	path  string // e.g., "github.com/jun3372/weaver"
	pkg   string // e.g., "weaver", "context", "time"
	alias string // e.g., foo in `import foo "context"`
	local bool   // are we in this package?
}

// name returns the name by which the imported package should be referenced in
// the generated code. If the package is imported without an alias, like this:
//
//	import "context"
//
// then the name is the same as the package name (e.g., "context"). However, if
// a package is imported with an alias, then the name is the alias:
//
//	import thisIsAnAlias "context"
//
// If the package is local, an empty string is returned.
func (i importPkg) name() string {
	if i.local {
		return ""
	} else if i.alias != "" {
		return i.alias
	}
	return i.pkg
}

// qualify returns the provided member of the package, qualified with the
// package name. For example, the "Context" type inside the "context" package
// is qualified "context.Context". The "Now" function inside the "time" package
// is qualified "time.Now". Note that the package name is not prefixed when
// qualifying members of the local package.
func (i importPkg) qualify(member string) string {
	if i.local {
		return member
	}
	return fmt.Sprintf("%s.%s", i.name(), member)
}

// newTypeSet returns the container for types found in pkg.
func newTypeSet(pkg *packages.Package) *typeSet {
	return &typeSet{
		pkg:            pkg,
		imported:       []importPkg{},
		importedByPath: map[string]importPkg{},
		importedByName: map[string]importPkg{},
	}
}

// importPackage imports a package with the provided path and package name. The
// package is imported with an alias if there is a package name clash.
func (tset *typeSet) importPackage(path, pkg string) importPkg {
	newImportPkg := func(path, pkg, alias string, local bool) importPkg {
		i := importPkg{path: path, pkg: pkg, alias: alias, local: local}
		tset.imported = append(tset.imported, i)
		tset.importedByPath[i.path] = i
		tset.importedByName[i.name()] = i
		return i
	}

	if imp, ok := tset.importedByPath[path]; ok {
		// This package has already been imported.
		return imp
	}

	if _, ok := tset.importedByName[pkg]; !ok {
		// Import the package without an alias.
		return newImportPkg(path, pkg, "", path == tset.pkg.PkgPath)
	}

	// Find an unused alias.
	var alias string
	counter := 1
	for {
		alias = fmt.Sprintf("%s%d", pkg, counter)
		if _, ok := tset.importedByName[alias]; !ok {
			break
		}
		counter++
	}
	return newImportPkg(path, pkg, alias, path == tset.pkg.PkgPath)
}

// imports returns the list of packages to import in generated code.
func (tset *typeSet) imports() []importPkg {
	slices.SortFunc(tset.imported, func(a, b importPkg) int {
		return strings.Compare(a.path, b.path)
	})
	return tset.imported
}

// genTypeString returns the string representation of t as to be printed
// in the generated code, updating import definitions to account for the
// returned type string.
//
// Since this call has side-effects (i.e., updating import definitions), it
// should only be called when the returned type string is written into
// the generated file; otherwise, the generated code may end up with spurious
// imports.
func (tset *typeSet) genTypeString(t types.Type) string {
	// qualifier is passed to types.TypeString(Type, Qualifier) to determine
	// how packages are printed when pretty printing types. For this qualifier,
	// types in the root package are printed without their package name, while
	// types outside the root package are printed with their package name. For
	// example, if we're in root package foo, then the type foo.Bar is printed
	// as Bar, while the type io.Reader is printed as io.Reader. See [1] for
	// more information on qualifiers and pretty printing types.
	//
	// [1]: https://github.com/golang/example/tree/master/gotypes#formatting-support
	qualifier := func(pkg *types.Package) string {
		if pkg == tset.pkg.Types {
			return ""
		}
		return tset.importPackage(pkg.Path(), pkg.Name()).name()
	}
	return types.TypeString(t, qualifier)
}

// isWeaverType returns true iff t is a named type from the weaver package with
// the specified name and n type arguments.
func isWeaverType(t types.Type, name string, n int) bool {
	named, ok := t.(*types.Named)
	return ok &&
		named.Obj().Pkg() != nil &&
		named.Obj().Pkg().Path() == weaverPackagePath &&
		named.Obj().Name() == name &&
		named.TypeArgs().Len() == n
}

func isWeaverImplements(t types.Type) bool {
	return isWeaverType(t, "Implements", 1)
}

func isWeaverRef(t types.Type) bool {
	return isWeaverType(t, "Ref", 1)
}

func isWeaverListener(t types.Type) bool {
	return isWeaverType(t, "Listener", 0)
}

func isWeaverMain(t types.Type) bool {
	return isWeaverType(t, "Main", 0)
}

func isWeaverWithRouter(t types.Type) bool {
	return isWeaverType(t, "WithRouter", 1)
}

func isWeaverAutoMarshal(t types.Type) bool {
	return isWeaverType(t, "AutoMarshal", 0)
}

func isWeaverNotRetriable(t types.Type) bool {
	return isWeaverType(t, "NotRetriable", 0)
}

// isPrimitiveRouter returns whether the provided type is a valid primitive
// router type (i.e. an integer, a float, or a string).
func isPrimitiveRouter(t types.Type) bool {
	b, ok := t.(*types.Basic)
	if !ok {
		return false
	}
	switch b.Kind() {
	case types.Int, types.Int8, types.Int16, types.Int32, types.Int64,
		types.Uint, types.Uint8, types.Uint16, types.Uint32, types.Uint64,
		types.Float32, types.Float64, types.String:
		return true
	default:
		return false
	}
}

// isValidRouterType returns whether the provided type is a valid router type.
// A router type can be one of the following: an integer (signed or unsigned),
// a float, or a string. Alternatively, it can be a struct that may optioanly
// embed the weaver.AutoMarshal struct and rest of the fields must be either
// integers, floats, or strings.
func isValidRouterType(t types.Type) bool {
	t = t.Underlying()
	if isPrimitiveRouter(t) {
		return true
	}
	s, ok := t.(*types.Struct)
	if !ok {
		return false
	}
	for field := range s.Fields() {
		ft := field.Type()
		if !isPrimitiveRouter(ft) && !isWeaverAutoMarshal(ft) {
			return false
		}
	}
	return true
}
