package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/YankongLi/talent-profile/backend/internal/auth"
	"github.com/YankongLi/talent-profile/backend/internal/config"
	"github.com/gin-gonic/gin"
)

type authHandler struct {
	cfg     config.Config
	service *auth.Service
}

type emailCodeRequest struct {
	Email string `json:"email"`
}

type emailCodeResponse struct {
	OK               bool      `json:"ok"`
	Email            string    `json:"email"`
	ExpiresAt        time.Time `json:"expires_at"`
	ExpiresInSeconds int       `json:"expires_in_seconds"`
	DebugCode        string    `json:"debug_code,omitempty"`
}

type verifyRequest struct {
	Email string `json:"email"`
	Code  string `json:"code"`
}

type verifyResponse struct {
	Token     string    `json:"token"`
	TokenType string    `json:"token_type"`
	ExpiresAt time.Time `json:"expires_at"`
	User      auth.User `json:"user"`
}

type meResponse struct {
	User auth.User `json:"user"`
}

type okResponse struct {
	OK bool `json:"ok"`
}

func newAuthHandler(cfg config.Config, service *auth.Service) *authHandler {
	if service == nil {
		return nil
	}
	return &authHandler{
		cfg:     cfg,
		service: service,
	}
}

func (h *authHandler) requestEmailCode(c *gin.Context) {
	var req emailCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		AbortWithError(c, http.StatusBadRequest, ErrCodeBadRequest, "invalid request body")
		return
	}

	result, err := h.service.RequestEmailCode(c.Request.Context(), req.Email)
	if err != nil {
		h.abortAuthError(c, err)
		return
	}

	c.JSON(http.StatusOK, emailCodeResponse{
		OK:               true,
		Email:            result.Email,
		ExpiresAt:        result.ExpiresAt,
		ExpiresInSeconds: result.ExpiresInSeconds,
		DebugCode:        result.DebugCode,
	})
}

func (h *authHandler) verify(c *gin.Context) {
	var req verifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		AbortWithError(c, http.StatusBadRequest, ErrCodeBadRequest, "invalid request body")
		return
	}

	result, err := h.service.VerifyEmailCode(c.Request.Context(), req.Email, req.Code)
	if err != nil {
		h.abortAuthError(c, err)
		return
	}

	h.setSessionCookie(c, result.Token, result.ExpiresAt)
	c.JSON(http.StatusOK, verifyResponse{
		Token:     result.Token,
		TokenType: "Bearer",
		ExpiresAt: result.ExpiresAt,
		User:      result.User,
	})
}

func (h *authHandler) logout(c *gin.Context) {
	token := h.sessionToken(c)
	if token == "" {
		AbortWithError(c, http.StatusUnauthorized, ErrCodeUnauthorized, "missing session token")
		return
	}

	if err := h.service.Logout(c.Request.Context(), token); err != nil {
		h.abortAuthError(c, err)
		return
	}

	h.clearSessionCookie(c)
	c.JSON(http.StatusOK, okResponse{OK: true})
}

func (h *authHandler) me(c *gin.Context) {
	token := h.sessionToken(c)
	if token == "" {
		AbortWithError(c, http.StatusUnauthorized, ErrCodeUnauthorized, "missing session token")
		return
	}

	user, err := h.service.CurrentUser(c.Request.Context(), token)
	if err != nil {
		h.abortAuthError(c, err)
		return
	}

	c.JSON(http.StatusOK, meResponse{User: user})
}

func (h *authHandler) deleteAccount(c *gin.Context) {
	token := h.sessionToken(c)
	if token == "" {
		AbortWithError(c, http.StatusUnauthorized, ErrCodeUnauthorized, "missing session token")
		return
	}

	if err := h.service.DeleteAccount(c.Request.Context(), token); err != nil {
		h.abortAuthError(c, err)
		return
	}

	h.clearSessionCookie(c)
	c.JSON(http.StatusOK, okResponse{OK: true})
}

func (h *authHandler) abortAuthError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, auth.ErrInvalidEmail):
		AbortWithError(c, http.StatusBadRequest, ErrCodeBadRequest, "invalid email")
	case errors.Is(err, auth.ErrInvalidCode):
		AbortWithError(c, http.StatusUnauthorized, ErrCodeUnauthorized, "invalid email code")
	case errors.Is(err, auth.ErrSessionTokenEmpty):
		AbortWithError(c, http.StatusUnauthorized, ErrCodeUnauthorized, "missing session token")
	case errors.Is(err, auth.ErrUnauthorized):
		AbortWithError(c, http.StatusUnauthorized, ErrCodeUnauthorized, "unauthorized")
	default:
		AbortWithError(c, http.StatusInternalServerError, ErrCodeInternal, "internal server error")
	}
}

func (h *authHandler) setSessionCookie(c *gin.Context, token string, expiresAt time.Time) {
	maxAge := int(time.Until(expiresAt).Seconds())
	if maxAge < 0 {
		maxAge = 0
	}
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(h.cfg.Auth.CookieName, token, maxAge, "/", "", h.cfg.IsProduction(), true)
}

func (h *authHandler) clearSessionCookie(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(h.cfg.Auth.CookieName, "", -1, "/", "", h.cfg.IsProduction(), true)
}

func (h *authHandler) sessionToken(c *gin.Context) string {
	authHeader := strings.TrimSpace(c.GetHeader("Authorization"))
	if authHeader != "" {
		parts := strings.Fields(authHeader)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			return parts[1]
		}
	}

	token, err := c.Cookie(h.cfg.Auth.CookieName)
	if err != nil {
		return ""
	}
	return token
}
