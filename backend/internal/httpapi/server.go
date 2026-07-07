package httpapi

import (
	"net/http"

	"github.com/YankongLi/talent-profile/backend/internal/config"
	"github.com/gin-gonic/gin"
)

func NewServer(cfg config.Config) *http.Server {
	return &http.Server{
		Addr:         cfg.HTTP.Addr,
		Handler:      NewRouter(cfg),
		ReadTimeout:  cfg.HTTP.ReadTimeout,
		WriteTimeout: cfg.HTTP.WriteTimeout,
	}
}

func NewRouter(cfg config.Config) *gin.Engine {
	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.TestMode)
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
	registerV1Routes(v1)

	return router
}
