package routes

import (
	"context"
	"time"

	"github.com/grigta/conveer/pkg/config"
	"github.com/grigta/conveer/pkg/middleware"
	"github.com/grigta/conveer/services/api-gateway/internal/handlers"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func SetupRoutes(router *gin.Engine, h *handlers.Handlers, cfg *config.Config) {
	corsConfig := middleware.DefaultCORSConfig()
	router.Use(middleware.CORS(corsConfig))

	router.Use(gin.LoggerWithConfig(gin.LoggerConfig{
		SkipPaths: []string{"/health", "/metrics"},
	}))

	if cfg.RateLimit.Enabled {
		rateLimiter := middleware.NewRateLimiter(cfg.RateLimit.Requests, cfg.RateLimit.Window)
		router.Use(rateLimiter.Middleware())
	}

	router.Use(requestTimeout(30 * time.Second))

	router.GET("/health", h.HealthCheck)
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	api := router.Group("/api/v1")
	{
		auth := api.Group("/auth")
		{
			auth.POST("/register", h.AuthProxy)
			auth.POST("/login", h.AuthProxy)
			auth.POST("/logout", h.AuthProxy)
			auth.POST("/refresh", h.AuthProxy)
			auth.POST("/forgot-password", h.AuthProxy)
			auth.POST("/reset-password", h.AuthProxy)
			auth.POST("/verify-email", h.AuthProxy)
		}

		authMiddleware := middleware.NewAuthMiddleware(cfg.JWT.Secret)

		users := api.Group("/users")
		users.Use(authMiddleware.Authenticate())
		{
			users.GET("", h.UserProxy)
			users.GET("/:id", h.UserProxy)
			users.GET("/profile", h.UserProxy)
			users.PUT("/profile", h.UserProxy)
			users.DELETE("/:id", h.UserProxy)
			users.PUT("/:id/password", h.UserProxy)
			users.POST("/:id/avatar", h.UserProxy)
		}

		products := api.Group("/products")
		{
			products.GET("", h.ProductProxy)
			products.GET("/:id", h.ProductProxy)
			products.GET("/search", h.ProductProxy)
			products.GET("/categories", h.ProductProxy)

			productsAuth := products.Group("")
			productsAuth.Use(authMiddleware.Authenticate())
			{
				productsAuth.POST("", authMiddleware.RequireRole("admin", "moderator"), h.ProductProxy)
				productsAuth.PUT("/:id", authMiddleware.RequireRole("admin", "moderator"), h.ProductProxy)
				productsAuth.DELETE("/:id", authMiddleware.RequireRole("admin"), h.ProductProxy)
				productsAuth.POST("/:id/reviews", h.ProductProxy)
			}
		}

		orders := api.Group("/orders")
		orders.Use(authMiddleware.Authenticate())
		{
			orders.GET("", h.OrderProxy)
			orders.GET("/:id", h.OrderProxy)
			orders.POST("", h.OrderProxy)
			orders.PUT("/:id", h.OrderProxy)
			orders.DELETE("/:id", h.OrderProxy)
			orders.POST("/:id/cancel", h.OrderProxy)
			orders.POST("/:id/confirm", h.OrderProxy)
			orders.GET("/:id/track", h.OrderProxy)
		}

		notifications := api.Group("/notifications")
		notifications.Use(authMiddleware.Authenticate())
		{
			notifications.GET("", h.NotificationProxy)
			notifications.GET("/:id", h.NotificationProxy)
			notifications.PUT("/:id/read", h.NotificationProxy)
			notifications.PUT("/read-all", h.NotificationProxy)
			notifications.DELETE("/:id", h.NotificationProxy)
			notifications.GET("/preferences", h.NotificationProxy)
			notifications.PUT("/preferences", h.NotificationProxy)
		}

		analytics := api.Group("/analytics")
		analytics.Use(authMiddleware.Authenticate())
		analytics.Use(authMiddleware.RequireRole("admin", "moderator"))
		{
			analytics.GET("/dashboard", h.AnalyticsProxy)
			analytics.GET("/reports/sales", h.AnalyticsProxy)
			analytics.GET("/reports/users", h.AnalyticsProxy)
			analytics.GET("/reports/products", h.AnalyticsProxy)
			analytics.GET("/reports/revenue", h.AnalyticsProxy)
			analytics.POST("/reports/custom", h.AnalyticsProxy)
		}

		proxies := api.Group("/proxies")
		proxies.Use(authMiddleware.Authenticate())
		{
			proxies.POST("/allocate", h.ProxyProxy)
			proxies.POST("/release", h.ProxyProxy)
			proxies.GET("/:id", h.ProxyProxy)
			proxies.GET("/account/:account_id", h.ProxyProxy)
			proxies.GET("/health/:id", h.ProxyProxy)
			proxies.POST("/:id/rotate", h.ProxyProxy)
			proxies.GET("/statistics", h.ProxyProxy)
		}

		providers := api.Group("/providers")
		providers.Use(authMiddleware.Authenticate())
		providers.Use(authMiddleware.RequireRole("admin", "moderator"))
		{
			providers.GET("", h.ProxyProxy)
		}

		sms := api.Group("/sms")
		sms.Use(authMiddleware.Authenticate())
		{
			sms.POST("/purchase", h.SMSProxy)
			sms.GET("/code/:activation_id", h.SMSProxy)
			sms.POST("/cancel/:activation_id", h.SMSProxy)
			sms.GET("/status/:activation_id", h.SMSProxy)
			sms.GET("/statistics", h.SMSProxy)
			sms.GET("/balance", h.SMSProxy)
		}

		vk := api.Group("/vk")
		vk.Use(authMiddleware.Authenticate())
		{
			vk.POST("/accounts", h.VKProxy)
			vk.GET("/accounts", h.VKProxy)
			vk.GET("/accounts/:id", h.VKProxy)
			vk.PUT("/accounts/:id/status", h.VKProxy)
			vk.POST("/accounts/:id/retry", h.VKProxy)
			vk.DELETE("/accounts/:id", h.VKProxy)
			vk.GET("/statistics", h.VKProxy)
		}

		telegram := api.Group("/telegram")
		telegram.Use(authMiddleware.Authenticate())
		{
			telegram.POST("/accounts", h.TelegramProxy)
			telegram.GET("/accounts", h.TelegramProxy)
			telegram.GET("/accounts/:id", h.TelegramProxy)
			telegram.PUT("/accounts/:id/status", h.TelegramProxy)
			telegram.POST("/accounts/:id/retry", h.TelegramProxy)
			telegram.DELETE("/accounts/:id", h.TelegramProxy)
			telegram.GET("/statistics", h.TelegramProxy)
		}

		mail := api.Group("/mail")
		mail.Use(authMiddleware.Authenticate())
		{
			mail.POST("/accounts", h.MailProxy)
			mail.GET("/accounts", h.MailProxy)
			mail.GET("/accounts/:id", h.MailProxy)
			mail.PUT("/accounts/:id/status", h.MailProxy)
			mail.POST("/accounts/:id/retry", h.MailProxy)
			mail.DELETE("/accounts/:id", h.MailProxy)
			mail.GET("/statistics", h.MailProxy)
		}

		max := api.Group("/max")
		max.Use(authMiddleware.Authenticate())
		{
			max.POST("/accounts", h.MaxProxy)
			max.GET("/accounts", h.MaxProxy)
			max.GET("/accounts/:id", h.MaxProxy)
			max.PUT("/accounts/:id/status", h.MaxProxy)
			max.POST("/accounts/:id/retry", h.MaxProxy)
			max.POST("/accounts/:id/link-vk", h.MaxProxy)
			max.DELETE("/accounts/:id", h.MaxProxy)
			max.GET("/statistics", h.MaxProxy)
		}

		warming := api.Group("/warming")
		warming.Use(authMiddleware.Authenticate())
		{
			warming.POST("/start", h.WarmingProxy)
			warming.POST("/:taskId/pause", h.WarmingProxy)
			warming.POST("/:taskId/resume", h.WarmingProxy)
			warming.POST("/:taskId/stop", h.WarmingProxy)
			warming.GET("/:taskId", h.WarmingProxy)
			warming.GET("/statistics", h.WarmingProxy)
			warming.POST("/scenarios", h.WarmingProxy)
			warming.PUT("/scenarios/:scenarioId", h.WarmingProxy)
			warming.GET("/scenarios", h.WarmingProxy)
			warming.GET("/tasks", h.WarmingProxy)
		}

		admin := api.Group("/admin")
		admin.Use(authMiddleware.Authenticate())
		admin.Use(authMiddleware.RequireRole("admin"))
		{
			admin.GET("/users", h.UserProxy)
			admin.PUT("/users/:id/role", h.UserProxy)
			admin.PUT("/users/:id/status", h.UserProxy)
			admin.GET("/system/info", h.HealthCheck)
			admin.GET("/system/config", h.HealthCheck)
			admin.POST("/system/cache/clear", h.HealthCheck)
		}
	}

	router.NoRoute(h.NotFound)
	router.NoMethod(h.MethodNotAllowed)
}

func requestTimeout(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Create a new context with deadline
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()

		// Replace request context with the one that has timeout
		c.Request = c.Request.WithContext(ctx)

		// Continue with the next handlers
		c.Next()

		// Check if context was cancelled due to timeout
		if ctx.Err() == context.DeadlineExceeded {
			c.AbortWithStatusJSON(504, gin.H{
				"error": "Request timeout",
				"message": "The server did not receive a timely response from the upstream server",
			})
		}
	}
}
