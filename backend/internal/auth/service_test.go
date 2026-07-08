package auth

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type memoryStore struct {
	mu       sync.Mutex
	codes    []EmailCode
	users    map[string]User
	sessions map[string]Session
}

func newMemoryStore() *memoryStore {
	return &memoryStore{
		users:    make(map[string]User),
		sessions: make(map[string]Session),
	}
}

func (s *memoryStore) SaveEmailCode(_ context.Context, email string, codeHash string, expiresAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.codes = append(s.codes, EmailCode{
		ID:        "code_1",
		Email:     email,
		CodeHash:  codeHash,
		ExpiresAt: expiresAt,
	})
	return nil
}

func (s *memoryStore) LatestEmailCode(_ context.Context, email string, now time.Time) (EmailCode, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := len(s.codes) - 1; i >= 0; i-- {
		code := s.codes[i]
		if code.Email == email && now.Before(code.ExpiresAt) {
			return code, nil
		}
	}
	return EmailCode{}, ErrEmailCodeNotFound
}

func (s *memoryStore) RecordEmailCodeFailure(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.codes {
		if s.codes[i].ID == id {
			s.codes[i].Attempts++
			return nil
		}
	}
	return nil
}

func (s *memoryStore) ConsumeEmailCode(_ context.Context, id string, _ time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.codes {
		if s.codes[i].ID == id {
			s.codes = append(s.codes[:i], s.codes[i+1:]...)
			return nil
		}
	}
	return ErrInvalidCode
}

func (s *memoryStore) UpsertVerifiedUser(_ context.Context, email string, verifiedAt time.Time) (User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if user, ok := s.users[email]; ok {
		return user, nil
	}
	user := User{
		ID:              "user_1",
		Email:           email,
		EmailVerifiedAt: &verifiedAt,
		Status:          "active",
	}
	s.users[email] = user
	return user, nil
}

func (s *memoryStore) CreateSession(_ context.Context, userID string, tokenHash string, expiresAt time.Time) (Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session := Session{
		ID:        "session_1",
		UserID:    userID,
		ExpiresAt: expiresAt,
	}
	s.sessions[tokenHash] = session
	return session, nil
}

func (s *memoryStore) RevokeSessionByTokenHash(_ context.Context, tokenHash string, _ time.Time) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.sessions[tokenHash]; !ok {
		return false, nil
	}
	delete(s.sessions, tokenHash)
	return true, nil
}

func (s *memoryStore) CurrentUserBySessionTokenHash(_ context.Context, tokenHash string, now time.Time) (User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[tokenHash]
	if !ok || !now.Before(session.ExpiresAt) {
		return User{}, ErrUnauthorized
	}
	for _, user := range s.users {
		if user.ID == session.UserID && user.Status == "active" {
			return user, nil
		}
	}
	return User{}, ErrUnauthorized
}

func (s *memoryStore) SoftDeleteUserBySessionTokenHash(_ context.Context, tokenHash string, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[tokenHash]
	if !ok || !now.Before(session.ExpiresAt) {
		return ErrUnauthorized
	}
	for email, user := range s.users {
		if user.ID == session.UserID && user.Status == "active" {
			user.Status = "disabled"
			s.users[email] = user
			for existingTokenHash, existingSession := range s.sessions {
				if existingSession.UserID == user.ID {
					delete(s.sessions, existingTokenHash)
				}
			}
			return nil
		}
	}
	return ErrUnauthorized
}

type captureSender struct {
	email string
	code  string
}

func (s *captureSender) SendLoginCode(_ context.Context, email string, code string) error {
	s.email = email
	s.code = code
	return nil
}

func TestRequestEmailCode(t *testing.T) {
	store := newMemoryStore()
	sender := &captureSender{}
	service := NewService(store, sender, Config{CodeTTL: time.Minute, ExposeDebugCode: true})
	service.generateCode = func() (string, error) { return "123456", nil }

	result, err := service.RequestEmailCode(context.Background(), "USER@example.com")
	if err != nil {
		t.Fatalf("RequestEmailCode() error = %v", err)
	}
	if result.Email != "user@example.com" {
		t.Fatalf("Email = %q, want user@example.com", result.Email)
	}
	if result.DebugCode != "123456" {
		t.Fatalf("DebugCode = %q, want 123456", result.DebugCode)
	}
	if sender.email != "user@example.com" || sender.code != "123456" {
		t.Fatalf("sender captured email=%q code=%q", sender.email, sender.code)
	}
}

