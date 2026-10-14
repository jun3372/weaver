package s3

type Config struct {
	Enabled bool   `default:"false"`
	DSN     string `default:"s3://minioadmin:minioadmin@localhost:9000?secure=false"`
}
