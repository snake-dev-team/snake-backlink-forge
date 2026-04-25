// Package bot — middleware chain for Telegram update handlers.
// Chain order: recover → logger → loadUser → banCheck → i18n → handler.
// Context types and update helpers live in update_helpers.go.
package bot

import (
	"context"
	"fmt"
	"runtime/debug"
	"slices"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/config"
	"go.uber.org/zap"
)

// chain builds a single HandlerFunc from the middleware stack + terminal handler.
// Stack is applied outermost-first: chain[0] wraps chain[1] wraps … wraps final.
func chain(middlewares []func(HandlerFunc) HandlerFunc, final HandlerFunc) HandlerFunc {
	h := final
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}
	return h
}

// recoverMiddleware catches any panic in downstream handlers, logs the stack trace
// (redacting message text beyond 200 chars), and sends a generic error reply.
func recoverMiddleware(log *zap.Logger) func(HandlerFunc) HandlerFunc {
	return func(next HandlerFunc) HandlerFunc {
		return func(ctx context.Context, bot *tgbotapi.BotAPI, update tgbotapi.Update) (retErr error) {
			defer func() {
				if r := recover(); r != nil {
					stack := debug.Stack()
					// Redact message text to avoid leaking phone numbers from contact shares.
					safeText := ""
					if update.Message != nil && update.Message.Text != "" {
						t := update.Message.Text
						if len(t) > 200 {
							t = t[:200] + "[redacted]"
						}
						safeText = t
					}
					log.Error("panic in bot handler",
						zap.Any("recover", r),
						zap.String("stack", string(stack)),
						zap.String("text_prefix", safeText),
					)
					replyText(bot, update, "Đã xảy ra lỗi. Vui lòng thử lại sau.")
					retErr = fmt.Errorf("panic: %v", r)
				}
			}()
			return next(ctx, bot, update)
		}
	}
}

// loggerMiddleware records latency, command, tgID, and result status for every update.
func loggerMiddleware(log *zap.Logger) func(HandlerFunc) HandlerFunc {
	return func(next HandlerFunc) HandlerFunc {
		return func(ctx context.Context, bot *tgbotapi.BotAPI, update tgbotapi.Update) error {
			start := time.Now()
			err := next(ctx, bot, update)
			latency := time.Since(start).Milliseconds()

			status := "ok"
			if err != nil {
				status = "error"
			}

			log.Info("update handled",
				zap.Int64("tg_id", updateTelegramID(update)),
				zap.String("command", updateCommand(update)),
				zap.Int64("latency_ms", latency),
				zap.String("result", status),
				zap.Error(err),
			)
			return err
		}
	}
}

// loadUser upserts the Telegram user in Postgres via UserService and attaches a BotUser to ctx.
//
// Failure semantics are FAIL-CLOSED (H1): on any DB error, no next() call is made
// so that banCheck cannot be bypassed by a transient DB blip.
//
// Dev-mode (deps.UserService == nil): a synthetic BotUser with safe defaults is attached
// so handlers work normally in local dev without a database.
func loadUser(deps *Deps) func(HandlerFunc) HandlerFunc {
	return func(next HandlerFunc) HandlerFunc {
		return func(ctx context.Context, bot *tgbotapi.BotAPI, update tgbotapi.Update) error {
			if deps.UserService == nil {
				// Dev mode: create synthetic user so middleware chain is exercisable.
				synthetic := BotUser{
					Language: "vi",
					IsBanned: false,
				}
				ctx = context.WithValue(ctx, ctxKeyUser, synthetic)
				return next(ctx, bot, update)
			}

			tgID := updateTelegramID(update)
			if tgID == 0 {
				return next(ctx, bot, update)
			}

			// Extract first name and username for upsert.
			var username, firstName string
			if update.Message != nil && update.Message.From != nil {
				username = update.Message.From.UserName
				firstName = update.Message.From.FirstName
			} else if update.CallbackQuery != nil {
				username = update.CallbackQuery.From.UserName
				firstName = update.CallbackQuery.From.FirstName
			}

			dbUser, err := deps.UserService.EnsureStub(ctx, tgID, username, firstName)
			if err != nil {
				// Fail closed: do NOT call next() — a banned user must not reach the handler.
				deps.Log.Warn("loadUser: EnsureStub failed, failing closed",
					zap.Int64("tg_id", tgID), zap.Error(err))
				msg := tgbotapi.NewMessage(updateChatID(update),
					"Hệ thống tạm thời gặp sự cố. Vui lòng thử lại sau.")
				_, _ = bot.Send(msg)
				return nil
			}

			user := BotUser{
				ID:         dbUser.ID,
				Language:   dbUser.Language,
				IsBanned:   dbUser.IsBanned,
				IsVerified: dbUser.IsVerified,
			}
			ctx = context.WithValue(ctx, ctxKeyUser, user)
			return next(ctx, bot, update)
		}
	}
}

// banCheck short-circuits the handler chain when a user is banned.
// deps param removed (M1): ban status lives in BotUser from loadUser, no DB query needed.
func banCheck(next HandlerFunc) HandlerFunc {
	return func(ctx context.Context, bot *tgbotapi.BotAPI, update tgbotapi.Update) error {
		user, ok := UserFromCtx(ctx)
		if ok && user.IsBanned {
			replyText(bot, update, "Tài khoản đã bị tạm khoá. Liên hệ @support để được hỗ trợ.")
			return nil
		}
		return next(ctx, bot, update)
	}
}

// i18nMiddleware sets the lang key in ctx from the user's language preference.
func i18nMiddleware() func(HandlerFunc) HandlerFunc {
	return func(next HandlerFunc) HandlerFunc {
		return func(ctx context.Context, bot *tgbotapi.BotAPI, update tgbotapi.Update) error {
			lang := "vi"
			if user, ok := UserFromCtx(ctx); ok && user.Language != "" {
				lang = user.Language
			}
			ctx = context.WithValue(ctx, ctxKeyLang, lang)
			return next(ctx, bot, update)
		}
	}
}

// buildChain assembles the full middleware stack for a bot handler.
func buildChain(deps *Deps, final HandlerFunc) HandlerFunc {
	return chain([]func(HandlerFunc) HandlerFunc{
		recoverMiddleware(deps.Log),
		loggerMiddleware(deps.Log),
		loadUser(deps),
		banCheck, // M1: banCheck no longer takes deps
		i18nMiddleware(),
	}, final)
}

// IsAdmin reports whether the given Telegram user ID is in the admin allowlist.
// Zero-cost linear scan against pre-validated cfg.AdminTelegramIDs (sorted, deduped by config.Load).
// Uses slices.Contains (Go 1.21+) for clarity; list is small (typically 1-5 entries).
func IsAdmin(cfg *config.Config, tgID int64) bool {
	return slices.Contains(cfg.AdminTelegramIDs, tgID)
}
