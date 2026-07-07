package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func registerV1Routes(v1 *gin.RouterGroup, authHandler *authHandler) {
	if authHandler != nil {
		v1.POST("/auth/email-code", authHandler.requestEmailCode)
		v1.POST("/auth/verify", authHandler.verify)
		v1.POST("/auth/logout", authHandler.logout)
		v1.GET("/auth/me", authHandler.me)
		v1.DELETE("/account", authHandler.deleteAccount)
	} else {
		v1.POST("/auth/email-code", notImplementedHandler("email code login is not implemented"))
		v1.POST("/auth/verify", notImplementedHandler("email verification is not implemented"))
		v1.POST("/auth/logout", notImplementedHandler("logout is not implemented"))
		v1.GET("/auth/me", notImplementedHandler("current user is not implemented"))
		v1.DELETE("/account", notImplementedHandler("account deletion is not implemented"))
	}

	v1.POST("/resumes", notImplementedHandler("resume upload is not implemented"))
	v1.GET("/resumes/:id/status", notImplementedHandler("resume status is not implemented"))
	v1.DELETE("/resumes/:id", notImplementedHandler("resume deletion is not implemented"))
	v1.POST("/resumes/:id/generate-profile", notImplementedHandler("profile generation is not implemented"))

	v1.GET("/profile", notImplementedHandler("profile retrieval is not implemented"))
	v1.PATCH("/profile", notImplementedHandler("profile update is not implemented"))
	v1.POST("/profile/sections", notImplementedHandler("profile section creation is not implemented"))
	v1.PATCH("/profile/sections/:id", notImplementedHandler("profile section update is not implemented"))
	v1.DELETE("/profile/sections/:id", notImplementedHandler("profile section deletion is not implemented"))
	v1.POST("/profile/sections/reorder", notImplementedHandler("profile section reorder is not implemented"))
	v1.POST("/profile/sections/:id/rewrite", notImplementedHandler("profile section rewrite is not implemented"))

	v1.GET("/domains/check", notImplementedHandler("domain availability check is not implemented"))
	v1.PUT("/profile/domain", notImplementedHandler("profile domain update is not implemented"))
	v1.POST("/profile/publish", notImplementedHandler("profile publishing is not implemented"))
	v1.POST("/profile/unpublish", notImplementedHandler("profile unpublishing is not implemented"))
	v1.GET("/profile/preview", notImplementedHandler("profile preview is not implemented"))

	v1.GET("/profile/analytics", notImplementedHandler("profile analytics is not implemented"))
}

func notImplementedHandler(message string) gin.HandlerFunc {
	return func(c *gin.Context) {
		AbortWithError(c, http.StatusNotImplemented, ErrCodeNotImplemented, message)
	}
}
