package routes

import (
	"time"

	"github.com/gin-gonic/gin"

	"sportss/internal/config"
	"sportss/internal/handlers"
	"sportss/internal/middleware"
)

// Register configura vistas, API, estáticos y grupos de permisos.
func Register(router *gin.Engine, cfg *config.Config, app *handlers.App) {
	router.Static("/static", cfg.StaticDir)
	router.GET("/healthz", app.Health)

	// Público.
	router.GET("/", app.Home)
	router.GET("/buscar", app.SearchCourts)
	router.GET("/canchas/:id", app.CourtDetail)
	router.GET("/login", app.LoginPage)
	router.GET("/registro", app.RegisterPage)
	router.GET("/api/courts", app.APICourts)
	router.GET("/api/courts/:id", app.APICourt)
	router.GET("/api/courts/:id/availability", app.APIAvailability)
	router.POST("/api/payments/webhook", app.PaymentWebhook)

	loginLimiter := middleware.LoginRateLimit(8, 15*time.Minute)
	router.POST("/login", loginLimiter, app.LoginSubmit)
	router.POST("/registro", loginLimiter, app.RegisterSubmit)
	router.POST("/api/auth/login", loginLimiter, app.LoginSubmit)
	router.POST("/api/auth/register", loginLimiter, app.RegisterSubmit)

	// Sesión y vistas privadas.
	auth := router.Group("/")
	auth.Use(middleware.RequireAuth(cfg), middleware.NoCache())
	{
		auth.POST("/logout", app.Logout)
		auth.GET("/perfil", app.Profile)
		auth.POST("/perfil", app.UpdateProfile)
		auth.POST("/perfil/password", app.ChangePassword)
		auth.GET("/mis-reservas", app.MyBookings)
		auth.GET("/facturas/:id/descargar", app.DownloadInvoice)
		auth.GET("/checkout/mock/:id", app.MockCheckout)
		auth.POST("/checkout/mock/:id/confirmar", app.MockConfirm)
		auth.GET("/pago/exito", app.PaymentSuccess)
		auth.GET("/pago/pendiente", app.PaymentPending)
		auth.GET("/pago/fallo", app.PaymentFailure)
	}

	// API autenticada.
	api := router.Group("/api")
	api.Use(middleware.RequireAuth(cfg), middleware.NoCache())
	{
		api.POST("/auth/logout", app.Logout)
		api.POST("/bookings", app.APICreateBooking)
		api.GET("/bookings/:id", app.APIGetBooking)
		api.POST("/bookings/:id/cancel", app.APICancelBooking)
		api.POST("/bookings/:id/complete", middleware.RequireBusiness(), app.APICompleteBooking)
		api.POST("/payments/checkout/:id", app.APICheckout)
		api.GET("/notifications", app.APINotifications)
		api.GET("/notifications/stream", app.NotificationStream)
		api.POST("/notifications/:id/read", app.APIMarkNotificationRead)
		api.POST("/notifications/read-all", app.APIMarkAllRead)
		api.POST("/reviews", app.APICreateReview)
		api.POST("/reviews/:id/respond", middleware.RequireBusiness(), app.APIRespondReview)
	}

	// Panel del dueño.
	panel := router.Group("/panel")
	panel.Use(middleware.RequireBusiness(), middleware.NoCache())
	{
		panel.GET("", app.Dashboard)
		panel.GET("/canchas", app.PanelCourts)
		panel.GET("/canchas/nueva", app.NewCourtForm)
		panel.POST("/canchas", app.SaveCourt)
		panel.GET("/canchas/:id/editar", app.EditCourtForm)
		panel.POST("/canchas/:id", app.SaveCourt)
		panel.POST("/canchas/:id/estado", app.ToggleCourtStatus)
		panel.POST("/canchas/:id/eliminar", app.DeleteCourt)
		panel.GET("/reservas", app.OwnerBookings)
		panel.POST("/reservas/:id/cancelar", app.OwnerCancelBooking)
		panel.POST("/reservas/:id/completar", app.OwnerCompleteBooking)
		panel.GET("/resenas", app.OwnerReviews)
		panel.POST("/resenas/:id/responder", app.OwnerRespondReview)
	}

	// Super administrador.
	admin := router.Group("/admin")
	admin.Use(middleware.RequireAdmin(), middleware.NoCache())
	{
		admin.GET("/usuarios", app.AdminUsers)
		admin.POST("/usuarios/:id/estado", app.AdminToggleUser)
		admin.GET("/canchas", app.AdminCourts)
		admin.POST("/canchas/:id/estado", app.ToggleCourtStatus)
		admin.POST("/canchas/:id/eliminar", app.DeleteCourt)
		admin.GET("/resenas", app.AdminReviews)
		admin.POST("/resenas/:id/ocultar", app.AdminToggleReview)
		admin.GET("/reportes", app.AdminReports)
	}

	router.NoRoute(app.NotFound)
}
