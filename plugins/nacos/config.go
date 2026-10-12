package nacos

type Config struct {
	Enabled bool   `default:"false"`
	DSN     string `default:"nacos://localhost:8848"`
}
