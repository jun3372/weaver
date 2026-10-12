package etcd

type Config struct {
	Enabled bool   `default:"false"`
	DSN     string `default:"etcd://localhost:2379"`
}
