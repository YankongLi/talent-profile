package httpapi

import (
	"database/sql"
	"log/slog"
	"net/http"

	"github.com/YankongLi/talent-profile/backend/internal/auth"
	"github.com/YankongLi/talent-profile/backend/internal/config"
	profiledomain "github.com/YankongLi/talent-profile/backend/internal/profile"
	"github.com/gin-gonic/gin"
)

type RouterOption func(*routerOptions)

type routerOptions struct {
	db             *sql.DB
	authService    *auth.Service
	profileService *profiledomain.Service
	logger         *slog.Logger
}

func WithDatabase(db *sql.DB) RouterOption {
	return func(opts *routerOptions) {
		opts.db = db
	}
}

func WithAuthService(service *auth.Service) RouterOption {
	return func(opts *routerOptions) {
		opts.authService = service
	}
}

func WithProfileService(service *profiledomain.Service) RouterOption {
	return func(opts *routerOptions) {
		opts.profileService = service
	}
}

func WithLogger(logger *slog.Logger) RouterOption {
	return func(opts *routerOptions) {
		opts.logger = logger
	}
}

func NewServer(cfg config.Config, options ...RouterOption) *http.Server {
	return &http.Server{
		Addr:         cfg.HTTP.Addr,
		Handler:      NewRouter(cfg, options...),
		ReadTimeout:  cfg.HTTP.ReadTimeout,
		WriteTimeout: cfg.HTTP.WriteTimeout,
	}
}

func NewRouter(cfg config.Config, options ...RouterOption) *gin.Engine {
	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.TestMode)
	}

	opts := routerOptions{}
	for _, apply := range options {
		apply(&opts)
	}
	authService := opts.authService
	if authService == nil && opts.db != nil {
		authService = auth.NewService(
			auth.NewPostgresStore(opts.db),
			auth.LogEmailCodeSender{
				Logger:      opts.logger,
				IncludeCode: !cfg.IsProduction(),
			},
			auth.Config{
				CodeTTL:         cfg.Auth.CodeTTL,
				SessionTTL:      cfg.Auth.SessionTTL,
				ExposeDebugCode: !cfg.IsProduction(),
			},
		)
	}
	profileService := opts.profileService
	if profileService == nil && opts.db != nil {
		profileService = profiledomain.NewService(profiledomain.NewPostgresStore(opts.db))
	}

	router := gin.New()
	router.HandleMethodNotAllowed = true
	router.Use(
		requestIDMiddleware(),
		securityHeadersMiddleware(),
		gin.CustomRecovery(func(c *gin.Context, recovered any) {
			AbortWithError(c, http.StatusInternalServerError, ErrCodeInternal, "internal server error")
		}),
	)

	router.NoRoute(func(c *gin.Context) {
		AbortWithError(c, http.StatusNotFound, ErrCodeNotFound, "resource not found")
	})
	router.NoMethod(func(c *gin.Context) {
		AbortWithError(c, http.StatusMethodNotAllowed, ErrCodeMethodDenied, "method not allowed")
	})

	router.GET("/healthz", healthHandler(cfg.AppName, cfg.Env))
	router.GET("/readyz", healthHandler(cfg.AppName, cfg.Env))

	v1 := router.Group("/api/v1")
	v1.GET("/health", healthHandler(cfg.AppName, cfg.Env))
	registerV1Routes(v1, newAuthHandler(cfg, authService), newProfileHandler(cfg, authService, profileService))

	return router
}
