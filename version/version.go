package version

import (
	"fmt"
	"os"
)

var (
	// Version 项目版本信息
	Version = ""
	// GoVersion Go版本信息
	GoVersion = ""
	// GitCommit git提交commmit id
	GitCommit = ""
	// BuildTime 构建时间
	BuildTime = ""
)

// PrintVersion 输出版本信息
func PrintVersion() {
	if Version != "" {
		fmt.Printf("Version: %s\n", Version)
	}

	if GoVersion != "" {
		fmt.Printf("Go Version: %s\n", GoVersion)
	}

	if GitCommit != "" {
		fmt.Printf("Git Commit: %s\n", GitCommit)
	}

	if BuildTime != "" {
		fmt.Printf("Build Time: %s\n", BuildTime)
	}
	os.Exit(0)
}
