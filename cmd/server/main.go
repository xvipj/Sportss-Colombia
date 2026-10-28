package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"sportss/internal/cache"
	"sportss/internal/config"
	"sportss/internal/database"
	"sportss/internal/handlers"
	"sportss/internal/middleware"
	"sportss/internal/payments"
	"sportss/internal/repositories"
	"sportss/internal/routes"
	"sportss/internal/services"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("configuración inválida", "error", err)
		os.Exit(1)
	}
	setupLogger(cfg)

	db, err := database.Connect(cfg.SQLitePath, cfg.AppEnv)
	if err != nil {
		slog.Error("no se pudo iniciar la base de datos", "error", err)
		os.Exit(1)
	}
	if err := database.Seed(db, cfg.Location); err != nil {
		slog.Error("no se pudo crear el seed", "error", err)
		os.Exit(1)
	}

	cacheStore, cacheErr := cache.New(context.Background(), cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	if cacheErr != nil {
		slog.Warn("Redis no disponible; usando caché y locks en memoria (solo una instancia)",
			"addr", cfg.RedisAddr, "error", cacheErr)
	} else {
		slog.Info("Redis conectado", "addr", cfg.RedisAddr, "modo", cacheStore.Name())
	}
	defer cacheStore.Close()

	userRepo := repositories.NewUserRepository(db)
	courtRepo := repositories.NewCourtRepository(db)
	bookingRepo := repositories.NewBookingRepository(db)
	paymentRepo := repositories.NewPaymentRepository(db)
	invoiceRepo := repositories.NewInvoiceRepository(db)
	reviewRepo := repositories.NewReviewRepository(db)
	notificationRepo := repositories.NewNotificationRepository(db)
	analyticsRepo := repositories.NewAnalyticsRepository(db)

	notifier := services.NewNotificationService(notificationRepo)
	authService := services.NewAuthService(cfg, userRepo)
	courtService := services.NewCourtService(cfg, courtRepo, bookingRepo, cacheStore)
	bookingService := services.NewBookingService(cfg, bookingRepo, courtRepo, cacheStore, notifier)
	invoiceService := services.NewInvoiceService(cfg, invoiceRepo, bookingRepo)
	mpClient := payments.NewClient(cfg.MPAccessToken, cfg.MPPublicKey, cfg.MPWebhookSecret, cfg.MPSandbox)
	paymentService := services.NewPaymentService(cfg, paymentRepo, bookingRepo, invoiceService, notifier, mpClient, cacheStore)
	reviewService := services.NewReviewService(reviewRepo, bookingRepo, courtRepo, notifier)
	analyticsService := services.NewAnalyticsService(cfg, analyticsRepo, courtRepo)

	app := handlers.NewApp(cfg, authService, courtService, bookingService, paymentService,
		invoiceService, reviewService, notifier, analyticsService)

	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()
	router.SetTrustedProxies(nil)
	router.Use(
		middleware.Recovery(),
		middleware.RequestLogger(),
		middleware.SecurityHeaders(cfg),
		middleware.BodyLimit(32<<20),
		middleware.Auth(cfg, authService),
		middleware.CSRF(cfg),
	)
	routes.Register(router, cfg, app)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	scheduler := services.NewScheduler(bookingService)
	go scheduler.Run(ctx)

	if cfg.PaymentMockEnabled() {
		slog.Warn("Mercado Pago sin credenciales: checkout en modo simulado habilitado")
	}
	slog.Info("Sportss Colombia iniciando",
		"url", cfg.BaseURL, "env", cfg.AppEnv, "timezone", cfg.Timezone,
		"comision", cfg.CommissionPercent, "pago_simulado", cfg.PaymentMockEnabled())

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      0, // SSE necesita conexiones de larga duración.
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("servidor detenido", "error", err)
			cancel()
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	slog.Info("apagando servidor...")
	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("error al apagar", "error", err)
	}
}

func setupLogger(cfg *config.Config) {
	level := slog.LevelInfo
	if cfg.AppEnv == "development" {
		level = slog.LevelDebug
	}
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	logger := slog.New(handler)
	slog.SetDefault(logger)
}
