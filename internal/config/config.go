package config

type Config struct {
	Logger Logger
}

type Logger struct {
	Level     string
	AddSource bool
	Type      string // json、text
	File      string
}
