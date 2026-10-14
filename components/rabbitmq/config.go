package rabbitmq

type Config struct {
	Enabled bool   `default:"false"`
	DSN     string `default:"amqp://guest:guest@localhost:5672/"`
}
