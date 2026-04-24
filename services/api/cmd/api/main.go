// Command api is the entry point for the Snake Backlink Forge API service.
// It wires config → logger → DB pool → Redis client → Fiber app, then starts
// listening and blocks until SIGINT/SIGTERM triggers a graceful shutdown.
package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/api"
	appbot "github.com/kekuta/snake-backlink-forge/services/api/internal/bot"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/config"
	appdb "github.com/kekuta/snake-backlink-forge/services/api/internal/db"
	sqlcdb "github.com/kekuta/snake-backlink-forge/services/api/internal/db/sqlc"
	appredis "github.com/kekuta/snake-backlink-forge/services/api/internal/redis"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/service"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/util"
	"go.uber.org/zap"
)

func main() {
	// Load .env file if present; ignore error (file won't exist in production).
	_ = godotenv.Load()

	// --- Config ---
	cfg, err := config.Load()
	if err != nil {
		// Use stderr directly — zap isn't built yet at this point.
		_, _ = os.Stderr.WriteString("fatal: config: " + err.Error() + "\n")
		os.Exit(1)
	}

	// --- Logger ---
	log, err := util.NewLogger(cfg)
	if err != nil {
		_, _ = os.Stderr.WriteString("fatal: logger: " + err.Error() + "\n")
		os.Exit(1)
	}
	defer func() { _ = log.Sync() }()

	// --- Database (lenient Phase 1: nil pool keeps the process running) ---
	dbPool, err := appdb.NewPool(context.Background(), cfg)
	if err != nil {
		log.Warn("database unavailable — continuing without DB (Phase 1 lenient boot)",
			zap.Error(err))
		// dbPool is nil on error; handlers check for nil gracefully.
	} else {
		log.Info("database connected")
	}

	// --- Redis (lenient Phase 1: nil client keeps the process running) ---
	rdb, err := appredis.NewClient(cfg)
	if err != nil {
		log.Warn("redis unavailable — continuing without Redis (Phase 1 lenient boot)",
			zap.Error(err))
		// rdb is nil on error; handlers check for nil gracefully.
	} else {
		log.Info("redis connected")
	}

	// --- KeyService (Phase 03) ---
	// Wired before UserService so it can be passed as the KeyIssuer dependency.
	// If dbPool is nil (lenient boot), both services remain nil — bot falls back to dev mode.
	var keySvc *service.KeyService
	if dbPool != nil {
		keySvc = service.NewKeyService(dbPool, log.Named("key_svc"))
		log.Info("key service initialized")
	}

	// --- UserService (Phase 02+03) ---
	// Phase 03: real KeyService replaces NoopKeyIssuer.
	// If dbPool is nil (lenient boot), UserService is nil — bot handlers fall back to dev mode.
	var userSvc *service.UserService
	if dbPool != nil {
		userSvc = service.New(dbPool, rdb, keySvc, log.Named("user_svc"))
		log.Info("user service initialized")
	} else {
		log.Warn("user service disabled — no DB pool")
	}

	// --- WalletService (Phase 04) ---
	// Shares the same sqlcdb.Queries instance constructed from dbPool.
	// Nil when dbPool is nil — bot /balance falls back to "coming soon" message.
	var walletSvc *service.WalletService
	if dbPool != nil {
		walletSvc = service.NewWalletService(dbPool, sqlcdb.New(dbPool), log.Named("wallet_svc"))
		log.Info("wallet service initialized")
	}

	// --- TransactionService (Phase 05) ---
	// Manages pending topup intents, QR URL generation, and cancel lifecycle.
	// Nil when dbPool is nil — bot /buy falls back to "coming soon" message.
	var txSvc *service.TransactionService
	if dbPool != nil {
		txSvc = service.NewTransactionService(dbPool, sqlcdb.New(dbPool), rdb, cfg, log.Named("tx"))
		log.Info("transaction service initialized")
	}

	// --- Bot (Phase 03) ---
	// Bot manages its own internal contexts (loopCtx + handlerCtx).
	// Shutdown is coordinated via bot.Stop(), which drains in-flight handlers
	// before cancelling handlerCtx — so we do NOT pass a cancellable ctx here.
	var bot *appbot.Bot
	if cfg.TelegramBotToken == "" {
		log.Warn("bot disabled — TELEGRAM_BOT_TOKEN empty")
	} else {
		b, botErr := appbot.New(&appbot.Deps{
			Pool:          dbPool,
			Rdb:           rdb,
			Log:           log.Named("bot"),
			Cfg:           cfg,
			UserService:   userSvc,
			KeyService:    keySvc,    // Phase 03
			WalletService: walletSvc, // Phase 04
			TxService:     txSvc,     // Phase 05
		})
		if botErr != nil {
			if errors.Is(botErr, appbot.ErrBotDisabled) {
				log.Warn("bot disabled", zap.Error(botErr))
			} else {
				log.Error("bot init failed", zap.Error(botErr))
			}
		} else {
			bot = b
			go bot.Start(context.Background())
		}
	}

	// --- HTTP Server ---
	app := api.New(cfg, log, dbPool, rdb)

	// Start listener in a goroutine so we can wait for shutdown below.
	listenErr := make(chan error, 1)
	go func() {
		addr := ":" + cfg.Port
		log.Info("starting server", zap.String("addr", addr), zap.String("env", cfg.Env))
		if listenE := app.Listen(addr); listenE != nil {
			listenErr <- listenE
		}
	}()

	// --- Graceful Shutdown ---
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		log.Info("shutdown signal received", zap.String("signal", sig.String()))
	case listenE := <-listenErr:
		log.Error("server listen error", zap.Error(listenE))
	}

	log.Info("shutting down — draining connections (max 10s)")

	// 1. Stop bot first: loopCancel stops new updates, drains in-flights (10s),
	//    then handlerCancel fires. Must happen before closing DB/Redis.
	if bot != nil {
		bot.Stop()
		log.Info("bot stopped")
	}

	// 2. Shutdown HTTP server.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if shutdownErr := app.ShutdownWithContext(shutdownCtx); shutdownErr != nil {
		log.Error("graceful shutdown failed", zap.Error(shutdownErr))
	}

	// 3. Close shared resources — safe now that bot handlers have fully drained.
	if dbPool != nil {
		dbPool.Close()
		log.Info("database pool closed")
	}
	if rdb != nil {
		if closeErr := rdb.Close(); closeErr != nil {
			log.Warn("redis close error", zap.Error(closeErr))
		} else {
			log.Info("redis client closed")
		}
	}

	log.Info("shutdown complete")
}
