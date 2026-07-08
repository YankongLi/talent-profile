package httpapi

import (
	"errors"
	"net/http"

	authdomain "github.com/YankongLi/talent-profile/backend/internal/auth"
	"github.com/YankongLi/talent-profile/backend/internal/config"
	"github.com/YankongLi/talent-profile/backend/internal/publishing"
	"github.com/gin-gonic/gin"
)

type domainHandler struct {
	cfg               config.Config
	authService       *authdomain.Service
	publishingService *publishing.Service
}

type updateDomainRequest struct {
	Slug string `json:"slug"`
}

type checkDomainResponse struct {
	Slug      string `json:"slug"`
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
}

type updateDomainResponse struct {
	Domain         publishing.Domain  `json:"domain"`
	PreviousDomain *publishing.Domain `json:"previous_domain,omitempty"`
}

func newDomainHandler(cfg config.Config, authService *authdomain.Service, publishingService *publishing.Service) *domainHandler {
	if publishingService == nil {
		return nil
	}
	return &domainHandler{
		cfg:               cfg,
		authService:       authService,
		publishingService: publishingService,
	}
}

func (h *domainHandler) canUpdate() bool {
	return h.authService != nil
}

func (h *domainHandler) check(c *gin.Context) {
	result, err := h.publishingService.CheckSlug(c.Request.Context(), c.Query("slug"))
	if err != nil {
		AbortWithError(c, http.StatusInternalServerError, ErrCodeInternal, "internal server error")
		return
	}

	c.JSON(http.StatusOK, checkDomainResponse{
		Slug:      result.Slug,
		Available: result.Available,
		Reason:    result.Reason,
	})
}

func (h *domainHandler) update(c *gin.Context) {
	user, ok := h.currentUser(c)
	if !ok {
		return
	}

	var req updateDomainRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		AbortWithError(c, http.StatusBadRequest, ErrCodeBadRequest, "invalid request body")
		return
	}

	result, err := h.publishingService.SetPrimaryDomain(c.Request.Context(), user.ID, req.Slug)
	if err != nil {
		abortPublishingError(c, err)
		return
	}

	c.JSON(http.StatusOK, updateDomainResponse{
		Domain:         result.Domain,
		PreviousDomain: result.PreviousDomain,
	})
}

func (h *domainHandler) currentUser(c *gin.Context) (authdomain.User, bool) {
	token := sessionToken(c, h.cfg.Auth.CookieName)
	if token == "" {
		AbortWithError(c, http.StatusUnauthorized, ErrCodeUnauthorized, "missing session token")
		return authdomain.User{}, false
	}

	user, err := h.authService.CurrentUser(c.Request.Context(), token)
	if err != nil {
		abortAuthError(c, err)
		return authdomain.User{}, false
	}
	return user, true
}

func abortPublishingError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, publishing.ErrSlugRequired),
		errors.Is(err, publishing.ErrSlugTooShort),
		errors.Is(err, publishing.ErrSlugTooLong),
		errors.Is(err, publishing.ErrSlugInvalidChar),
		errors.Is(err, publishing.ErrSlugInvalidHyphen),
		errors.Is(err, publishing.ErrSlugReserved):
		AbortWithError(c, http.StatusBadRequest, ErrCodeBadRequest, "invalid slug")
	case errors.Is(err, publishing.ErrSlugTaken):
		AbortWithError(c, http.StatusConflict, ErrCodeConflict, "slug is already taken")
	default:
		AbortWithError(c, http.StatusInternalServerError, ErrCodeInternal, "internal server error")
	}
}
