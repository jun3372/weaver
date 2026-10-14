package elasticsearch

type Config struct {
	Enabled bool   `default:"false"`
	DSN     string `default:"http://localhost:9200"`
}
