package nats

type Config struct {
	Enabled bool   `default:"false"`
	DSN     string `default:"nats://localhost:4222"`
}
