package env

import (
	"casino_backend/internal/config"
	"errors"
	"log"
	"os"
)

const (
	dsnName = "PG_DSN"
)

type pgConfig struct {
	dsn string
}

func NewPGConfig() (config.PGConfig, error) {
	dsn := os.Getenv(dsnName)
	log.Printf("dsn = %v", dsn)
	if len(dsn) == 0 {
		return nil, errors.New("pg dsn not found")
	}

	return &pgConfig{
		dsn: dsn,
	}, nil
}

func (cfg *pgConfig) DSN() string {
	return cfg.dsn
}
