package zookeeper

type Config struct {
	Enabled bool   `default:"false"`
	DSN     string `default:"zk://localhost:2181"`
}
