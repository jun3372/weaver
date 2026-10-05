package version

import (
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"strings"
	"time"
)

var (
	// Version 项目版本信息,构建时通过 -ldflags "-X github.com/jun3372/weaver/version.Version=..." 注入
	Version = ""
	// GoVersion Go版本信息,留空时自动取编译器版本
	GoVersion = ""
	// GitCommit git提交commit id,留空时自动取构建时的 VCS 信息
	GitCommit = ""
	// BuildTime 构建时间,留空时自动取 VCS 提交时间
	BuildTime = time.Now().Format(time.DateTime + " MST")
)

// PrintVersion 输出版本信息。
// ldflags 注入的字段优先;未注入时依次回退:模块版本(go install 安装时)、
// 编译器版本、VCS 提交与时间。
func PrintVersion() {
	info, _ := debug.ReadBuildInfo()

	fmt.Printf("Weaver %s\n", cmpOr(Version, moduleVersion(info), "(dev)"))

	row("Go", cmpOr(GoVersion, runtime.Version()))
	row("OS/Arch", runtime.GOOS+"/"+runtime.GOARCH)

	commit, buildTime := vcsInfo(info)
	if c := cmpOr(GitCommit, commit); c != "" {
		row("Commit", shortCommit(c))
	}
	if b := cmpOr(buildTime, BuildTime); b != "" {
		row("Built", fmtTime(b))
	}
	os.Exit(0)
}

// row 输出对齐的键值行,值为空时整行省略。
func row(key, val string) {
	if val == "" {
		return
	}
	fmt.Printf("  %-8s %s\n", key+":", val)
}

// shortCommit 将完整 40 位提交哈希截短为 7 位,保留 -modified 后缀。
func shortCommit(commit string) string {
	if s, ok := strings.CutSuffix(commit, "-modified"); ok {
		return shortCommit(s) + "-modified"
	}
	if len(commit) == 40 {
		return commit[:7]
	}
	return commit
}

// fmtTime 将 VCS 的 RFC3339 时间戳格式化为更易读的形式,解析失败时原样返回。
func fmtTime(t string) string {
	if ts, err := time.Parse(time.RFC3339, t); err == nil {
		return ts.UTC().Format("2006-01-02 15:04:05 UTC")
	}
	return t
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
