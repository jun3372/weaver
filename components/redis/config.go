package redis

type Config struct {
	Enabled bool   `default:"false"`
	DSN     string `default:"redis://localhost:6379/0"`
}
