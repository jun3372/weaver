package main

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
)

// 根命令 -version 应打印版本信息并以 0 退出,输出与 `weaver version` 同源。
func TestVersionFlagPrintsAndExits(t *testing.T) {
	out, err := exec.Command("go", "run", ".", "-version").CombinedOutput()
	if err != nil {
		t.Fatalf("weaver -version 应成功执行: %v\n%s", err, out)
	}
	if !strings.HasPrefix(string(out), "Weaver") {
		t.Fatalf("版本输出应以 \"Weaver\" 开头, got:\n%s", out)
	}
}

// generate 的失败路径必须以非 0 退出码结束,stderr 有可读错误;
// 这里验证真实进程退出码,而不只是 cobra 的返回错误。
func TestGenerateFailuresExitNonZero(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantHint string
	}{
		{"缺参数", []string{"generate"}, "缺少必需参数"},
		{"目标包不存在", []string{"generate", "./no/such/package"}, "生成代码失败"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := exec.Command("go", append([]string{"run", "."}, tt.args...)...).CombinedOutput()
			if err == nil {
				t.Fatalf("weaver %v 应以非 0 退出, got 0:\n%s", tt.args, out)
			}
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) {
				t.Fatalf("应得到进程退出错误, got %v", err)
			}
			if exitErr.ExitCode() == 0 {
				t.Fatalf("退出码应为非 0, got 0:\n%s", out)
			}
			if !strings.Contains(string(out), tt.wantHint) {
				t.Fatalf("stderr 应包含 %q, got:\n%s", tt.wantHint, out)
			}
		})
	}
}

// generate 成功路径仍以 0 退出。
func TestGenerateSuccessExitsZero(t *testing.T) {
	out, err := exec.Command("go", "run", ".", "generate", "../../examples/hello").CombinedOutput()
	if err != nil {
		t.Fatalf("合法包生成应以 0 退出: %v\n%s", err, out)
	}
}
