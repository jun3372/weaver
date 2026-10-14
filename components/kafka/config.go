package kafka

type Config struct {
	Enabled bool   `default:"false"`
	DSN     string `default:"kafka://localhost:9092"`
}
