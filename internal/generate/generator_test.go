package generate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 组件全部删除后重新运行 generate，残留的旧 weaver_gen.go 必须被删除，
// 否则其中对已不存在类型的引用会让包无法编译，且生成器无法自愈。
func TestGenerateRemovesStaleGeneratedFile(t *testing.T) {
	dir := filepath.Join("testdata", "stale-tmp")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	genFile := filepath.Join(dir, generatedCodeFile)
	stale := `package staleremove

import (
	"reflect"

	"github.com/jun3372/weaver/runtime/codegen"
)

type staleIntf interface{ Ping() }

type staleImpl struct{}

func init() {
	codegen.Register(codegen.Registration{
		Name:      "example/stale",
		Interface: reflect.TypeOf((*staleIntf)(nil)).Elem(),
		Impl:      reflect.TypeOf(staleImpl{}),
	})
}
`
	if err := os.WriteFile(genFile, []byte(stale), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "doc.go"), []byte("package staleremove\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Generate(".", []string{"./" + filepath.ToSlash(dir)}, Options{}); err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if _, err := os.Stat(genFile); !os.IsNotExist(err) {
		t.Fatal("无组件包的残留 weaver_gen.go 应被删除")
	}
}

// 含组件的包不受影响：仍正常生成 weaver_gen.go。
func TestGenerateKeepsGeneratedFileForComponentPackage(t *testing.T) {
	dir := filepath.Join("testdata", "comp-tmp")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	src := `package compkeep

import (
	"context"

	"github.com/jun3372/weaver"
)

type Ping interface {
	Ping(ctx context.Context) (string, error)
}

type ping struct {
	weaver.Implements[Ping]
}

func (p *ping) Ping(ctx context.Context) (string, error) {
	return "pong", nil
}
`
	if err := os.WriteFile(filepath.Join(dir, "ping.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Generate(".", []string{"./" + filepath.ToSlash(dir)}, Options{}); err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, generatedCodeFile)); err != nil {
		t.Fatalf("含组件的包应生成 weaver_gen.go: %v", err)
	}
}

// 合规签名（首参 context.Context、末返回 error）不应产生警告。
func TestGenerateNoWarningForConformingSignatures(t *testing.T) {
	dir := filepath.Join("testdata", "comp-tmp")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	src := `package compkeep

import (
	"context"

	"github.com/jun3372/weaver"
)

type Ping interface {
	Ping(ctx context.Context) (string, error)
}

type ping struct {
	weaver.Implements[Ping]
}

func (p *ping) Ping(ctx context.Context) (string, error) {
	return "pong", nil
}
`
	if err := os.WriteFile(filepath.Join(dir, "ping.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	var warns []error
	if err := Generate(".", []string{"./" + filepath.ToSlash(dir)}, Options{Warn: func(err error) { warns = append(warns, err) }}); err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if len(warns) != 0 {
		t.Fatalf("合规签名不应产生警告，实际: %v", warns)
	}
}

// 方法签名不合规（缺 context.Context 首参或缺 error 末返回）应产生警告。
func TestGenerateWarnsOnNonConformingSignatures(t *testing.T) {
	dir := filepath.Join("testdata", "comp-warn-tmp")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	src := `package compwarn

import "github.com/jun3372/weaver"

type Ping interface {
	Ping() string
}

type ping struct {
	weaver.Implements[Ping]
}

func (p *ping) Ping() string {
	return "pong"
}
`
	if err := os.WriteFile(filepath.Join(dir, "ping.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	var warns []error
	if err := Generate(".", []string{"./" + filepath.ToSlash(dir)}, Options{Warn: func(err error) { warns = append(warns, err) }}); err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if len(warns) == 0 {
		t.Fatal("不合规签名应产生警告")
	}
	for _, w := range warns {
		if strings.Contains(w.Error(), "context.Context") {
			return
		}
	}
	t.Fatalf("警告应提及缺失的 context.Context，实际: %v", warns)
}
