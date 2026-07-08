package httpapi

import (
	"net/http"

	"github.com/YankongLi/talent-profile/backend/internal/publishing"
	"github.com/gin-gonic/gin"
)

type domainHandler struct {
	publishingService *publishing.Service
}

type checkDomainResponse struct {
	Slug      string `json:"slug"`
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
}

func newDomainHandler(publishingService *publishing.Service) *domainHandler {
	if publishingService == nil {
		return nil
	}
	return &domainHandler{publishingService: publishingService}
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
