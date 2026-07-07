package httpapi

import (
	"errors"
	"net/http"

	authdomain "github.com/YankongLi/talent-profile/backend/internal/auth"
	"github.com/YankongLi/talent-profile/backend/internal/config"
	profiledomain "github.com/YankongLi/talent-profile/backend/internal/profile"
	"github.com/gin-gonic/gin"
)

type profileHandler struct {
	cfg            config.Config
	authService    *authdomain.Service
	profileService *profiledomain.Service
}

type profileResponse struct {
	Profile  profiledomain.Profile   `json:"profile"`
	Sections []profiledomain.Section `json:"sections"`
}

type updateProfileRequest struct {
	Headline    *string         `json:"headline"`
	Summary     *string         `json:"summary"`
	TargetRoles *[]string       `json:"target_roles"`
	Visibility  *string         `json:"visibility"`
	TemplateID  *string         `json:"template_id"`
	Theme       *map[string]any `json:"theme"`
}

type createSectionRequest struct {
	SectionType     string         `json:"section_type"`
	Content         map[string]any `json:"content"`
	SortOrder       *int           `json:"sort_order"`
	IsVisible       *bool          `json:"is_visible"`
	IsUserConfirmed *bool          `json:"is_user_confirmed"`
}

type updateSectionRequest struct {
	SectionType     *string         `json:"section_type"`
	Content         *map[string]any `json:"content"`
	SortOrder       *int            `json:"sort_order"`
	IsVisible       *bool           `json:"is_visible"`
	IsUserConfirmed *bool           `json:"is_user_confirmed"`
}

type sectionResponse struct {
	Section profiledomain.Section `json:"section"`
}

func newProfileHandler(cfg config.Config, authService *authdomain.Service, profileService *profiledomain.Service) *profileHandler {
	if authService == nil || profileService == nil {
		return nil
	}
	return &profileHandler{
		cfg:            cfg,
		authService:    authService,
		profileService: profileService,
	}
}

func (h *profileHandler) get(c *gin.Context) {
	user, ok := h.currentUser(c)
	if !ok {
		return
	}

	profile, err := h.profileService.Get(c.Request.Context(), user.ID)
	if err != nil {
		h.abortProfileError(c, err)
		return
	}
	sections, err := h.profileService.Sections(c.Request.Context(), user.ID)
	if err != nil {
		h.abortProfileError(c, err)
		return
	}

	c.JSON(http.StatusOK, profileResponse{Profile: profile, Sections: sections})
}

func (h *profileHandler) patch(c *gin.Context) {
	user, ok := h.currentUser(c)
	if !ok {
		return
	}

	var req updateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		AbortWithError(c, http.StatusBadRequest, ErrCodeBadRequest, "invalid request body")
		return
	}

	profile, err := h.profileService.Update(c.Request.Context(), user.ID, profiledomain.UpdateInput{
		Headline:    req.Headline,
		Summary:     req.Summary,
		TargetRoles: req.TargetRoles,
		Visibility:  req.Visibility,
		TemplateID:  req.TemplateID,
		Theme:       req.Theme,
	})
	if err != nil {
		h.abortProfileError(c, err)
		return
	}
	sections, err := h.profileService.Sections(c.Request.Context(), user.ID)
	if err != nil {
		h.abortProfileError(c, err)
		return
	}

	c.JSON(http.StatusOK, profileResponse{Profile: profile, Sections: sections})
}

func (h *profileHandler) createSection(c *gin.Context) {
	user, ok := h.currentUser(c)
	if !ok {
		return
	}

	var req createSectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		AbortWithError(c, http.StatusBadRequest, ErrCodeBadRequest, "invalid request body")
		return
	}

	section, err := h.profileService.CreateSection(c.Request.Context(), user.ID, profiledomain.CreateSectionInput{
		SectionType:     req.SectionType,
		Content:         req.Content,
		SortOrder:       req.SortOrder,
		IsVisible:       req.IsVisible,
		IsUserConfirmed: req.IsUserConfirmed,
	})
	if err != nil {
		h.abortProfileError(c, err)
		return
	}

	c.JSON(http.StatusCreated, sectionResponse{Section: section})
}

func (h *profileHandler) patchSection(c *gin.Context) {
	user, ok := h.currentUser(c)
	if !ok {
		return
	}

	var req updateSectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		AbortWithError(c, http.StatusBadRequest, ErrCodeBadRequest, "invalid request body")
		return
	}

	section, err := h.profileService.UpdateSection(c.Request.Context(), user.ID, c.Param("id"), profiledomain.UpdateSectionInput{
		SectionType:     req.SectionType,
		Content:         req.Content,
		SortOrder:       req.SortOrder,
		IsVisible:       req.IsVisible,
		IsUserConfirmed: req.IsUserConfirmed,
	})
	if err != nil {
		h.abortProfileError(c, err)
		return
	}

	c.JSON(http.StatusOK, sectionResponse{Section: section})
}

func (h *profileHandler) deleteSection(c *gin.Context) {
	user, ok := h.currentUser(c)
	if !ok {
		return
	}

	if err := h.profileService.DeleteSection(c.Request.Context(), user.ID, c.Param("id")); err != nil {
		h.abortProfileError(c, err)
		return
	}

	c.JSON(http.StatusOK, okResponse{OK: true})
}

func (h *profileHandler) currentUser(c *gin.Context) (authdomain.User, bool) {
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

func (h *profileHandler) abortProfileError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, profiledomain.ErrInvalidHeadline),
		errors.Is(err, profiledomain.ErrInvalidSummary),
		errors.Is(err, profiledomain.ErrInvalidTargetRole),
		errors.Is(err, profiledomain.ErrInvalidVisibility),
		errors.Is(err, profiledomain.ErrInvalidTemplateID),
		errors.Is(err, profiledomain.ErrInvalidTheme),
		errors.Is(err, profiledomain.ErrInvalidSectionID),
		errors.Is(err, profiledomain.ErrInvalidSection),
		errors.Is(err, profiledomain.ErrInvalidSortOrder):
		AbortWithError(c, http.StatusBadRequest, ErrCodeBadRequest, "invalid profile")
	case errors.Is(err, profiledomain.ErrNotFound):
		AbortWithError(c, http.StatusNotFound, ErrCodeNotFound, "profile not found")
	default:
		AbortWithError(c, http.StatusInternalServerError, ErrCodeInternal, "internal server error")
	}
}
