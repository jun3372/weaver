package gocron

type Config struct {
	Enabled  bool   `default:"false"`
	Timezone string `default:""`
}
