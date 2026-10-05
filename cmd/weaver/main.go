package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jun3372/weaver/cmd/weaver/generate"
	"github.com/jun3372/weaver/cmd/weaver/initialization"
	"github.com/jun3372/weaver/cmd/weaver/make"
	"github.com/jun3372/weaver/cmd/weaver/version"
)

var rootCmd = &cobra.Command{
	Use:   "weaver",
	Short: "Weaver 组件化应用框架的代码生成与项目脚手架 CLI",
	// Args 放开为任意参数,未匹配到子命令时由下方 RunE 输出中文错误,
	// 避免 cobra 内部硬编码的英文 unknown command 分支。
	Args: cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}
		var b strings.Builder
		fmt.Fprintf(&b, "未知命令 %q", args[0])
		if sugg := cmd.SuggestionsFor(args[0]); len(sugg) > 0 {
			b.WriteString("\n\n你是想输入以下命令吗?\n")
			for _, s := range sugg {
				fmt.Fprintf(&b, "  %s\n", s)
			}
			fmt.Fprintf(&b, "使用 \"%s [command] --help\" 查看单个命令的详细信息。", cmd.CommandPath())
		} else {
			fmt.Fprintf(&b, "\n\n运行 \"%s --help\" 查看可用命令。", cmd.CommandPath())
		}
		return errors.New(b.String())
	},
}

// cnUsageTemplate 是 cobra 默认 usage 模板的中文版(帮助模板无固定英文文案,无需覆盖)。
const cnUsageTemplate = `用法:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

别名:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

示例:
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}{{$cmds := .Commands}}{{if eq (len .Groups) 0}}

可用命令:{{range $cmds}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{else}}{{range $group := .Groups}}

{{.Title}}{{range $cmds}}{{if (and (eq .GroupID $group.ID) (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if not .AllChildCommandsHaveGroup}}

其他命令:{{range $cmds}}{{if (and (eq .GroupID "") (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

全局 Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

附加帮助主题:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

使用 "{{.CommandPath}} [command] --help" 查看单个命令的详细信息。{{end}}
`

// completionLong 是 completion 各 shell 子命令的中文说明,键为 shell 名。
var completionLong = map[string]string{
	"bash": `生成 bash 的自动补全脚本。

当前会话加载:
  source <(weaver completion bash)

持久化(二选一):
  echo 'source <(weaver completion bash)' >> ~/.bashrc
  weaver completion bash > /etc/bash_completion.d/weaver

注意:依赖 bash-completion 包,未安装时请先用系统包管理器安装。`,
	"fish": `生成 fish 的自动补全脚本。

当前会话加载:
  weaver completion fish | source

持久化:
  weaver completion fish > ~/.config/fish/completions/weaver.fish`,
	"powershell": `生成 powershell 的自动补全脚本。

当前会话加载:
  weaver completion powershell | Out-String | Invoke-Expression

持久化:把上一条命令追加到 PowerShell 配置文件($PROFILE)中。`,
	"zsh": `生成 zsh 的自动补全脚本。

若 shell 报 "command not found: compdef",请在 ~/.zshrc 开头确保启用补全:
  autoload -U compinit; compinit

当前会话加载:
  source <(weaver completion zsh)

持久化:
  echo 'source <(weaver completion zsh)' >> ~/.zshrc`,
}

func init() {
	rootCmd.AddCommand(version.VersionCmd)
	rootCmd.AddCommand(generate.GenerateCmd)
	rootCmd.AddCommand(initialization.InitializationCmd)
	rootCmd.AddCommand(make.MakeCmd)

	// 内建 help/completion 命令由 cobra 懒创建,这里显式初始化后改写为中文描述。
	rootCmd.InitDefaultHelpCmd()
	rootCmd.InitDefaultCompletionCmd()
	walk(rootCmd, func(c *cobra.Command) {
		c.InitDefaultHelpFlag()
		if f := c.Flags().Lookup("help"); f != nil {
			f.Usage = "显示帮助信息"
		}
	})
	for _, c := range rootCmd.Commands() {
		switch c.Name() {
		case "help":
			c.Short = "查看某个命令的帮助"
		case "completion":
			c.Short = "生成指定 shell 的自动补全脚本"
			c.Long = "生成 bash / fish / powershell / zsh 的自动补全脚本。"
			for _, sub := range c.Commands() {
				sub.Short = "生成 " + sub.Name() + " 的自动补全脚本"
				sub.Long = completionLong[sub.Name()]
				if f := sub.Flags().Lookup("no-descriptions"); f != nil {
					f.Usage = "禁用补全描述"
				}
			}
		}
	}

	rootCmd.SetUsageTemplate(cnUsageTemplate)
	rootCmd.SetErrPrefix("错误:")
	// 错误时不再追加 usage 转储,错误信息本身已含指引。
	rootCmd.SilenceUsage = true
}

// main 中 Execute 出错即以非零码退出;错误输出已由 cobra(经 ErrPrefix 与
// SilenceErrors=false)写到 stderr。
func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// walk 递归遍历 c 及其全部子命令。
func walk(c *cobra.Command, fn func(*cobra.Command)) {
	fn(c)
	for _, sub := range c.Commands() {
		walk(sub, fn)
	}
}
