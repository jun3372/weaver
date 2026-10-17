package main

import (
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
