package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"bot-jual/internal/atl"
	"bot-jual/internal/cache"
	"bot-jual/internal/config"
	"bot-jual/internal/convo"
	"bot-jual/internal/handlers"
	"bot-jual/internal/httpserver"
	"bot-jual/internal/logging"
	"bot-jual/internal/metrics"
	"bot-jual/internal/nlu"
	"bot-jual/internal/repo"
	"bot-jual/internal/wa"
	"bot-jual/migrations"

	"github.com/joho/godotenv"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger := logging.NewLogger(cfg.LogLevel)
	logger.Info("starting wa-sales-bot", "env", cfg.AppEnv)

	if cfg.PublicBaseURL != "" {
		webhookURL := strings.TrimRight(cfg.PublicBaseURL, "/") + "/webhook/atlantic"
		logger.Info("public base url configured", "base_url", cfg.PublicBaseURL, "webhook_url", webhookURL)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	metricRegistry := metrics.Registry(cfg.MetricsNamespace)

	repository, err := repo.New(ctx, cfg.DatabaseURL, cfg.SupabaseSchema, logger)
	if err != nil {
		return fmt.Errorf("init repository: %w", err)
	}
	defer repository.Close()

	if err := repository.RunMigrations(ctx, migrations.Files); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	logger.Info("database migrated")

	if err := repository.SyncGeminiKeys(ctx, cfg.GeminiAPIKeys); err != nil {
		return fmt.Errorf("sync gemini keys: %w", err)
	}

	redisClient := cache.New(cache.Config{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
		UseTLS:   cfg.RedisTLS,
	}, logger)
	defer func() {
		if err := redisClient.Close(); err != nil {
			logger.Warn("failed closing redis", "error", err)
		}
	}()
	if err := redisClient.Ping(ctx); err != nil {
		logger.Warn("redis ping failed", "error", err)
	}

	nluClient := nlu.New(repository, logger, metricRegistry, nlu.Config{
		Model:    cfg.GeminiModel,
		Timeout:  cfg.GeminiTimeout,
		Cooldown: cfg.GeminiCooldown,
	})

	atlClient := atl.New(atl.Config{
		BaseURL: cfg.AtlanticBaseURL,
		APIKey:  cfg.AtlanticAPIKey,
		Timeout: cfg.AtlanticTimeout,
	}, logger, metricRegistry, redisClient)

	waClient, err := wa.New(ctx, wa.Config{
		StorePath: cfg.WhatsAppStorePath,
		LogLevel:  cfg.WhatsAppLogLevel,
		Metrics:   metricRegistry,
	}, logger)
	if err != nil {
		return fmt.Errorf("init whatsapp client: %w", err)
	}
	defer waClient.Close()

	convoEngine := convo.New(repository, nluClient, atlClient, waClient, redisClient, metricRegistry, logger, convo.EngineConfig{
		DefaultDepositMethod: cfg.AtlanticDepositMethod,
		DefaultDepositType:   cfg.AtlanticDepositType,
		DepositFeeFixed:      cfg.AtlanticDepositFeeFixed,
		DepositFeePercent:    cfg.AtlanticDepositFeePercent,
	})
	waClient.SetMessageProcessor(convoEngine)

	webhookProcessor := handlers.NewAtlanticWebhookProcessor(repository, waClient, metricRegistry, logger, atlClient)
	webhookHandler := atl.NewWebhookHandler(logger, metricRegistry, cfg.AtlanticWebhookSecretMD5Username, cfg.AtlanticWebhookSecretMD5Password, webhookProcessor)

	waCtx, waCancel := context.WithCancel(ctx)
	defer waCancel()
	go func() {
		if err := waClient.Start(waCtx); err != nil {
			logger.Error("whatsapp client stopped", "error", err)
			stop()
		}
	}()

	httpSrv := httpserver.New(cfg.HTTPListenAddr, logger, metricRegistry, httpserver.Handlers{
		AtlanticWebhook: webhookHandler,
	}, cfg.PublicBasePath)
	httpSrv.SetDependencies(httpserver.Dependencies{
		Repository: repository,
		Redis:      redisClient,
		NLU:        nluClient,
		Atlantic:   atlClient,
	})

	errCh := make(chan error, 1)
	go func() {
		if err := httpSrv.Start(); err != nil {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("http server error: %w", err)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		logger.Error("http server shutdown error", "error", err)
	}

	return nil
}
