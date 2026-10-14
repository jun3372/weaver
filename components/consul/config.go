package consul

type Config struct {
	Enabled bool   `default:"false"`
	DSN     string `default:"http://127.0.0.1:8500"`
}
