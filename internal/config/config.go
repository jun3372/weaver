package config

type Config struct {
	Logger Logger
}

type Logger struct {
	AddSource bool        `default:"false"`
	Component bool        `default:"false"` // 是否开启组件日志
	Level     string      `default:"info"`
	Type      string      `default:"text"` // json、text
	File      *LoggerFile `json:"file,omitempty" yaml:"file,omitempty" toml:"file,omitempty"`
}

type LoggerFile struct {
	Filename   string // 日志文件名
	MaxSize    int    `default:"100"`  // MaxSize是日志文件在获取之前的最大大小（MB）。
	MaxAge     int    `default:"7"`    // MaxAge是保留旧日志文件的最大天数
	MaxBackups int    `default:"3"`    // MaxBackups是要保留的旧日志文件的最大数量
	LocalTime  bool   `default:"true"` // LocalTime确定用于格式化时间戳的时间是否为本地时间。
	Compress   bool   `default:"true"` // 压缩决定是否应压缩旋转的日志文件。
}

// Tags 返回一个包含支持的配置文件标签的字符串切片。
// 这个函数没有输入参数。
// 返回值是一个字符串切片，包含了如"weaver"、"config"等标签，用于标识支持的配置文件类型。
func Tags() []string {
	return []string{"weaver", "config", "conf"}
	// return []string{"weaver", "config", "conf", "yaml", "yml", "toml", "json"}
}
