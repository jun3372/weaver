package meilisearch

type Config struct {
	Enabled bool   `default:"false"`
	Host    string `default:"http://localhost:7700"`
	APIKey  string `default:""`
}
