package config

import (
	"errors"
	"fmt"
	"os"
	"time"
)

const (
	EnvDevelopment = "development"
	EnvTest        = "test"
	EnvStaging     = "staging"
	EnvProduction  = "production"
)

type Config struct {
	AppName string
	Env     string
	HTTP    HTTPConfig
}

type HTTPConfig struct {
	Addr            string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		AppName: getEnv("APP_NAME", "talentpage-api"),
		Env:     getEnv("APP_ENV", EnvDevelopment),
		HTTP: HTTPConfig{
			Addr:            getEnv("HTTP_ADDR", ":8080"),
			ReadTimeout:     getDurationEnv("HTTP_READ_TIMEOUT", 5*time.Second),
			WriteTimeout:    getDurationEnv("HTTP_WRITE_TIMEOUT", 10*time.Second),
			ShutdownTimeout: getDurationEnv("HTTP_SHUTDOWN_TIMEOUT", 10*time.Second),
		},
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	if c.AppName == "" {
		return errors.New("app name is required")
	}
	if c.HTTP.Addr == "" {
		return errors.New("http addr is required")
	}
	if c.HTTP.ReadTimeout <= 0 {
		return errors.New("http read timeout must be positive")
	}
	if c.HTTP.WriteTimeout <= 0 {
		return errors.New("http write timeout must be positive")
	}
	if c.HTTP.ShutdownTimeout <= 0 {
		return errors.New("http shutdown timeout must be positive")
	}

	switch c.Env {
	case EnvDevelopment, EnvTest, EnvStaging, EnvProduction:
		return nil
	default:
		return fmt.Errorf("unsupported app env %q", c.Env)
	}
}

func (c Config) IsProduction() bool {
	return c.Env == EnvProduction
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func getDurationEnv(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	duration, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return duration
}