func TestVerifyEmailCodeCreatesUserAndSession(t *testing.T) {
	store := newMemoryStore()
	sender := &captureSender{}
	service := NewService(store, sender, Config{CodeTTL: time.Minute, SessionTTL: time.Hour})
	service.generateCode = func() (string, error) { return "123456", nil }
	service.generateToken = func() (string, error) { return "token_123", nil }

	if _, err := service.RequestEmailCode(context.Background(), "user@example.com"); err != nil {
		t.Fatalf("RequestEmailCode() error = %v", err)
	}

	result, err := service.VerifyEmailCode(context.Background(), "user@example.com", "123456")
	if err != nil {
		t.Fatalf("VerifyEmailCode() error = %v", err)
	}
	if result.User.ID != "user_1" {
		t.Fatalf("User.ID = %q, want user_1", result.User.ID)
	}
	if result.Token != "token_123" {
		t.Fatalf("Token = %q, want token_123", result.Token)
	}
	if _, ok := store.sessions[HashSessionToken("token_123")]; !ok {
		t.Fatal("session was not stored by token hash")
	}
}

func TestCurrentUserReturnsSessionUser(t *testing.T) {
	store := newMemoryStore()
	service := NewService(store, &captureSender{}, Config{CodeTTL: time.Minute, SessionTTL: time.Hour})
	service.generateCode = func() (string, error) { return "123456", nil }
	service.generateToken = func() (string, error) { return "token_123", nil }

	if _, err := service.RequestEmailCode(context.Background(), "user@example.com"); err != nil {
		t.Fatalf("RequestEmailCode() error = %v", err)
	}
	if _, err := service.VerifyEmailCode(context.Background(), "user@example.com", "123456"); err != nil {
		t.Fatalf("VerifyEmailCode() error = %v", err)
	}

	user, err := service.CurrentUser(context.Background(), "token_123")
	if err != nil {
		t.Fatalf("CurrentUser() error = %v", err)
	}
	if user.Email != "user@example.com" {
		t.Fatalf("Email = %q, want user@example.com", user.Email)
	}
}

func TestDeleteAccountSoftDeletesUserAndRevokesSessions(t *testing.T) {
	store := newMemoryStore()
	service := NewService(store, &captureSender{}, Config{CodeTTL: time.Minute, SessionTTL: time.Hour})
	service.generateCode = func() (string, error) { return "123456", nil }
	service.generateToken = func() (string, error) { return "token_123", nil }

	if _, err := service.RequestEmailCode(context.Background(), "user@example.com"); err != nil {
		t.Fatalf("RequestEmailCode() error = %v", err)
	}
	if _, err := service.VerifyEmailCode(context.Background(), "user@example.com", "123456"); err != nil {
		t.Fatalf("VerifyEmailCode() error = %v", err)
	}

	if err := service.DeleteAccount(context.Background(), "token_123"); err != nil {
		t.Fatalf("DeleteAccount() error = %v", err)
	}
	if status := store.users["user@example.com"].Status; status != "disabled" {
		t.Fatalf("user status = %q, want disabled", status)
	}
	if _, ok := store.sessions[HashSessionToken("token_123")]; ok {
		t.Fatal("session was not revoked")
	}
	if _, err := service.CurrentUser(context.Background(), "token_123"); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("CurrentUser() after delete error = %v, want ErrUnauthorized", err)
	}
}

func TestVerifyEmailCodeRejectsWrongCode(t *testing.T) {
	store := newMemoryStore()
	service := NewService(store, &captureSender{}, Config{CodeTTL: time.Minute})
	service.generateCode = func() (string, error) { return "123456", nil }

	if _, err := service.RequestEmailCode(context.Background(), "user@example.com"); err != nil {
		t.Fatalf("RequestEmailCode() error = %v", err)
	}

	_, err := service.VerifyEmailCode(context.Background(), "user@example.com", "654321")
	if !errors.Is(err, ErrInvalidCode) {
		t.Fatalf("VerifyEmailCode() error = %v, want ErrInvalidCode", err)
	}
}

func TestNormalizeEmailRejectsInvalidEmail(t *testing.T) {
	tests := []string{"", "missing-at", "Name <user@example.com>", "user@bad host"}

	for _, tt := range tests {
		t.Run(tt, func(t *testing.T) {
			if _, err := NormalizeEmail(tt); !errors.Is(err, ErrInvalidEmail) {
				t.Fatalf("NormalizeEmail() error = %v, want ErrInvalidEmail", err)
			}
		})
	}
}
