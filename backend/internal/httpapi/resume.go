package httpapi

import (
	"errors"
	"net/http"

	authdomain "github.com/YankongLi/talent-profile/backend/internal/auth"
	"github.com/YankongLi/talent-profile/backend/internal/config"
	resumedomain "github.com/YankongLi/talent-profile/backend/internal/resume"
	"github.com/gin-gonic/gin"
)

type resumeHandler struct {
	cfg           config.Config
	authService   *authdomain.Service
	resumeService *resumedomain.Service
}

type resumeResponse struct {
	Resume resumedomain.Resume `json:"resume"`
}

type resumeStatusResponse struct {
	ID               string `json:"id"`
	OriginalFilename string `json:"original_filename"`
	MimeType         string `json:"mime_type"`
	FileSize         int64  `json:"file_size"`
	ParseStatus      string `json:"parse_status"`
	ParseError       string `json:"parse_error,omitempty"`
}

func newResumeHandler(cfg config.Config, authService *authdomain.Service, resumeService *resumedomain.Service) *resumeHandler {
	if authService == nil || resumeService == nil {
		return nil
	}
	return &resumeHandler{
		cfg:           cfg,
		authService:   authService,
		resumeService: resumeService,
	}
}

func (h *resumeHandler) upload(c *gin.Context) {
	user, ok := h.currentUser(c)
	if !ok {
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, resumedomain.MaxUploadBytes+(1<<20))
	fileHeader, err := c.FormFile("file")
	if err != nil {
		AbortWithError(c, http.StatusBadRequest, ErrCodeBadRequest, "resume file is required")
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		AbortWithError(c, http.StatusBadRequest, ErrCodeBadRequest, "resume file is invalid")
		return
	}
	defer file.Close()

	resume, err := h.resumeService.Upload(c.Request.Context(), user.ID, resumedomain.UploadInput{
		OriginalFilename: fileHeader.Filename,
		ContentType:      fileHeader.Header.Get("Content-Type"),
		Reader:           file,
	})
	if err != nil {
		h.abortResumeError(c, err)
		return
	}

	c.JSON(http.StatusCreated, resumeResponse{Resume: resume})
}

func (h *resumeHandler) status(c *gin.Context) {
	user, ok := h.currentUser(c)
	if !ok {
		return
	}

	resume, err := h.resumeService.Status(c.Request.Context(), user.ID, c.Param("id"))
	if err != nil {
		h.abortResumeError(c, err)
		return
	}

	c.JSON(http.StatusOK, resumeStatusResponse{
		ID:               resume.ID,
		OriginalFilename: resume.OriginalFilename,
		MimeType:         resume.MimeType,
		FileSize:         resume.FileSize,
		ParseStatus:      resume.ParseStatus,
		ParseError:       resume.ParseError,
	})
}

func (h *resumeHandler) delete(c *gin.Context) {
	user, ok := h.currentUser(c)
	if !ok {
		return
	}

	if err := h.resumeService.Delete(c.Request.Context(), user.ID, c.Param("id")); err != nil {
		h.abortResumeError(c, err)
		return
	}

	c.JSON(http.StatusOK, okResponse{OK: true})
}

func (h *resumeHandler) currentUser(c *gin.Context) (authdomain.User, bool) {
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

func (h *resumeHandler) abortResumeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, resumedomain.ErrFileRequired):
		AbortWithError(c, http.StatusBadRequest, ErrCodeBadRequest, "resume file is required")
	case errors.Is(err, resumedomain.ErrFileTooLarge):
		AbortWithError(c, http.StatusRequestEntityTooLarge, ErrCodeBadRequest, "resume file must be 10MB or smaller")
	case errors.Is(err, resumedomain.ErrUnsupportedFileType),
		errors.Is(err, resumedomain.ErrInvalidFileHeader):
		AbortWithError(c, http.StatusBadRequest, ErrCodeBadRequest, "resume file must be a valid PDF or DOCX")
	case errors.Is(err, resumedomain.ErrInvalidUserID):
		AbortWithError(c, http.StatusUnauthorized, ErrCodeUnauthorized, "unauthorized")
	case errors.Is(err, resumedomain.ErrInvalidResumeID):
		AbortWithError(c, http.StatusBadRequest, ErrCodeBadRequest, "invalid resume id")
	case errors.Is(err, resumedomain.ErrNotFound):
		AbortWithError(c, http.StatusNotFound, ErrCodeNotFound, "resume not found")
	default:
		AbortWithError(c, http.StatusInternalServerError, ErrCodeInternal, "internal server error")
	}
}
