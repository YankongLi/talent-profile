package storage

import (
	"context"
	"testing"
	"time"

	"github.com/YankongLi/talent-profile/backend/internal/config"
)

func TestNewMinIOClientRequiresCredentials(t *testing.T) {
	cfg := config.StorageConfig{
		Endpoint: "127.0.0.1:9000",
		Bucket:   "talentpage-private",
	}

	if _, err := NewMinIOClient(cfg); err == nil {
		t.Fatal("NewMinIOClient() error = nil, want error")
	}
}

func TestMinIOClientPresignedGetURL(t *testing.T) {
	client, err := NewMinIOClient(config.StorageConfig{
		Endpoint:        "https://storage.example.com",
		Region:          "us-east-1",
		Bucket:          "talentpage-private",
		AccessKeyID:     "minio",
		SecretAccessKey: "minio-secret",
		UseSSL:          false,
	})
	if err != nil {
		t.Fatalf("NewMinIOClient() error = %v", err)
	}

	u, err := client.PresignedGetURL(context.Background(), "resumes/file.pdf", 5*time.Minute)
	if err != nil {
		t.Fatalf("PresignedGetURL() error = %v", err)
	}

	if u.Scheme != "https" {
		t.Fatalf("url scheme = %q, want https", u.Scheme)
	}
	if u.Host != "storage.example.com" {
		t.Fatalf("url host = %q, want storage.example.com", u.Host)
	}
	if got := u.Query().Get("X-Amz-Expires"); got != "300" {
		t.Fatalf("X-Amz-Expires = %q, want 300", got)
	}
	if got := u.Query().Get("X-Amz-Credential"); got == "" {
		t.Fatal("X-Amz-Credential is empty")
	}
}

func TestMinIOClientPresignedGetURLRejectsInvalidInput(t *testing.T) {
	client, err := NewMinIOClient(config.StorageConfig{
		Endpoint:        "127.0.0.1:9000",
		Bucket:          "talentpage-private",
		AccessKeyID:     "minio",
		SecretAccessKey: "minio-secret",
	})
	if err != nil {
		t.Fatalf("NewMinIOClient() error = %v", err)
	}

	if _, err := client.PresignedGetURL(context.Background(), "", time.Minute); err == nil {
		t.Fatal("PresignedGetURL() empty key error = nil, want error")
	}
	if _, err := client.PresignedGetURL(context.Background(), "resumes/file.pdf", 0); err == nil {
		t.Fatal("PresignedGetURL() zero expiry error = nil, want error")
	}
}
