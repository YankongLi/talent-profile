package database

import (
	"context"
	"testing"
	"time"

	"github.com/YankongLi/talent-profile/backend/internal/config"
)

func TestOpenRejectsMissingDSN(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err := Open(ctx, config.DatabaseConfig{
		MaxOpenConns:    1,
		MaxIdleConns:    1,
		ConnMaxLifetime: time.Minute,
		ConnMaxIdleTime: time.Minute,
	})
	if err == nil {
		t.Fatal("Open() error = nil, want error")
	}
}
