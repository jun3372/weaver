package mongodb

type Config struct {
	Enabled bool   `default:"false"`
	DSN     string `default:"mongodb://localhost:27017"`
}
