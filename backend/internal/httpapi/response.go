package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

var requestIDCounter uint64

const (
	ErrCodeBadRequest     = "bad_request"
	ErrCodeUnauthorized   = "unauthorized"
	ErrCodeForbidden      = "forbidden"
	ErrCodeConflict       = "conflict"
	ErrCodeInternal       = "internal_error"
	ErrCodeNotFound       = "not_found"
	ErrCodeMethodDenied   = "method_not_allowed"
	ErrCodeNotImplemented = "not_implemented"

	requestIDContextKey = "request_id"
)

type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

type ErrorDetail struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
}

func AbortWithError(c *gin.Context, status int, code string, message string) {
	if code == "" {
		code = ErrCodeInternal
	}
	if message == "" {
		message = http.StatusText(status)
	}

	c.AbortWithStatusJSON(status, ErrorResponse{
		Error: ErrorDetail{
			Code:      code,
			Message:   message,
			RequestID: requestID(c),
		},
	})
}

func requestID(c *gin.Context) string {
	if requestID, ok := c.Get(requestIDContextKey); ok {
		if value, ok := requestID.(string); ok {
			return value
		}
	}
	return c.GetHeader("X-Request-ID")
}

func requestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = newRequestID()
		}

		c.Set(requestIDContextKey, requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

func newRequestID() string {
	var randomBytes [12]byte
	if _, err := rand.Read(randomBytes[:]); err != nil {
		counter := atomic.AddUint64(&requestIDCounter, 1)
		return strconv.FormatInt(time.Now().UnixNano(), 36) + strconv.FormatUint(counter, 36)
	}
	return hex.EncodeToString(randomBytes[:])
}
