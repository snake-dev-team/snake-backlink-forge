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
	"github.com/kekuta/snake-backlink-forge/services/api/internal/api/handlers"
	appbot "github.com/kekuta/snake-backlink-forge/services/api/internal/bot"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/bot/templates"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/config"
	appdb "github.com/kekuta/snake-backlink-forge/services/api/internal/db"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/db/migrator"
	sqlcdb "github.com/kekuta/snake-backlink-forge/services/api/internal/db/sqlc"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/notify"
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

	// --- Root context — server lifetime; cancel triggers goroutine shutdown. ---
	rootCtx, rootCancel := context.WithCancel(context.Background())
	defer rootCancel()

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

	// --- Auto-migration on boot (Phase 10 production hard-fail policy) ---
	// Idempotent: applies pending migrations; no-op if up-to-date.
	// Hard fail (log.Fatal exits non-zero) prevents serving traffic with stale schema.
	// Fly.io machine restart policy applies backoff; permanent failure surfaces via crash-loop logs.
	if dbPool != nil {
		log.Info("running migrations on startup")
		if migErr := migrator.Up(rootCtx, dbPool); migErr != nil {
			log.Fatal("startup migration failed — refusing to boot", zap.Error(migErr))
		}
		log.Info("migrations applied successfully")
	}

	// --- KeyService (Phase 03) ---
	var keySvc *service.KeyService
	if dbPool != nil {
		keySvc = service.NewKeyService(dbPool, log.Named("key_svc"))
		log.Info("key service initialized")
	}

	// --- UserService (Phase 02+03) ---
	var userSvc *service.UserService
	if dbPool != nil {
		userSvc = service.New(dbPool, rdb, keySvc, log.Named("user_svc"))
		log.Info("user service initialized")
	} else {
		log.Warn("user service disabled — no DB pool")
	}

	// --- WalletService (Phase 04) ---
	var walletSvc *service.WalletService
	if dbPool != nil {
		walletSvc = service.NewWalletService(dbPool, sqlcdb.New(dbPool), log.Named("wallet_svc"))
		log.Info("wallet service initialized")
	}

	// --- TransactionService (Phase 05) ---
	var txSvc *service.TransactionService
	if dbPool != nil {
		txSvc = service.NewTransactionService(dbPool, sqlcdb.New(dbPool), rdb, cfg, log.Named("tx"))
		log.Info("transaction service initialized")
	}

	// --- SupportService + ReferralService (Phase 07) ---
	var supportSvc *service.SupportService
	var refSvc *service.ReferralService
	if dbPool != nil {
		supportSvc = service.NewSupportService(dbPool, sqlcdb.New(dbPool), log.Named("support_svc"))
		refSvc = service.NewReferralService(dbPool, sqlcdb.New(dbPool), log.Named("ref_svc"))
		log.Info("support + referral services initialized")
	}

	// --- AuditService + AdminService (Phase 08) ---
	var auditSvc *service.AuditService
	var adminSvc *service.AdminService
	if dbPool != nil {
		auditSvc = service.NewAuditService(dbPool, sqlcdb.New(dbPool), log.Named("audit_svc"))
		adminSvc = service.NewAdminService(dbPool, sqlcdb.New(dbPool), auditSvc, cfg, log.Named("admin_svc"))
		log.Info("audit + admin services initialized")
	}

	// --- [Q5] Admin alert channel (Phase 06) ---
	// Buffered cap=100; non-blocking sends in webhook/retry consumer; closed on shutdown.
	adminAlertCh := make(chan notify.AdminAlert, 100)

	// --- WebhookService (Phase 06) ---
	var webhookSvc *service.WebhookService
	if dbPool != nil && walletSvc != nil {
		webhookSvc = service.NewWebhookService(
			dbPool, walletSvc, cfg, log.Named("webhook_svc"), adminAlertCh,
		)
		log.Info("webhook service initialized")
	}

	// --- Templates Renderer (Phase 09) ---
	// Single instance shared across all bot handlers. sync.Map cache is
	// concurrent-safe; lifetime = process lifetime.
	tmplRenderer := templates.NewRenderer(log.Named("templates"))
	log.Info("templates renderer initialized")

	// --- Bot (Phase 03) ---
	var bot *appbot.Bot
	if cfg.TelegramBotToken == "" {
		log.Warn("bot disabled — TELEGRAM_BOT_TOKEN empty")
	} else {
		b, botErr := appbot.New(&appbot.Deps{
			Pool:           dbPool,
			Rdb:            rdb,
			Log:            log.Named("bot"),
			Cfg:            cfg,
			UserService:    userSvc,
			KeyService:     keySvc,
			WalletService:  walletSvc,
			TxService:      txSvc,
			SupportService: supportSvc,
			RefService:     refSvc,
			AdminService:   adminSvc,
			AuditService:   auditSvc,
			Templates:      tmplRenderer,
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

			// [Q5] Admin alert consumer — requires bot.Api() to send DMs.
			// Goroutine exits when rootCtx cancelled OR adminAlertCh closed.
			go appbot.ConsumeAdminAlerts(rootCtx, adminAlertCh, bot.Api(), cfg, log.Named("admin_alerts"))
			log.Info("admin alert consumer started")
		}
	}

	// --- [round-3] Retry queue consumer (Phase 06) ---
	if rdb != nil && webhookSvc != nil {
		go service.RetryQueueConsumer(rootCtx, rdb, webhookSvc, adminAlertCh, log.Named("retry_consumer"))
		log.Info("retry queue consumer started")
	}

	// --- [Phase 08] Auth-fail burst alert watcher ---
	// Polls audit_log every 60s; ≥20 sepay_auth_fail in 15min → admin alert (bucket-deduped).
	if dbPool != nil {
		go appbot.AuditFailAlertWatcher(rootCtx, dbPool, adminAlertCh, log.Named("audit_fail_watcher"))
		log.Info("audit fail watcher started")
	}

	// --- HTTP Server ---
	app := api.New(cfg, log, dbPool, rdb)

	// Register SePay webhook route with deps fully wired. [Phase 06]
	if webhookSvc != nil {
		webhookDeps := &handlers.WebhookDeps{
			Pool:         dbPool,
			Rdb:          rdb,
			Cfg:          cfg,
			Log:          log.Named("webhook"),
			WebhookSvc:   webhookSvc,
			AdminAlertCh: adminAlertCh,
			Templates:    tmplRenderer,
			RootCtx:      rootCtx,
		}
		// Wire BotAPI for post-payment Telegram DM notify (replaces Phase 06 placeholder).
		// nil-safe: when bot is disabled (no token), notify is silently skipped.
		if bot != nil {
			webhookDeps.BotAPI = bot.Api()
		}
		api.RegisterWebhook(app, webhookDeps, log)
		log.Info("webhook route registered")
	}

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

	// 1. Cancel root context — stops retry consumer + admin alert consumer goroutines.
	rootCancel()

	// 2. Stop bot first: drains in-flight handlers before closing DB/Redis.
	if bot != nil {
		bot.Stop()
		log.Info("bot stopped")
	}

	// 3. Close admin alert channel — signals ConsumeAdminAlerts to drain remaining alerts.
	close(adminAlertCh)

	// 4. Shutdown HTTP server.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if shutdownErr := app.ShutdownWithContext(shutdownCtx); shutdownErr != nil {
		log.Error("graceful shutdown failed", zap.Error(shutdownErr))
	}

	// 5. Close shared resources — safe now that all goroutines have drained.
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
