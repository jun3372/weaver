package config

type Config struct {
	Logger Logger
}

type Logger struct {
	AddSource bool
	Component bool // 是否开启组件日志
	Level     string
	Type      string      // json、text
	File      *LoggerFile `json:"file,omitempty" yaml:"file,omitempty" toml:"file,omitempty"`
}

type LoggerFile struct {
	Filename   string // 日志文件名
	MaxSize    int    // MaxSize是日志文件在获取之前的最大大小（MB）。默认值为100 MB。
	MaxAge     int    // MaxAge是保留旧日志文件的最大天数
	MaxBackups int    // MaxBackups是要保留的旧日志文件的最大数量
	LocalTime  bool   // LocalTime确定用于格式化时间戳的时间是否为本地时间。备份文件是计算机的本地时间。默认情况下使用UTC时间。
	Compress   bool   // 压缩决定是否应压缩旋转的日志文件。使用gzip。默认情况下不执行压缩。
}
