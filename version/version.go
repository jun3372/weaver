package version

import (
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
)

var (
	// Version 项目版本信息,构建时通过 -ldflags "-X github.com/jun3372/weaver/version.Version=..." 注入
	Version = ""
	// GoVersion Go版本信息,留空时自动取编译器版本
	GoVersion = ""
	// GitCommit git提交commit id,留空时自动取构建时的 VCS 信息
	GitCommit = ""
	// BuildTime 构建时间,留空时自动取 VCS 提交时间
	BuildTime = ""
)

// PrintVersion 输出版本信息。
// ldflags 注入的字段优先;未注入时依次回退:模块版本(go install 安装时)、
// 编译器版本、VCS 提交与时间。
func PrintVersion() {
	info, _ := debug.ReadBuildInfo()

	fmt.Printf("Version: %s\n", cmpOr(Version, moduleVersion(info), "(dev)"))
	fmt.Printf("Go Version: %s\n", cmpOr(GoVersion, runtime.Version()))

	commit, buildTime := vcsInfo(info)
	if c := cmpOr(GitCommit, commit, ""); c != "" {
		fmt.Printf("Git Commit: %s\n", c)
	}
	if b := cmpOr(BuildTime, buildTime, ""); b != "" {
		fmt.Printf("Build Time: %s\n", b)
	}
	os.Exit(0)
}

// cmpOr 返回第一个非空值,全空返回 def。
func cmpOr(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// moduleVersion 返回主模块版本;仓库内 go run/build 时为 "(devel)",视为未提供。
func moduleVersion(info *debug.BuildInfo) string {
	if info == nil || info.Main.Version == "(devel)" {
		return ""
	}
	return info.Main.Version
}

// vcsInfo 从构建信息中读取 VCS 注入的提交与时间,工作区有未提交修改时追加 -modified。
func vcsInfo(info *debug.BuildInfo) (commit, buildTime string) {
	if info == nil {
		return "", ""
	}
	var modified bool
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			commit = s.Value
		case "vcs.time":
			buildTime = s.Value
		case "vcs.modified":
			modified = s.Value == "true"
		}
	}
	if modified && commit != "" {
		commit += "-modified"
	}
	return commit, buildTime
}
