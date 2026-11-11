package generate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// newGenerateCmd 返回一个只挂载 generate 子命令的根命令,在进程内驱动
// cobra 的错误路径,不依赖编译好的二进制。
func newGenerateCmd() *cobra.Command {
	root := &cobra.Command{Use: "weaver", SilenceUsage: true, SilenceErrors: true}
	root.AddCommand(GenerateCmd)
	return root
}

// executeGenerate 以给定参数执行 generate,返回其错误。
func executeGenerate(t *testing.T, args ...string) error {
	t.Helper()
	root := newGenerateCmd()
	root.SetArgs(append([]string{"generate"}, args...))
	root.SetContext(t.Context())
	return root.Execute()
}

// 缺参数必须返回错误(而非仅告警后正常返回),否则 CI 感知不到误用。
func TestRunGenerateMissingArgumentReturnsError(t *testing.T) {
	err := executeGenerate(t)
	if err == nil {
		t.Fatal("无参数调用 generate 应返回错误,实际返回 nil(退出码将为 0)")
	}
	msg := err.Error()
	if !strings.Contains(msg, "缺少必需参数") {
		t.Fatalf("错误信息应说明缺少参数, got: %q", msg)
	}
	// 根命令设置 SilenceUsage 后 cobra 不再打印 usage,错误信息必须自带用法。
	// 这里只断言命令行原文,不依赖中文 usage 模板(模板由 cmd/weaver 包设置)。
	if !strings.Contains(msg, "weaver generate [packages]") {
		t.Fatalf("错误信息应包含用法说明, got: %q", msg)
	}
}

// 目标包不存在时 packages.Load 报错,必须向上返回。
func TestRunGenerateMissingPackageReturnsError(t *testing.T) {
	err := executeGenerate(t, "./no/such/package")
	if err == nil {
		t.Fatal("目标包不存在时应返回错误,实际返回 nil(退出码将为 0)")
	}
	if !strings.Contains(err.Error(), "生成代码失败") {
		t.Fatalf("错误信息应带生成失败上下文, got: %q", err.Error())
	}
}

// 生成过程出错时必须向上返回错误,并带上生成器的具体原因。
func TestRunGenerateGenerationFailureReturnsError(t *testing.T) {
	err := executeGenerate(t, "./testdata/dups")
	if err == nil {
		t.Fatal("组件重复实现时应返回错误,实际返回 nil(退出码将为 0)")
	}
	if !strings.Contains(err.Error(), "Duplicate implementation") {
		t.Fatalf("错误信息应包含生成器的具体原因, got: %q", err.Error())
	}
}

// 成功路径行为不变:正常生成、返回 nil,并产出 weaver_gen.go。
func TestRunGenerateSuccessReturnsNil(t *testing.T) {
	dir := filepath.Join("testdata", "success-tmp")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	src := `package successtmp

import (
	"context"

	"github.com/jun3372/weaver"
)

type Svc interface {
	Ping(ctx context.Context) (string, error)
}

type svcImpl struct {
	weaver.Implements[Svc]
}

func (svcImpl) Ping(ctx context.Context) (string, error) { return "pong", nil }
`

	if err := os.WriteFile(filepath.Join(dir, "svc.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := executeGenerate(t, "./"+filepath.ToSlash(dir)); err != nil {
		t.Fatalf("合法包生成应成功: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "weaver_gen.go")); err != nil {
		t.Fatalf("成功生成后应产出 weaver_gen.go: %v", err)
	}
}
