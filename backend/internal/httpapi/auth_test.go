package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/YankongLi/talent-profile/backend/internal/auth"
)

type authTestStore struct {
	mu       sync.Mutex
	codes    []auth.EmailCode
	users    map[string]auth.User
	sessions map[string]auth.Session
}

func newAuthTestStore() *authTestStore {
	return &authTestStore{
		users:    make(map[string]auth.User),
		sessions: make(map[string]auth.Session),
	}
}

func (s *authTestStore) SaveEmailCode(_ context.Context, email string, codeHash string, expiresAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.codes = append(s.codes, auth.EmailCode{ID: "code_1", Email: email, CodeHash: codeHash, ExpiresAt: expiresAt})
	return nil
}

func (s *authTestStore) LatestEmailCode(_ context.Context, email string, now time.Time) (auth.EmailCode, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := len(s.codes) - 1; i >= 0; i-- {
		code := s.codes[i]
		if code.Email == email && now.Before(code.ExpiresAt) {
			return code, nil
		}
	}
	return auth.EmailCode{}, auth.ErrEmailCodeNotFound
}

func (s *authTestStore) RecordEmailCodeFailure(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.codes {
		if s.codes[i].ID == id {
			s.codes[i].Attempts++
		}
	}
	return nil
}

func (s *authTestStore) ConsumeEmailCode(_ context.Context, id string, _ time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.codes {
		if s.codes[i].ID == id {
			s.codes = append(s.codes[:i], s.codes[i+1:]...)
			return nil
		}
	}
	return auth.ErrInvalidCode
}

func (s *authTestStore) UpsertVerifiedUser(_ context.Context, email string, verifiedAt time.Time) (auth.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if user, ok := s.users[email]; ok {
		return user, nil
	}
	user := auth.User{ID: "user_1", Email: email, EmailVerifiedAt: &verifiedAt, Status: "active"}
	s.users[email] = user
	return user, nil
}

func (s *authTestStore) CreateSession(_ context.Context, userID string, tokenHash string, expiresAt time.Time) (auth.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	session := auth.Session{ID: "session_1", UserID: userID, ExpiresAt: expiresAt}
	s.sessions[tokenHash] = session
	return session, nil
}

func (s *authTestStore) RevokeSessionByTokenHash(_ context.Context, tokenHash string, _ time.Time) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.sessions[tokenHash]; !ok {
		return false, nil
	}
	delete(s.sessions, tokenHash)
	return true, nil
}

func (s *authTestStore) CurrentUserBySessionTokenHash(_ context.Context, tokenHash string, now time.Time) (auth.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[tokenHash]
	if !ok || !now.Before(session.ExpiresAt) {
		return auth.User{}, auth.ErrUnauthorized
	}
	for _, user := range s.users {
		if user.ID == session.UserID && user.Status == "active" {
			return user, nil
		}
	}
	return auth.User{}, auth.ErrUnauthorized
}

func (s *authTestStore) SoftDeleteUserBySessionTokenHash(_ context.Context, tokenHash string, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[tokenHash]
	if !ok || !now.Before(session.ExpiresAt) {
		return auth.ErrUnauthorized
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
	return auth.ErrUnauthorized
}

type authTestSender struct{}

func (authTestSender) SendLoginCode(context.Context, string, string) error {
	return nil
}

func TestAuthEmailCodeAndVerifyRoutes(t *testing.T) {
	service := auth.NewService(newAuthTestStore(), authTestSender{}, auth.Config{
		CodeTTL:         time.Minute,
		SessionTTL:      time.Hour,
		ExposeDebugCode: true,
	})
	router := NewRouter(testConfig(), WithAuthService(service))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/email-code", bytes.NewBufferString(`{"email":"USER@example.com"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("email-code status = %d, want %d", rec.Code, http.StatusOK)
	}
	var codeBody emailCodeResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &codeBody); err != nil {
		t.Fatalf("decode email-code response: %v", err)
	}
	if codeBody.Email != "user@example.com" {
		t.Fatalf("email = %q, want user@example.com", codeBody.Email)
	}
	if codeBody.DebugCode == "" {
		t.Fatal("debug code is empty")
	}

	verifyBody := `{"email":"user@example.com","code":"` + codeBody.DebugCode + `"}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/verify", bytes.NewBufferString(verifyBody))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("verify status = %d, want %d", rec.Code, http.StatusOK)
	}
	var body verifyResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode verify response: %v", err)
	}
	if body.Token == "" {
		t.Fatal("token is empty")
	}
	if body.User.ID != "user_1" {
		t.Fatalf("user id = %q, want user_1", body.User.ID)
	}
	if got := rec.Result().Cookies(); len(got) == 0 || got[0].Name != "talentpage_session" || !got[0].HttpOnly {
		t.Fatalf("session cookie = %#v", got)
	}
}

func TestAuthVerifyRejectsInvalidCode(t *testing.T) {
	service := auth.NewService(newAuthTestStore(), authTestSender{}, auth.Config{CodeTTL: time.Minute})
	router := NewRouter(testConfig(), WithAuthService(service))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/verify", bytes.NewBufferString(`{"email":"user@example.com","code":"000000"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestAuthMeAndDeleteAccountRoutes(t *testing.T) {
	service := auth.NewService(newAuthTestStore(), authTestSender{}, auth.Config{
		CodeTTL:         time.Minute,
		SessionTTL:      time.Hour,
		ExposeDebugCode: true,
	})
	router := NewRouter(testConfig(), WithAuthService(service))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/email-code", bytes.NewBufferString(`{"email":"user@example.com"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	var codeBody emailCodeResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &codeBody); err != nil {
		t.Fatalf("decode email-code response: %v", err)
	}

	verifyBody := `{"email":"user@example.com","code":"` + codeBody.DebugCode + `"}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/verify", bytes.NewBufferString(verifyBody))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	var verifyBodyResponse verifyResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &verifyBodyResponse); err != nil {
		t.Fatalf("decode verify response: %v", err)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+verifyBodyResponse.Token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("me status = %d, want %d", rec.Code, http.StatusOK)
	}
	var meBody meResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &meBody); err != nil {
		t.Fatalf("decode me response: %v", err)
	}
	if meBody.User.Email != "user@example.com" {
		t.Fatalf("me email = %q, want user@example.com", meBody.User.Email)
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/v1/account", nil)
	req.Header.Set("Authorization", "Bearer "+verifyBodyResponse.Token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("delete account status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Result().Cookies(); len(got) == 0 || got[0].Name != "talentpage_session" || got[0].MaxAge != -1 {
		t.Fatalf("clear session cookie = %#v", got)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+verifyBodyResponse.Token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("me after delete status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}
