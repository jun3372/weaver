package mqtt

type Config struct {
	Enabled  bool   `default:"false"`
	DSN      string `default:"mqtt://localhost:1883"`
	ClientID string `default:"weaver"`
}
