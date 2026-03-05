package main

import (
	"context"
	"fmt"
	"log/slog"

	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/debmalyaroy/ai-cm/internal/agent"
	"github.com/debmalyaroy/ai-cm/internal/cron"

	// _ "github.com/debmalyaroy/ai-cm/docs" // Swagger docs
	// swaggerFiles "github.com/swaggo/files"
	// ginSwagger "github.com/swaggo/gin-swagger"

	"github.com/debmalyaroy/ai-cm/internal/config"
	"github.com/debmalyaroy/ai-cm/internal/database"
	"github.com/debmalyaroy/ai-cm/internal/handlers"
	"github.com/debmalyaroy/ai-cm/internal/llm"
	"github.com/debmalyaroy/ai-cm/internal/logger"
	"github.com/debmalyaroy/ai-cm/internal/prompts"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// @title AI Category Manager API
// @version 1.0
// @description REST API for AI Category Manager
// @host localhost:8080
// @BasePath /api
func main() {
	// Load config: YAML file + env overrides
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "../../config/config.local.yaml"
	}
	cfg := config.Load(configPath)

	// Initialize global structured logger
	logger.Init(cfg.Logging.Level)

	slog.Info("config loaded",
		"port", cfg.Server.Port,
		"llm_provider", cfg.LLM.Provider,
		"log_level", cfg.Logging.Level)

	// Initialize database
	ctx := context.Background()

	// Set DATABASE_URL from config if not already in env
	if os.Getenv("DATABASE_URL") == "" && cfg.Database.URL != "" {
		os.Setenv("DATABASE_URL", cfg.Database.URL)
	}

	db, err := database.NewPool(ctx)
	if err != nil {
		slog.Error("database connection failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// Initialize LLM client
	if os.Getenv("LLM_PROVIDER") == "" {
		os.Setenv("LLM_PROVIDER", cfg.LLM.Provider)
	}
	if cfg.LLM.GeminiKey != "" && os.Getenv("GEMINI_API_KEY") == "" {
		os.Setenv("GEMINI_API_KEY", cfg.LLM.GeminiKey)
	}
	if cfg.LLM.GeminiModel != "" && os.Getenv("GEMINI_MODEL") == "" {
		os.Setenv("GEMINI_MODEL", cfg.LLM.GeminiModel)
	}
	if cfg.LLM.OpenAIKey != "" && os.Getenv("OPENAI_API_KEY") == "" {
		os.Setenv("OPENAI_API_KEY", cfg.LLM.OpenAIKey)
	}
	if cfg.LLM.OpenAIModel != "" && os.Getenv("OPENAI_MODEL") == "" {
		os.Setenv("OPENAI_MODEL", cfg.LLM.OpenAIModel)
	}

	llmClient, err := llm.NewClient(cfg)
	if err != nil {
		slog.Error("LLM init failed", "provider", cfg.LLM.Provider, "error", err)
		os.Exit(1)
	}
	slog.Info("LLM provider initialized", "provider", cfg.LLM.Provider)

	// Initialize System Prompts loader
	promptDir := os.Getenv("PROMPT_DIR")
	if promptDir == "" {
		promptDir = "../prompts" // Default fallback relative to cmd/server
	}
	if err := prompts.Init(promptDir); err != nil {
		slog.Warn("failed to initialize prompts. System prompts may be unavailable.", "error", err)
	}

	// Initialize Distributed Cron Scheduler
	nodeID := os.Getenv("POD_NAME")
	if nodeID == "" {
		nodeID = fmt.Sprintf("node-%d", time.Now().UnixNano())
	}
	scheduler := cron.NewScheduler(db, nodeID)

	watchdogAgent := agent.NewWatchdogAgent(db)

	// Register generic interval anomaly check
	scheduler.Register(cron.NewIntervalJob("watchdog-anomaly-checks", 5*time.Minute, func(c context.Context) error {
		_, err := watchdogAgent.Process(c, &agent.Input{Query: "interval-check"})
		return err
	}))

	// Register specific time-based alert (e.g., Daily at 8:00 AM)
	scheduler.Register(cron.NewDailyJob("watchdog-daily-alerts", 8, 0, func(c context.Context) error {
		_, err := watchdogAgent.Process(c, &agent.Input{Query: "time-based-check"})
		return err
	}))

	// Start scheduler
	scheduler.Start(ctx)

	// Setup Gin router
	if cfg.Logging.Level != "debug" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()

	// Middleware stack
	router.Use(handlers.Recovery())
	router.Use(handlers.RequestLogger())
	router.Use(cors.New(cors.Config{
		AllowOrigins:     cfg.CORS.AllowOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		AllowCredentials: true,
	}))

	// Security: env-var overrides for API key auth
	if v := os.Getenv("API_KEY_AUTH_ENABLED"); v == "true" {
		cfg.Security.APIKeyAuthEnabled = true
	}
	if v := os.Getenv("API_KEYS"); v != "" {
		cfg.Security.APIKeys = strings.Split(v, ",")
	}
	if v := os.Getenv("RATE_LIMIT_PER_MINUTE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Security.RateLimitPerMinute = n
		}
	}
	if v := os.Getenv("RATE_LIMIT_ENABLED"); v == "false" {
		cfg.Security.RateLimitEnabled = false
	}

	// Health check (includes DB ping)
	router.GET("/api/health", func(c *gin.Context) {
		dbStatus := "ok"
		if err := db.Ping(c.Request.Context()); err != nil {
			dbStatus = fmt.Sprintf("error: %v", err)
		}
		c.JSON(http.StatusOK, gin.H{
			"status":   "ok",
			"provider": cfg.LLM.Provider,
			"database": dbStatus,
			"time":     time.Now().Format(time.RFC3339),
		})
	})

	// Readiness probe
	router.GET("/api/readiness", func(c *gin.Context) {
		if err := db.Ping(c.Request.Context()); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready", "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})

	// Register route handlers
	api := router.Group("/api")
	api.Use(handlers.RateLimiter(handlers.RateLimiterConfig{
		Enabled:           cfg.Security.RateLimitEnabled,
		RequestsPerMinute: cfg.Security.RateLimitPerMinute,
	}))
	api.Use(handlers.APIKeyAuth(func() []string {
		if cfg.Security.APIKeyAuthEnabled {
			return cfg.Security.APIKeys
		}
		return nil
	}()))
	{
		handlers.RegisterDashboardRoutes(api, db, llmClient)
		handlers.RegisterChatRoutes(api, db, llmClient, cfg.LLM.AgentModels)
		handlers.RegisterActionRoutes(api, db, llmClient)
		handlers.RegisterAlertRoutes(api, db) // Issue 2 Fix: Alert Routes
		handlers.RegisterReportRoutes(api, db)
		handlers.RegisterGraphQLRoutes(api, db, llmClient, cfg.LLM.AgentModels)
	}

	// Swagger documentation (accessible at /swagger/index.html)
	// router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Start server
	port := fmt.Sprintf(":%d", cfg.Server.Port)
	srv := &http.Server{
		Addr:         port,
		Handler:      router,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeoutSec) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeoutSec) * time.Second,
	}

	// Graceful shutdown
	go func() {
		slog.Info("server starting", "addr", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down services gracefully...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Server.ShutdownSec)*time.Second)
	defer cancel()

	// 1. Shutdown API Server
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server forced shutdown", "error", err)
	} else {
		slog.Info("server exited cleanly")
	}

	// 2. Shutdown Cron Scheduler
	scheduler.Stop()
	slog.Info("cron scheduler stopped")

	// 3. Database connection pool is closed via the `defer db.Close()` at the top of main.
	slog.Info("shutdown complete")
}
