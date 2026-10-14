package nsq

type Config struct {
	Enabled bool   `default:"false"`
	DSN     string `default:"nsq://localhost:4150"`
}
