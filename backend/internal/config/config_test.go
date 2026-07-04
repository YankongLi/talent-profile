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
}

func TestLoadReadsEnvironment(t *testing.T) {
	t.Setenv("APP_NAME", "talentpage-api-test")
	t.Setenv("APP_ENV", EnvTest)
	t.Setenv("HTTP_ADDR", "127.0.0.1:9090")
	t.Setenv("HTTP_READ_TIMEOUT", "2s")
	t.Setenv("HTTP_WRITE_TIMEOUT", "3s")
	t.Setenv("HTTP_SHUTDOWN_TIMEOUT", "4s")

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
