package weaver

// 本文件锁定「唯一 env 解析路径」的优先级与类型解析行为:
// 注入链统一为 viper.UnmarshalKey(读文件) → applyEnvVars(env 覆盖) →
// defaults.Apply(env/file 均未设且字段零值时填 default tag),
// 不再存在 AutomaticEnv / BindEnv 产生的第二条解析通道。

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/spf13/viper"

	"github.com/jun3372/weaver/internal/reflection"
	"github.com/jun3372/weaver/runtime/codegen"
)

// envCompConf 覆盖验收要求的字段类型:字符串/整数/布尔/嵌套 struct/time.Duration,
// 另含 uint 与 float 保证 setFieldFromString 各分支一致。
type envCompConf struct {
	Name    string        `default:"file-default"`
	Port    int           `default:"80"`
	Debug   bool          `default:"false"`
	Timeout time.Duration `default:"5s"`
	Rate    float64       `default:"1.5"`
	Quota   uint          `default:"100"`
	Nested  struct {
		Word  string `default:"nested-default"`
		Depth int    `default:"3"`
	}
}

type envIntf interface{ Ping() string }

type envComp struct {
	Implements[envIntf]
	WithConfig[envCompConf] `conf:"envcomp"`
}

func (c *envComp) Ping() string { return "pong" }

