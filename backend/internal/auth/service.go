package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"net/mail"
	"strings"
	"time"
)

const (
	defaultCodeTTL    = 10 * time.Minute
	defaultSessionTTL = 30 * 24 * time.Hour
	maxCodeAttempts   = 5
)

var (
	ErrInvalidEmail       = errors.New("invalid email")
	ErrInvalidCode        = errors.New("invalid code")
	ErrEmailCodeNotFound  = errors.New("email code not found")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrSessionTokenEmpty  = errors.New("session token is empty")
	ErrEmailSenderMissing = errors.New("email sender is required")
)

type Store interface {
	SaveEmailCode(ctx context.Context, email string, codeHash string, expiresAt time.Time) error
	LatestEmailCode(ctx context.Context, email string, now time.Time) (EmailCode, error)
	RecordEmailCodeFailure(ctx context.Context, id string) error
	ConsumeEmailCode(ctx context.Context, id string, consumedAt time.Time) error
	UpsertVerifiedUser(ctx context.Context, email string, verifiedAt time.Time) (User, error)
	CreateSession(ctx context.Context, userID string, tokenHash string, expiresAt time.Time) (Session, error)
	RevokeSessionByTokenHash(ctx context.Context, tokenHash string, revokedAt time.Time) (bool, error)
}

type EmailCodeSender interface {
	SendLoginCode(ctx context.Context, email string, code string) error
}

type Config struct {
	CodeTTL         time.Duration
	SessionTTL      time.Duration
	ExposeDebugCode bool
}

type Service struct {
	store         Store
	sender        EmailCodeSender
	codeTTL       time.Duration
	sessionTTL    time.Duration
	debugCodes    bool
	now           func() time.Time
	generateCode  func() (string, error)
	generateToken func() (string, error)
}

type EmailCode struct {
	ID        string
	Email     string
	CodeHash  string
	Attempts  int
	ExpiresAt time.Time
}

type User struct {
	ID              string     `json:"id"`
	Email           string     `json:"email"`
	EmailVerifiedAt *time.Time `json:"email_verified_at,omitempty"`
	Status          string     `json:"status"`
}

type Session struct {
	ID        string
	UserID    string
	ExpiresAt time.Time
}

type RequestEmailCodeResult struct {
	Email            string
	ExpiresAt        time.Time
	ExpiresInSeconds int
	DebugCode        string
}

type VerifyResult struct {
	User      User
	Token     string
	ExpiresAt time.Time
}

func NewService(store Store, sender EmailCodeSender, cfg Config) *Service {
	codeTTL := cfg.CodeTTL
	if codeTTL == 0 {
		codeTTL = defaultCodeTTL
	}
	sessionTTL := cfg.SessionTTL
	if sessionTTL == 0 {
		sessionTTL = defaultSessionTTL
	}

	return &Service{
		store:         store,
		sender:        sender,
		codeTTL:       codeTTL,
		sessionTTL:    sessionTTL,
		debugCodes:    cfg.ExposeDebugCode,
		now:           time.Now,
		generateCode:  generateNumericCode,
		generateToken: generateSessionToken,
	}
}

func (s *Service) RequestEmailCode(ctx context.Context, email string) (RequestEmailCodeResult, error) {
	normalizedEmail, err := NormalizeEmail(email)
	if err != nil {
		return RequestEmailCodeResult{}, err
	}
	if s.sender == nil {
		return RequestEmailCodeResult{}, ErrEmailSenderMissing
	}

	code, err := s.generateCode()
	if err != nil {
		return RequestEmailCodeResult{}, fmt.Errorf("generate email code: %w", err)
	}

	now := s.now().UTC()
	expiresAt := now.Add(s.codeTTL)
	if err := s.store.SaveEmailCode(ctx, normalizedEmail, hashLoginCode(normalizedEmail, code), expiresAt); err != nil {
		return RequestEmailCodeResult{}, err
	}
	if err := s.sender.SendLoginCode(ctx, normalizedEmail, code); err != nil {
		return RequestEmailCodeResult{}, err
	}

	result := RequestEmailCodeResult{
		Email:            normalizedEmail,
		ExpiresAt:        expiresAt,
		ExpiresInSeconds: int(s.codeTTL.Seconds()),
	}
	if s.debugCodes {
		result.DebugCode = code
	}
	return result, nil
}

func (s *Service) VerifyEmailCode(ctx context.Context, email string, code string) (VerifyResult, error) {
	normalizedEmail, err := NormalizeEmail(email)
	if err != nil {
		return VerifyResult{}, err
	}
	if !validCodeFormat(code) {
		return VerifyResult{}, ErrInvalidCode
	}

	now := s.now().UTC()
	emailCode, err := s.store.LatestEmailCode(ctx, normalizedEmail, now)
	if errors.Is(err, ErrEmailCodeNotFound) {
		return VerifyResult{}, ErrInvalidCode
	}
	if err != nil {
		return VerifyResult{}, err
	}
	if emailCode.Attempts >= maxCodeAttempts || now.After(emailCode.ExpiresAt) {
		return VerifyResult{}, ErrInvalidCode
	}

	expectedHash := hashLoginCode(normalizedEmail, code)
	if subtle.ConstantTimeCompare([]byte(emailCode.CodeHash), []byte(expectedHash)) != 1 {
		_ = s.store.RecordEmailCodeFailure(ctx, emailCode.ID)
		return VerifyResult{}, ErrInvalidCode
	}

	if err := s.store.ConsumeEmailCode(ctx, emailCode.ID, now); err != nil {
		return VerifyResult{}, err
	}

	user, err := s.store.UpsertVerifiedUser(ctx, normalizedEmail, now)
	if err != nil {
		return VerifyResult{}, err
	}

	token, err := s.generateToken()
	if err != nil {
		return VerifyResult{}, fmt.Errorf("generate session token: %w", err)
	}
	expiresAt := now.Add(s.sessionTTL)
	if _, err := s.store.CreateSession(ctx, user.ID, HashSessionToken(token), expiresAt); err != nil {
		return VerifyResult{}, err
	}

	return VerifyResult{
		User:      user,
		Token:     token,
		ExpiresAt: expiresAt,
	}, nil
}

func (s *Service) Logout(ctx context.Context, token string) error {
	if strings.TrimSpace(token) == "" {
		return ErrSessionTokenEmpty
	}
	_, err := s.store.RevokeSessionByTokenHash(ctx, HashSessionToken(token), s.now().UTC())
	return err
}

func NormalizeEmail(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || len(trimmed) > 320 {
		return "", ErrInvalidEmail
	}

	address, err := mail.ParseAddress(trimmed)
	if err != nil || address.Address != trimmed {
		return "", ErrInvalidEmail
	}

	return strings.ToLower(address.Address), nil
}

func HashSessionToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func hashLoginCode(email string, code string) string {
	sum := sha256.Sum256([]byte(email + "\x00" + code))
	return hex.EncodeToString(sum[:])
}

func validCodeFormat(code string) bool {
	if len(code) != 6 {
		return false
	}
	for _, char := range code {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func generateNumericCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

func generateSessionToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
