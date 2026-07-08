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
	t.Setenv("DATABASE_MAX_OPEN_CONNS", "")
	t.Setenv("DATABASE_MAX_IDLE_CONNS", "")
	t.Setenv("DATABASE_CONN_MAX_LIFETIME", "")
	t.Setenv("DATABASE_CONN_MAX_IDLE_TIME", "")
	t.Setenv("AUTH_CODE_TTL", "")
	t.Setenv("AUTH_SESSION_TTL", "")
	t.Setenv("AUTH_COOKIE_NAME", "")
	t.Setenv("REDIS_ADDR", "")
	t.Setenv("REDIS_PASSWORD", "")
	t.Setenv("REDIS_DB", "")
	t.Setenv("STORAGE_ENDPOINT", "")
	t.Setenv("STORAGE_REGION", "")
	t.Setenv("STORAGE_BUCKET", "")
	t.Setenv("STORAGE_ACCESS_KEY_ID", "")
	t.Setenv("STORAGE_SECRET_ACCESS_KEY", "")
	t.Setenv("STORAGE_USE_SSL", "")
	t.Setenv("STORAGE_SIGNED_URL_TTL", "")
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
	if cfg.Database.MaxOpenConns != 10 {
		t.Fatalf("Database.MaxOpenConns = %d, want 10", cfg.Database.MaxOpenConns)
	}
	if cfg.Database.MaxIdleConns != 5 {
		t.Fatalf("Database.MaxIdleConns = %d, want 5", cfg.Database.MaxIdleConns)
	}
	if cfg.Database.ConnMaxLifetime != 30*time.Minute {
		t.Fatalf("Database.ConnMaxLifetime = %s, want 30m", cfg.Database.ConnMaxLifetime)
	}
	if cfg.Database.ConnMaxIdleTime != 5*time.Minute {
		t.Fatalf("Database.ConnMaxIdleTime = %s, want 5m", cfg.Database.ConnMaxIdleTime)
	}
	if cfg.Auth.CodeTTL != 10*time.Minute {
		t.Fatalf("Auth.CodeTTL = %s, want 10m", cfg.Auth.CodeTTL)
	}
	if cfg.Auth.SessionTTL != 30*24*time.Hour {
		t.Fatalf("Auth.SessionTTL = %s, want 720h", cfg.Auth.SessionTTL)
	}
	if cfg.Auth.CookieName != "talentpage_session" {
		t.Fatalf("Auth.CookieName = %q, want talentpage_session", cfg.Auth.CookieName)
	}
	if cfg.Redis.Addr != "127.0.0.1:6379" {
		t.Fatalf("Redis.Addr = %q, want 127.0.0.1:6379", cfg.Redis.Addr)
	}
	if cfg.Storage.Bucket != "talentpage-private" {
		t.Fatalf("Storage.Bucket = %q, want talentpage-private", cfg.Storage.Bucket)
	}
	if cfg.Storage.SignedURLTTL != 5*time.Minute {
		t.Fatalf("Storage.SignedURLTTL = %s, want 5m", cfg.Storage.SignedURLTTL)
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
	t.Setenv("DATABASE_MAX_OPEN_CONNS", "20")
	t.Setenv("DATABASE_MAX_IDLE_CONNS", "8")
	t.Setenv("DATABASE_CONN_MAX_LIFETIME", "45m")
	t.Setenv("DATABASE_CONN_MAX_IDLE_TIME", "6m")
	t.Setenv("AUTH_CODE_TTL", "2m")
	t.Setenv("AUTH_SESSION_TTL", "24h")
	t.Setenv("AUTH_COOKIE_NAME", "tp_session")
	t.Setenv("REDIS_ADDR", "localhost:6380")
	t.Setenv("REDIS_PASSWORD", "redis-secret")
	t.Setenv("REDIS_DB", "2")
	t.Setenv("STORAGE_ENDPOINT", "minio:9000")
	t.Setenv("STORAGE_REGION", "ap-east-1")
	t.Setenv("STORAGE_BUCKET", "private-files")
	t.Setenv("STORAGE_ACCESS_KEY_ID", "minio")
	t.Setenv("STORAGE_SECRET_ACCESS_KEY", "minio-secret")
	t.Setenv("STORAGE_USE_SSL", "true")
	t.Setenv("STORAGE_SIGNED_URL_TTL", "2m")
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
	if cfg.Database.MaxOpenConns != 20 {
		t.Fatalf("Database.MaxOpenConns = %d", cfg.Database.MaxOpenConns)
	}
	if cfg.Database.MaxIdleConns != 8 {
		t.Fatalf("Database.MaxIdleConns = %d", cfg.Database.MaxIdleConns)
	}
	if cfg.Database.ConnMaxLifetime != 45*time.Minute {
		t.Fatalf("Database.ConnMaxLifetime = %s", cfg.Database.ConnMaxLifetime)
	}
	if cfg.Database.ConnMaxIdleTime != 6*time.Minute {
		t.Fatalf("Database.ConnMaxIdleTime = %s", cfg.Database.ConnMaxIdleTime)
	}
	if cfg.Auth.CodeTTL != 2*time.Minute {
		t.Fatalf("Auth.CodeTTL = %s", cfg.Auth.CodeTTL)
	}
	if cfg.Auth.SessionTTL != 24*time.Hour {
		t.Fatalf("Auth.SessionTTL = %s", cfg.Auth.SessionTTL)
	}
	if cfg.Auth.CookieName != "tp_session" {
		t.Fatalf("Auth.CookieName = %q", cfg.Auth.CookieName)
	}
	if cfg.Redis.DB != 2 {
		t.Fatalf("Redis.DB = %d", cfg.Redis.DB)
	}
	if !cfg.Storage.UseSSL {
		t.Fatal("Storage.UseSSL = false, want true")
	}
	if cfg.Storage.SignedURLTTL != 2*time.Minute {
		t.Fatalf("Storage.SignedURLTTL = %s", cfg.Storage.SignedURLTTL)
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

func TestValidateRejectsInvalidDatabasePool(t *testing.T) {
	base := Config{
		AppName: "talentpage-api",
		Env:     EnvTest,
		HTTP: HTTPConfig{
			Addr:            ":8080",
			ReadTimeout:     time.Second,
			WriteTimeout:    time.Second,
			ShutdownTimeout: time.Second,
		},
		Database: DatabaseConfig{
			MaxOpenConns:    10,
			MaxIdleConns:    5,
			ConnMaxLifetime: time.Minute,
			ConnMaxIdleTime: time.Minute,
		},
		Auth: AuthConfig{
			CodeTTL:    time.Minute,
			SessionTTL: time.Hour,
			CookieName: "talentpage_session",
		},
		Redis: RedisConfig{
			Addr: "127.0.0.1:6379",
		},
		Storage: StorageConfig{
			Endpoint:     "127.0.0.1:9000",
			Bucket:       "talentpage-private",
			SignedURLTTL: time.Minute,
		},
		AI: AIConfig{
			Provider:      "deepseek",
			Model:         "deepseek-chat",
			PromptVersion: "profile-v1",
			Timeout:       time.Second,
		},
	}

	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{
			name: "max open conns",
			mutate: func(cfg *Config) {
				cfg.Database.MaxOpenConns = 0
			},
		},
		{
			name: "max idle conns",
			mutate: func(cfg *Config) {
				cfg.Database.MaxIdleConns = -1
			},
		},
		{
			name: "idle exceeds open",
			mutate: func(cfg *Config) {
				cfg.Database.MaxIdleConns = 11
			},
		},
		{
			name: "conn max lifetime",
			mutate: func(cfg *Config) {
				cfg.Database.ConnMaxLifetime = 0
			},
		},
		{
			name: "conn max idle time",
			mutate: func(cfg *Config) {
				cfg.Database.ConnMaxIdleTime = 0
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := base
			tt.mutate(&cfg)
			if err := cfg.Validate(); err == nil {
				t.Fatal("Validate() error = nil, want error")
			}
		})
	}
}

func TestValidateRejectsInvalidAuthConfig(t *testing.T) {
	base := Config{
		AppName: "talentpage-api",
		Env:     EnvTest,
		HTTP: HTTPConfig{
			Addr:            ":8080",
			ReadTimeout:     time.Second,
			WriteTimeout:    time.Second,
			ShutdownTimeout: time.Second,
		},
		Database: DatabaseConfig{
			MaxOpenConns:    10,
			MaxIdleConns:    5,
			ConnMaxLifetime: time.Minute,
			ConnMaxIdleTime: time.Minute,
		},
		Auth: AuthConfig{
			CodeTTL:    time.Minute,
			SessionTTL: time.Hour,
			CookieName: "talentpage_session",
		},
		Redis: RedisConfig{
			Addr: "127.0.0.1:6379",
		},
		Storage: StorageConfig{
			Endpoint:     "127.0.0.1:9000",
			Bucket:       "talentpage-private",
			SignedURLTTL: time.Minute,
		},
		AI: AIConfig{
			Provider:      "deepseek",
			Model:         "deepseek-chat",
			PromptVersion: "profile-v1",
			Timeout:       time.Second,
		},
	}

	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{
			name: "code ttl",
			mutate: func(cfg *Config) {
				cfg.Auth.CodeTTL = 0
			},
		},
		{
			name: "session ttl",
			mutate: func(cfg *Config) {
				cfg.Auth.SessionTTL = 0
			},
		},
		{
			name: "cookie name",
			mutate: func(cfg *Config) {
				cfg.Auth.CookieName = ""
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := base
			tt.mutate(&cfg)
			if err := cfg.Validate(); err == nil {
				t.Fatal("Validate() error = nil, want error")
			}
		})
	}
}

func TestLoadRejectsInvalidBool(t *testing.T) {
	t.Setenv("STORAGE_USE_SSL", "sometimes")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want error")
	}
}
