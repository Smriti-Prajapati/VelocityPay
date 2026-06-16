package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/velocitypay/velocitypay/internal/analytics"
	"github.com/velocitypay/velocitypay/internal/audit"
	"github.com/velocitypay/velocitypay/internal/auth"
	"github.com/velocitypay/velocitypay/internal/config"
	"github.com/velocitypay/velocitypay/internal/database"
	"github.com/velocitypay/velocitypay/internal/fraud"
	"github.com/velocitypay/velocitypay/internal/metrics"
	"github.com/velocitypay/velocitypay/internal/middleware"
	"github.com/velocitypay/velocitypay/internal/notification"
	"github.com/velocitypay/velocitypay/internal/rabbitmq"
	redisclient "github.com/velocitypay/velocitypay/internal/redis"
	"github.com/velocitypay/velocitypay/internal/refund"
	"github.com/velocitypay/velocitypay/internal/transaction"
	"github.com/velocitypay/velocitypay/internal/users"
	"github.com/velocitypay/velocitypay/internal/wallet"
	"github.com/velocitypay/velocitypay/pkg/logger"
	"go.uber.org/zap"
)

func main() {
	// ── Configuration ─────────────────────────────────────────────────────────
	cfg, err := config.Load()
	if err != nil {
		panic("failed to load config: " + err.Error())
	}

	// ── Logger ────────────────────────────────────────────────────────────────
	log := logger.Must(logger.New(cfg.App.LogLevel, cfg.App.Env == "development"))
	defer log.Sync() //nolint:errcheck

	log.Info("starting VelocityPay",
		zap.String("env", cfg.App.Env),
		zap.String("port", cfg.App.Port),
	)

	// ── Database ──────────────────────────────────────────────────────────────
	db, err := database.NewPostgresDB(&cfg.Database, log)
	if err != nil {
		log.Fatal("postgres connection failed", zap.Error(err))
	}
	if err := database.RunMigrations(db, "file://migrations", log); err != nil {
		log.Fatal("migrations failed", zap.Error(err))
	}

	// ── Redis ─────────────────────────────────────────────────────────────────
	redisClient, err := redisclient.NewClient(&cfg.Redis, log)
	if err != nil {
		log.Fatal("redis connection failed", zap.Error(err))
	}
	defer redisClient.Close()

	// ── RabbitMQ ──────────────────────────────────────────────────────────────
	mqConn, err := rabbitmq.NewConnection(cfg.RabbitMQ.URL, log)
	if err != nil {
		log.Fatal("rabbitmq connection failed", zap.Error(err))
	}
	defer mqConn.Close()

	publisher, err := rabbitmq.NewPublisher(mqConn, log)
	if err != nil {
		log.Fatal("rabbitmq publisher failed", zap.Error(err))
	}

	// ── Metrics ───────────────────────────────────────────────────────────────
	m := metrics.New()

	// ── Auth ──────────────────────────────────────────────────────────────────
	tokenManager   := auth.NewTokenManager(cfg.JWT.Secret, cfg.JWT.AccessTokenTTL)
	authMiddleware := middleware.Authenticate(tokenManager)

	// ── Repositories ──────────────────────────────────────────────────────────
	userRepo         := users.NewRepository(db)
	walletRepo       := wallet.NewRepository(db)
	txnRepo          := transaction.NewRepository(db)
	refundRepo       := refund.NewRepository(db)
	notificationRepo := notification.NewRepository(db)
	fraudRepo        := fraud.NewRepository(db)
	analyticsRepo    := analytics.NewRepository(db)
	auditRepo        := audit.NewRepository(db)

	// ── Services ──────────────────────────────────────────────────────────────
	userSvc         := users.NewService(userRepo, tokenManager, redisClient, publisher, log)
	walletSvc       := wallet.NewService(walletRepo, publisher, redisClient, m, log)
	fraudChecker    := fraud.NewChecker(fraudRepo, redisClient, fraud.DefaultThresholds(), log)
	fraudSvc        := fraud.NewService(fraudChecker, fraudRepo, log)
	txnSvc          := transaction.NewService(txnRepo, walletRepo, userRepo, fraudSvc, publisher, redisClient, m, log)
	refundSvc       := refund.NewService(refundRepo, txnRepo, walletRepo, publisher, log)
	notificationSvc := notification.NewService(notificationRepo, log)
	analyticsSvc    := analytics.NewService(analyticsRepo, walletRepo, redisClient, log)
	auditSvc        := audit.NewService(auditRepo, log)

	// ── Background Workers ────────────────────────────────────────────────────
	workerCtx, cancelWorkers := context.WithCancel(context.Background())

	// Transaction worker pool
	txnSvc.StartWorkers(workerCtx)

	// Audit consumer
	auditConsumer, err := audit.NewConsumer(mqConn, auditSvc, log)
	if err != nil {
		log.Fatal("audit consumer init failed", zap.Error(err))
	}
	go func() {
		if err := auditConsumer.Start(workerCtx); err != nil {
			log.Error("audit consumer stopped", zap.Error(err))
		}
	}()

	// Notification consumer
	notifConsumer, err := notification.NewConsumer(mqConn, notificationSvc, log)
	if err != nil {
		log.Fatal("notification consumer init failed", zap.Error(err))
	}
	go func() {
		if err := notifConsumer.Start(workerCtx); err != nil {
			log.Error("notification consumer stopped", zap.Error(err))
		}
	}()

	// Analytics consumer
	analyticsConsumer, err := analytics.NewConsumer(mqConn, analyticsSvc, log)
	if err != nil {
		log.Fatal("analytics consumer init failed", zap.Error(err))
	}
	go func() {
		if err := analyticsConsumer.Start(workerCtx); err != nil {
			log.Error("analytics consumer stopped", zap.Error(err))
		}
	}()

	// ── HTTP Handlers ─────────────────────────────────────────────────────────
	userHandler         := users.NewHandler(userSvc, log)
	walletHandler       := wallet.NewHandler(walletSvc, log)
	txnHandler          := transaction.NewHandler(txnSvc, log)
	refundHandler       := refund.NewHandler(refundSvc, log)
	notificationHandler := notification.NewHandler(notificationSvc, log)
	fraudHandler        := fraud.NewHandler(fraudSvc, log)
	analyticsHandler    := analytics.NewHandler(analyticsSvc, log)
	auditHandler        := audit.NewHandler(auditSvc, log)

	// ── Router ────────────────────────────────────────────────────────────────
	if cfg.App.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(middleware.RequestLogger(log))
	router.Use(middleware.CORS(cfg.App.AllowOrigins))
	router.Use(middleware.SecureHeaders())
	router.Use(middleware.PrometheusMetrics(m))
	router.Use(middleware.RateLimit(redisClient, 100, time.Minute))

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"service": "velocitypay-api",
			"version": "1.0.0",
		})
	})
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Serve frontend static files
	router.Static("/web", "./web")
	router.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/web/index.html")
	})

	// API v1
	v1 := router.Group("/api/v1")
	userHandler.RegisterRoutes(v1, authMiddleware)
	walletHandler.RegisterRoutes(v1, authMiddleware)
	txnHandler.RegisterRoutes(v1, authMiddleware)
	refundHandler.RegisterRoutes(v1, authMiddleware)
	notificationHandler.RegisterRoutes(v1, authMiddleware)
	fraudHandler.RegisterRoutes(v1, authMiddleware)
	analyticsHandler.RegisterRoutes(v1, authMiddleware)
	auditHandler.RegisterRoutes(v1, authMiddleware)

	// ── HTTP Server ───────────────────────────────────────────────────────────
	srv := &http.Server{
		Addr:         ":" + cfg.App.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info("HTTP server listening", zap.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal("server error", zap.Error(err))
		}
	}()

	// ── Graceful Shutdown ─────────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutdown signal received")

	cancelWorkers()
	txnSvc.Shutdown()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("server shutdown error", zap.Error(err))
	}

	log.Info("VelocityPay stopped cleanly")
}
