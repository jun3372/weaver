package db

import "gorm.io/gorm"

type Config struct {
	Enabled    bool         `default:"false"`
	DSN        string       `default:"file:data/data.db?cache=shared&mode=rwc&_busy_timeout=5000&_journal_mode=WAL"`
	GormConfig *gorm.Config `default:"{}"`
}
