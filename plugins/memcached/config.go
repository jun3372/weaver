package memcached

type Config struct {
	Enabled bool   `default:"false"`
	DSN     string `default:"memcache://localhost:11211"`
}
