package generate

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/types/typeutil"
)

const (
	generatedCodeFile = "weaver_gen.go"
)

var GenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Print the version number of Hugo",
	Long:  `All software has versions. This is Hugo's`,
	Run: func(cmd *cobra.Command, args []string) {
		slog.Info("generate", "cmd.Args", args)
		if len(args) < 1 {
			slog.Warn("Missing required argument")
			return
		}
		buildTags := "ignoreWeaverGen"
		var tags string
		cmd.Flags().StringVar(&tags, "tags", "", "Build tags to use when generating code")
		if tags != "" { // tags flag was specified=.
			buildTags = buildTags + "," + tags
		}

		generate(".", args, options{BuildTags: buildTags})
	},
}

type options struct {
	Warn      func(error) // If non-nil, use the specified function to report warnings
	BuildTags string
}
type generator struct {
	pkg            *packages.Package
	tset           *typeSet
	fileset        *token.FileSet
	components     []*component
	sizeFuncNeeded typeutil.Map // types that need a serviceweaver_size_* function
	generated      typeutil.Map // memo cache for generateEncDecMethodsFor
}

// typeSet holds type information needed by the code generator.
type typeSet struct {
	pkg            *packages.Package
	imported       []importPkg          // imported packages
	importedByPath map[string]importPkg // imported, indexed by path
	importedByName map[string]importPkg // imported, indexed by name

	automarshals          *typeutil.Map // types that implement AutoMarshal
	automarshalCandidates *typeutil.Map // types that declare themselves AutoMarshal

	// If checked[t] != nil, then checked[t] is the cached result of calling
	// check(pkg, t, string[]{}). Otherwise, if checked[t] == nil, then t has
	// not yet been checked for serializability. Read typeutil.Map's
	// documentation for why checked shouldn't be a map[types.Type]bool.
	checked typeutil.Map

	// If sizes[t] != nil, then sizes[t] == sizeOfType(t).
	sizes typeutil.Map

	// If measurable[t] != nil, then measurable[t] == isMeasurableType(t).
	measurable typeutil.Map
}

type importPkg struct {
	path  string // e.g., "github.com/ServiceWeaver/weaver"
	pkg   string // e.g., "weaver", "context", "time"
	alias string // e.g., foo in `import foo "context"`
	local bool   // are we in this package?
}

func generate(dir string, pkgs []string, opt options) error {
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

	slog.Info("loaded packages", "pkgs", pkgList)
	return nil
}
func parseNonWeaverGenFile(fset *token.FileSet, filename string, src []byte) (*ast.File, error) {
	if filepath.Base(filename) == generatedCodeFile {
		return parser.ParseFile(fset, filename, src, parser.PackageClauseOnly)
	}
	return parser.ParseFile(fset, filename, src, parser.ParseComments|parser.DeclarationErrors)
}
