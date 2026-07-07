package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

const (
	EnvDevelopment = "development"
	EnvTest        = "test"
	EnvStaging     = "staging"
	EnvProduction  = "production"
)

type Config struct {
	AppName  string
	Env      string
	HTTP     HTTPConfig
	Database DatabaseConfig
	Redis    RedisConfig
	Storage  StorageConfig
	AI       AIConfig
}

type HTTPConfig struct {
	Addr            string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration
}

type DatabaseConfig struct {
	DSN string
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type StorageConfig struct {
	Endpoint        string
	Region          string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
	UseSSL          bool
}

type AIConfig struct {
	Provider      string
	APIKey        string
	Model         string
	PromptVersion string
	Timeout       time.Duration
	MaxRetries    int
}

func Load() (Config, error) {
	readTimeout, err := getDurationEnv("HTTP_READ_TIMEOUT", 5*time.Second)
	if err != nil {
		return Config{}, err
	}
	writeTimeout, err := getDurationEnv("HTTP_WRITE_TIMEOUT", 10*time.Second)
	if err != nil {
		return Config{}, err
	}
	shutdownTimeout, err := getDurationEnv("HTTP_SHUTDOWN_TIMEOUT", 10*time.Second)
	if err != nil {
		return Config{}, err
	}
	aiTimeout, err := getDurationEnv("AI_TIMEOUT", 30*time.Second)
	if err != nil {
		return Config{}, err
	}
	redisDB, err := getIntEnv("REDIS_DB", 0)
	if err != nil {
		return Config{}, err
	}
	storageUseSSL, err := getBoolEnv("STORAGE_USE_SSL", false)
	if err != nil {
		return Config{}, err
	}
	aiMaxRetries, err := getIntEnv("AI_MAX_RETRIES", 2)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		AppName: getEnv("APP_NAME", "talentpage-api"),
		Env:     getEnv("APP_ENV", EnvDevelopment),
		HTTP: HTTPConfig{
			Addr:            getEnv("HTTP_ADDR", ":8080"),
			ReadTimeout:     readTimeout,
			WriteTimeout:    writeTimeout,
			ShutdownTimeout: shutdownTimeout,
		},
		Database: DatabaseConfig{
			DSN: getEnv("DATABASE_DSN", ""),
		},
		Redis: RedisConfig{
			Addr:     getEnv("REDIS_ADDR", "127.0.0.1:6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       redisDB,
		},
		Storage: StorageConfig{
			Endpoint:        getEnv("STORAGE_ENDPOINT", "127.0.0.1:9000"),
			Region:          getEnv("STORAGE_REGION", "us-east-1"),
			Bucket:          getEnv("STORAGE_BUCKET", "talentpage-private"),
			AccessKeyID:     getEnv("STORAGE_ACCESS_KEY_ID", ""),
			SecretAccessKey: getEnv("STORAGE_SECRET_ACCESS_KEY", ""),
			UseSSL:          storageUseSSL,
		},
		AI: AIConfig{
			Provider:      getEnv("AI_PROVIDER", "deepseek"),
			APIKey:        getEnv("DEEPSEEK_API_KEY", ""),
			Model:         getEnv("AI_MODEL", "deepseek-chat"),
			PromptVersion: getEnv("AI_PROMPT_VERSION", "profile-v1"),
			Timeout:       aiTimeout,
			MaxRetries:    aiMaxRetries,
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
	if c.Redis.Addr == "" {
		return errors.New("redis addr is required")
	}
	if c.Redis.DB < 0 {
		return errors.New("redis db must not be negative")
	}
	if c.Storage.Endpoint == "" {
		return errors.New("storage endpoint is required")
	}
	if c.Storage.Bucket == "" {
		return errors.New("storage bucket is required")
	}
	if c.AI.Provider == "" {
		return errors.New("ai provider is required")
	}
	if c.AI.Model == "" {
		return errors.New("ai model is required")
	}
	if c.AI.PromptVersion == "" {
		return errors.New("ai prompt version is required")
	}
	if c.AI.Timeout <= 0 {
		return errors.New("ai timeout must be positive")
	}
	if c.AI.MaxRetries < 0 {
		return errors.New("ai max retries must not be negative")
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

func getDurationEnv(key string, fallback time.Duration) (time.Duration, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}

	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", key, err)
	}
	return duration, nil
}

func getIntEnv(key string, fallback int) (int, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", key, err)
	}
	return parsed, nil
}

func getBoolEnv(key string, fallback bool) (bool, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}

	switch value {
	case "1", "true", "TRUE", "True", "yes", "YES", "Yes":
		return true, nil
	case "0", "false", "FALSE", "False", "no", "NO", "No":
		return false, nil
	default:
		return false, fmt.Errorf("parse %s: unsupported boolean %q", key, value)
	}
}
