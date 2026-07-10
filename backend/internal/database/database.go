package database

import (
	"fmt"

	"github.com/shelly-app/shelly/internal/config"
	"github.com/shelly-app/shelly/internal/model"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func Init(cfg *config.DatabaseConfig) error {
	var err error

	switch cfg.Driver {
	case "sqlite", "":
		DB, err = gorm.Open(sqlite.Open(cfg.DSN), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
	default:
		return fmt.Errorf("unsupported database driver: %s", cfg.Driver)
	}

	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}

	// Enable WAL mode for SQLite
	if cfg.Driver == "sqlite" || cfg.Driver == "" {
		DB.Exec("PRAGMA journal_mode=WAL")
		DB.Exec("PRAGMA foreign_keys=ON")
	}

	// Auto migrate all models
	return DB.AutoMigrate(
		&model.User{},
		&model.Asset{},
		&model.AssetGroup{},
		&model.CommandSnippet{},
		&model.HighlightRule{},
		&model.SessionRecord{},
		&model.PortForwardRule{},
		&model.AIChatSession{},
		&model.AIChatMessage{},
		&model.SyncConfig{},
		&model.APIToken{},
		&model.AppSettings{},
	)
}
