package config

import (
	"testing"
	"time"
)

func TestLoadUsesDefaults(t *testing.T) {
	t.Setenv("APP_NAME", "")
	t.Setenv("APP_ENV", "")
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("HTTP_READ_TIMEOUT", "")
	t.Setenv("HTTP_WRITE_TIMEOUT", "")
	t.Setenv("HTTP_SHUTDOWN_TIMEOUT", "")
	t.Setenv("DATABASE_DSN", "")
	t.Setenv("REDIS_ADDR", "")
	t.Setenv("REDIS_PASSWORD", "")
	t.Setenv("REDIS_DB", "")
	t.Setenv("STORAGE_ENDPOINT", "")
	t.Setenv("STORAGE_REGION", "")
	t.Setenv("STORAGE_BUCKET", "")
	t.Setenv("STORAGE_ACCESS_KEY_ID", "")
	t.Setenv("STORAGE_SECRET_ACCESS_KEY", "")
	t.Setenv("STORAGE_USE_SSL", "")
	t.Setenv("AI_PROVIDER", "")
	t.Setenv("DEEPSEEK_API_KEY", "")
	t.Setenv("AI_MODEL", "")
	t.Setenv("AI_PROMPT_VERSION", "")
	t.Setenv("AI_TIMEOUT", "")
	t.Setenv("AI_MAX_RETRIES", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.AppName != "talentpage-api" {
		t.Fatalf("AppName = %q, want talentpage-api", cfg.AppName)
	}
	if cfg.Env != EnvDevelopment {
		t.Fatalf("Env = %q, want %q", cfg.Env, EnvDevelopment)
	}
	if cfg.HTTP.Addr != ":8080" {
		t.Fatalf("HTTP.Addr = %q, want :8080", cfg.HTTP.Addr)
	}
	if cfg.HTTP.ReadTimeout != 5*time.Second {
		t.Fatalf("ReadTimeout = %s, want 5s", cfg.HTTP.ReadTimeout)
	}
	if cfg.Redis.Addr != "127.0.0.1:6379" {
		t.Fatalf("Redis.Addr = %q, want 127.0.0.1:6379", cfg.Redis.Addr)
	}
	if cfg.Storage.Bucket != "talentpage-private" {
		t.Fatalf("Storage.Bucket = %q, want talentpage-private", cfg.Storage.Bucket)
	}
	if cfg.AI.Provider != "deepseek" {
		t.Fatalf("AI.Provider = %q, want deepseek", cfg.AI.Provider)
	}
	if cfg.AI.MaxRetries != 2 {
		t.Fatalf("AI.MaxRetries = %d, want 2", cfg.AI.MaxRetries)
	}
}

func TestLoadReadsEnvironment(t *testing.T) {
	t.Setenv("APP_NAME", "talentpage-api-test")
	t.Setenv("APP_ENV", EnvTest)
	t.Setenv("HTTP_ADDR", "127.0.0.1:9090")
	t.Setenv("HTTP_READ_TIMEOUT", "2s")
	t.Setenv("HTTP_WRITE_TIMEOUT", "3s")
	t.Setenv("HTTP_SHUTDOWN_TIMEOUT", "4s")
	t.Setenv("DATABASE_DSN", "postgres://talent:secret@localhost:5432/talentpage?sslmode=disable")
	t.Setenv("REDIS_ADDR", "localhost:6380")
	t.Setenv("REDIS_PASSWORD", "redis-secret")
	t.Setenv("REDIS_DB", "2")
	t.Setenv("STORAGE_ENDPOINT", "minio:9000")
	t.Setenv("STORAGE_REGION", "ap-east-1")
	t.Setenv("STORAGE_BUCKET", "private-files")
	t.Setenv("STORAGE_ACCESS_KEY_ID", "minio")
	t.Setenv("STORAGE_SECRET_ACCESS_KEY", "minio-secret")
	t.Setenv("STORAGE_USE_SSL", "true")
	t.Setenv("AI_PROVIDER", "deepseek")
	t.Setenv("DEEPSEEK_API_KEY", "test-key")
	t.Setenv("AI_MODEL", "deepseek-reasoner")
	t.Setenv("AI_PROMPT_VERSION", "profile-v2")
	t.Setenv("AI_TIMEOUT", "11s")
	t.Setenv("AI_MAX_RETRIES", "1")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.AppName != "talentpage-api-test" {
		t.Fatalf("AppName = %q", cfg.AppName)
	}
	if cfg.Env != EnvTest {
		t.Fatalf("Env = %q", cfg.Env)
	}
	if cfg.HTTP.Addr != "127.0.0.1:9090" {
		t.Fatalf("HTTP.Addr = %q", cfg.HTTP.Addr)
	}
	if cfg.HTTP.WriteTimeout != 3*time.Second {
		t.Fatalf("WriteTimeout = %s", cfg.HTTP.WriteTimeout)
	}
	if cfg.Database.DSN == "" {
		t.Fatal("Database.DSN is empty")
	}
	if cfg.Redis.DB != 2 {
		t.Fatalf("Redis.DB = %d", cfg.Redis.DB)
	}
	if !cfg.Storage.UseSSL {
		t.Fatal("Storage.UseSSL = false, want true")
	}
	if cfg.AI.Model != "deepseek-reasoner" {
		t.Fatalf("AI.Model = %q", cfg.AI.Model)
	}
	if cfg.AI.Timeout != 11*time.Second {
		t.Fatalf("AI.Timeout = %s", cfg.AI.Timeout)
	}
}

func TestValidateRejectsUnsupportedEnv(t *testing.T) {
	cfg := Config{
		AppName: "talentpage-api",
		Env:     "local",
		HTTP: HTTPConfig{
			Addr:            ":8080",
			ReadTimeout:     time.Second,
			WriteTimeout:    time.Second,
			ShutdownTimeout: time.Second,
		},
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
}

func TestLoadRejectsInvalidDuration(t *testing.T) {
	t.Setenv("APP_NAME", "")
	t.Setenv("APP_ENV", "")
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("HTTP_READ_TIMEOUT", "soon")
	t.Setenv("HTTP_WRITE_TIMEOUT", "")
	t.Setenv("HTTP_SHUTDOWN_TIMEOUT", "")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want error")
	}
}

func TestLoadRejectsInvalidInt(t *testing.T) {
	t.Setenv("REDIS_DB", "primary")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want error")
	}
}

func TestLoadRejectsInvalidBool(t *testing.T) {
	t.Setenv("STORAGE_USE_SSL", "sometimes")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want error")
	}
}
