package httpapi

import (
	"errors"
	"net/http"
	"strings"

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

type publicProfileResponse struct {
	Slug           string                        `json:"slug,omitempty"`
	RedirectToSlug string                        `json:"redirect_to_slug,omitempty"`
	Page           *publishing.PublicProfilePage `json:"page,omitempty"`
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

func (h *domainHandler) publicProfile(c *gin.Context) {
	slug := publicSlug(c)
	result, err := h.publishingService.GetPublicProfile(c.Request.Context(), slug)
	if err != nil {
		abortPublicProfileError(c, err)
		return
	}
	if result.Redirect != nil {
		c.JSON(http.StatusOK, publicProfileResponse{
			Slug:           result.Redirect.Slug,
			RedirectToSlug: result.Redirect.RedirectToSlug,
		})
		return
	}

	c.JSON(http.StatusOK, publicProfileResponse{Page: result.Page})
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

func abortPublicProfileError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, publishing.ErrPublicProfileNotFound):
		AbortWithError(c, http.StatusNotFound, ErrCodeNotFound, "public profile not found")
	default:
		AbortWithError(c, http.StatusInternalServerError, ErrCodeInternal, "internal server error")
	}
}

func publicSlug(c *gin.Context) string {
	if slug := strings.TrimSpace(c.Query("slug")); slug != "" {
		return slug
	}

	host := strings.TrimSpace(c.Request.Host)
	if host == "" {
		host = strings.TrimSpace(c.GetHeader("Host"))
	}
	if host == "" {
		return ""
	}
	if colon := strings.LastIndex(host, ":"); colon >= 0 {
		host = host[:colon]
	}
	parts := strings.Split(host, ".")
	if len(parts) < 3 {
		return ""
	}
	return parts[0]
}
