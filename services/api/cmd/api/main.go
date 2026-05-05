// Command api is the entry point for the Snake Backlink Forge API service.
// It wires config → logger → DB pool → Redis client → Fiber app, then starts
// listening and blocks until SIGINT/SIGTERM triggers a graceful shutdown.
package main

import (
	"context"
	"encoding/hex"
	"errors"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/ai"
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
	"github.com/kekuta/snake-backlink-forge/services/api/internal/verification"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/worker"
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

	if cfg.WPEncKey != "" {
		raw, decodeErr := hex.DecodeString(cfg.WPEncKey)
		if decodeErr != nil {
			log.Fatal("WP_ENC_KEY must be 32 bytes hex", zap.Error(decodeErr))
		}
		if len(raw) != 32 {
			log.Fatal("WP_ENC_KEY decoded length wrong", zap.Int("got_bytes", len(raw)), zap.Int("want_bytes", 32))
		}
	}

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
	if cfg.IsProduction() && rdb == nil {
		log.Fatal("redis unavailable — refusing production boot because auth rate limiting cannot be enforced")
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

	// --- Campaign automation services (Phase 3 web SaaS) ---
	var campaignSvc *service.CampaignService
	var jobSvc *service.JobService
	var executionSvc *service.ExecutionService
	if dbPool != nil {
		q := sqlcdb.New(dbPool)
		campaignSvc = service.NewCampaignService(dbPool, q, log.Named("campaign_svc"))
		jobSvc = service.NewJobService(dbPool, q, log.Named("job_svc"))
		// Phase 7.06: inject JobService into CampaignService for auto-enqueue on CreateWithSites.
		campaignSvc.SetJobService(jobSvc)
		executionSvc = service.NewExecutionService(dbPool, q, cfg.WorkerLeaseSec, cfg.WorkerRetryLimit, cfg.WorkerModel, log.Named("execution_svc"))
		log.Info("campaign + job + execution services initialized")
	}

	// --- ContentService (Phase 7.04: AI content generation) ---
	// Wires Claude primary + OpenAI fallback via Router. Graceful when keys unset.
	var contentSvc *service.ContentService
	if dbPool != nil {
		aiRouter := ai.NewRouter(
			ai.NewClaudeClient(cfg.AnthropicAPIKey, cfg.AIModelPrimary),
			ai.NewOpenAIClient(cfg.OpenAIAPIKey, ""),
		)
		contentSvc = service.NewContentService(dbPool, sqlcdb.New(dbPool), aiRouter, log.Named("content_svc"))
		if cfg.AnthropicAPIKey == "" && cfg.OpenAIAPIKey == "" {
			log.Warn("content service: no AI keys configured — generate-content endpoint will return 503")
		} else {
			log.Info("content service initialized")
		}
	}

	// --- WpSiteService (Phase 3 web SaaS) ---
	var wpSiteSvc *service.WpSiteService
	if dbPool != nil && cfg.WPEncKey != "" {
		wpSiteSvc = service.NewWpSiteService(dbPool, sqlcdb.New(dbPool), cfg.WPEncKey, log.Named("wp_site_svc"))
		log.Info("wp site service initialized")
	} else if dbPool != nil {
		log.Warn("wp site service disabled — WP_ENC_KEY empty")
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
			Pool:            dbPool,
			Rdb:             rdb,
			Log:             log.Named("bot"),
			Cfg:             cfg,
			UserService:     userSvc,
			KeyService:      keySvc,
			WalletService:   walletSvc,
			TxService:       txSvc,
			SupportService:  supportSvc,
			RefService:      refSvc,
			AdminService:    adminSvc,
			AuditService:    auditSvc,
			Templates:       tmplRenderer,
			CampaignService: campaignSvc,
			JobService:      jobSvc,
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

	// --- Post-publish Verifier (Phase 7.07) ---
	// Runs as a background goroutine alongside the worker. Receives job IDs via a
	// buffered channel (non-blocking push from job_runner) and a 5-min retry ticker.
	// Nil-safe: worker skips Verify() call when Verifier is nil.
	var verifier *verification.Verifier
	if dbPool != nil {
		verifier = verification.New(sqlcdb.New(dbPool), log.Named("verifier"))
		go verifier.Start(rootCtx)
		log.Info("post-publish verifier started")
	}

	// --- Embedded Go Worker (Phase 7.05) ---
	// Spawned as a detached goroutine so it never blocks API server startup.
	// Controlled by WORKER_ENABLED env var; gracefully shut down via rootCtx cancellation.
	var embeddedWorker *worker.Worker
	if cfg.WorkerEnabled && dbPool != nil && jobSvc != nil && wpSiteSvc != nil {
		workerCfg := worker.Config{
			Enabled:       true,
			PollInterval:  cfg.WorkerPollIntervalDuration(),
			MaxConcurrent: cfg.WorkerMaxConcurrent,
			RetryLimit:    cfg.WorkerInternalRetryLimit,
			LeaseDuration: cfg.WorkerLeaseDurationDuration(),
		}
		embeddedWorker = worker.New(workerCfg, worker.Deps{
			Q:         sqlcdb.New(dbPool),
			JobSvc:    jobSvc,
			WpSiteSvc: wpSiteSvc,
			Verifier:  verifier, // nil-safe: skipped when verifier unavailable
		}, log)
		go embeddedWorker.Start(rootCtx)
		log.Info("embedded worker started",
			zap.Bool("enabled", true),
			zap.Duration("poll_interval", workerCfg.PollInterval),
			zap.Int("max_concurrent", workerCfg.MaxConcurrent),
		)
	} else if cfg.WorkerEnabled {
		log.Warn("embedded worker disabled — missing required dependencies (dbPool, jobSvc, or wpSiteSvc)")
	}

	// --- HTTP Server ---
	app := api.New(cfg, log, dbPool, rdb)
	apiDeps := &handlers.ApiHandlerDeps{
		Pool:        dbPool,
		Rdb:         rdb,
		KeySvc:      keySvc,
		UserSvc:     userSvc,
		WalletSvc:   walletSvc,
		TxSvc:       txSvc,
		AuditSvc:    auditSvc,
		WpSiteSvc:   wpSiteSvc,
		CampaignSvc: campaignSvc,
		JobSvc:            jobSvc,
		ContentSvc:        contentSvc,
		ExecutionSvc:      executionSvc,
		WorkerSharedToken: cfg.WorkerSharedToken,
		WorkerEnabled:     cfg.WorkerEnabled,
		WorkerID:          func() string {
			if embeddedWorker != nil {
				return embeddedWorker.WorkerID()
			}
			return ""
		}(),
		Log: log.Named("api_v1"),
	}
	if dbPool != nil {
		apiDeps.Queries = sqlcdb.New(dbPool)
	}
	api.RegisterV1(app, apiDeps)
	log.Info("api v1 routes registered")

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

	// 1. Cancel root context — stops retry consumer, admin alert consumer, and embedded worker.
	rootCancel()

	// 1a. Signal embedded worker to drain in-flight jobs (it observes rootCtx cancellation).
	// Stop() is a no-op hook for symmetry; actual drain happens inside worker.Start().
	if embeddedWorker != nil {
		embeddedWorker.Stop(context.Background())
		log.Info("embedded worker stop signalled")
	}

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
