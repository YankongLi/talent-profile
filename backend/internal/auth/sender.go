package auth

import (
	"context"
	"log/slog"
)

type LogEmailCodeSender struct {
	Logger      *slog.Logger
	IncludeCode bool
}

func (s LogEmailCodeSender) SendLoginCode(_ context.Context, email string, code string) error {
	logger := s.Logger
	if logger == nil {
		logger = slog.Default()
	}

	if s.IncludeCode {
		logger.Info("email login code generated", "email", email, "code", code)
		return nil
	}

	logger.Info("email login code generated", "email", email)
	return nil
}