// newEnvWidget 用给定 yaml 内容构造只含 envComp 的 widget 并返回组件实例。
func newEnvWidget(t *testing.T, yaml string) (*widget, *envComp) {
	t.Helper()

	confFile := filepath.Join(t.TempDir(), "weaver.yaml")
	if err := os.WriteFile(confFile, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	conf := viper.New()
	conf.SetConfigFile(confFile)
	if err := conf.ReadInConfig(); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	w := newWidget(ctx, cancel, conf, []*codegen.Registration{{
		Name:      "test/env",
		Interface: reflection.Type[envIntf](),
		Impl:      reflect.TypeFor[envComp](),
	}}, "WEAVER")

	c, err := w.getImpl(reflect.TypeFor[envComp]())
	if err != nil {
		t.Fatalf("create component: %v", err)
	}
	return w, c.(*envComp)
}

// TestComponentConfigPriority 表格化锁定组件配置段的 env>file>default 优先级
// 与各字段类型的 env 解析。
func TestComponentConfigPriority(t *testing.T) {
	cases := []struct {
		name  string
		env   map[string]string
		yaml  string
		check func(t *testing.T, c envCompConf)
	}{
		{
			name: "仅默认值:文件与 env 均未设置时填 default tag",
			yaml: "weaver:\n  logger:\n    level: info\n",
			check: func(t *testing.T, c envCompConf) {
				if c.Name != "file-default" || c.Port != 80 || c.Debug || c.Timeout != 5*time.Second ||
					c.Rate != 1.5 || c.Quota != 100 || c.Nested.Word != "nested-default" || c.Nested.Depth != 3 {
					t.Fatalf("default 填充错误: %+v", c)
				}
			},
		},
		{
			name: "文件覆盖默认值",
			yaml: "envcomp:\n  name: from-file\n  port: 8080\n  debug: true\n  timeout: 9s\n  rate: 2.5\n  quota: 200\n  nested:\n    word: nested-file\n    depth: 7\n",
			check: func(t *testing.T, c envCompConf) {
				if c.Name != "from-file" || c.Port != 8080 || !c.Debug || c.Timeout != 9*time.Second ||
					c.Rate != 2.5 || c.Quota != 200 || c.Nested.Word != "nested-file" || c.Nested.Depth != 7 {
					t.Fatalf("文件值未生效: %+v", c)
				}
			},
		},
		{
			name: "env 覆盖文件与默认值(字符串)",
			env:  map[string]string{"WEAVER_ENVCOMP_NAME": "from-env"},
			yaml: "envcomp:\n  name: from-file\n",
			check: func(t *testing.T, c envCompConf) {
				if c.Name != "from-env" {
					t.Fatalf("env 应覆盖文件: got Name=%q", c.Name)
				}
				if c.Port != 80 { // 未涉及的字段仍走 default
					t.Fatalf("未涉及字段应保留 default: got Port=%d", c.Port)
				}
			},
		},
		{
			name: "env 覆盖整数/布尔/Duration/uint/float",
			env: map[string]string{
				"WEAVER_ENVCOMP_PORT":    "9090",
				"WEAVER_ENVCOMP_DEBUG":   "true",
				"WEAVER_ENVCOMP_TIMEOUT": "30s",
				"WEAVER_ENVCOMP_QUOTA":   "300",
				"WEAVER_ENVCOMP_RATE":    "9.75",
			},
			yaml: "envcomp:\n  port: 8080\n  debug: false\n  timeout: 9s\n  quota: 200\n  rate: 2.5\n",
			check: func(t *testing.T, c envCompConf) {
				if c.Port != 9090 || !c.Debug || c.Timeout != 30*time.Second || c.Quota != 300 || c.Rate != 9.75 {
					t.Fatalf("env 覆盖标量失败: %+v", c)
				}
			},
		},
		{
			name: "env 覆盖嵌套 struct 字段",
			env: map[string]string{
				"WEAVER_ENVCOMP_NESTED_WORD":  "nested-env",
				"WEAVER_ENVCOMP_NESTED_DEPTH": "11",
			},
			yaml: "envcomp:\n  nested:\n    word: nested-file\n    depth: 7\n",
			check: func(t *testing.T, c envCompConf) {
				if c.Nested.Word != "nested-env" || c.Nested.Depth != 11 {
					t.Fatalf("嵌套字段 env 覆盖失败: %+v", c.Nested)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			_, c := newEnvWidget(t, tc.yaml)
			tc.check(t, c.Config())
		})
	}
}

// TestExplicitZeroSemantics 显式锁定「env 给零值」的行为,证明单路径化无回归:
// applyEnvVars 注入 0/false 后字段为零值,defaults.Apply 的「字段非零才跳过」
// 判断不成立 → 回填 default tag。已在基线(旧三通道)代码上验证行为一致
// (旧通道 IsSet 命中同一 env,但 defaults 的零值判断同样回填),新旧均为
// Port=80/Debug=false。env 的非零覆盖由上表锁定。
func TestExplicitZeroSemantics(t *testing.T) {
	t.Setenv("WEAVER_ENVCOMP_PORT", "0")
	t.Setenv("WEAVER_ENVCOMP_DEBUG", "false")

	_, c := newEnvWidget(t, "weaver:\n  logger:\n    level: info\n")
	cfg := c.Config()
	if cfg.Port != 80 {
		t.Fatalf("env 零值后 default 回填行为发生变化: got Port=%d, want 80(与基线一致)", cfg.Port)
	}
	if cfg.Debug {
		t.Fatalf("env 显式 false 不应被改判: got Debug=true, want false(与基线一致)")
	}
}

// TestSystemConfigEnvPriority 锁定系统配置段(weaver.logger.*)的 env 覆盖:
// env 名剥掉 weaver 顶层(WEAVER_LOGGER_LEVEL 而非 WEAVER_WEAVER_LOGGER_LEVEL)。
func TestSystemConfigEnvPriority(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		yaml string
		want configLike
	}{
		{
			name: "系统配置默认值",
			yaml: "weaver:\n  logger:\n    level: info\n",
			want: configLike{Level: "info", Type: "text", AddSource: false},
		},
		{
			name: "系统配置 env 覆盖字符串与布尔",
			env: map[string]string{
				"WEAVER_LOGGER_LEVEL":     "debug",
				"WEAVER_LOGGER_TYPE":      "json",
				"WEAVER_LOGGER_ADDSOURCE": "true",
			},
			yaml: "weaver:\n  logger:\n    level: info\n    type: text\n",
			want: configLike{Level: "debug", Type: "json", AddSource: true},
		},
		{
			name: "系统配置 env 覆盖嵌套 file 段整数",
			env: map[string]string{
				"WEAVER_LOGGER_FILE_MAXSIZE": "256",
				"WEAVER_LOGGER_FILE_MAXAGE":  "14",
			},
			yaml: "weaver:\n  logger:\n    level: info\n    file:\n      filename: /tmp/x.log\n      maxsize: 100\n      maxage: 7\n",
			want: configLike{Level: "info", Type: "text", FileMaxSize: 256, FileMaxAge: 14, FileName: "/tmp/x.log"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for k, v := range tc.env {
				t.Setenv(k, v)
			}

			confFile := filepath.Join(t.TempDir(), "weaver.yaml")
			if err := os.WriteFile(confFile, []byte(tc.yaml), 0o644); err != nil {
				t.Fatal(err)
			}
			conf := viper.New()
			conf.SetConfigFile(confFile)
			if err := conf.ReadInConfig(); err != nil {
				t.Fatal(err)
			}

			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			w := newWidget(ctx, cancel, conf, nil, "WEAVER")

			got := configLike{
				Level:     w.option.Logger.Level,
				Type:      w.option.Logger.Type,
				AddSource: w.option.Logger.AddSource,
			}
			if w.option.Logger.File != nil {
				got.FileMaxSize = w.option.Logger.File.MaxSize
				got.FileMaxAge = w.option.Logger.File.MaxAge
				got.FileName = w.option.Logger.File.Filename
			}
			if got != tc.want {
				t.Fatalf("系统配置解析不符:\n got %+v\nwant %+v", got, tc.want)
			}
		})
	}
}

type configLike struct {
	Level       string
	Type        string
	AddSource   bool
	FileMaxSize int
	FileMaxAge  int
	FileName    string
}
