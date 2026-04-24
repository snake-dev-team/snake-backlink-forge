// Package bot — middleware chain for Telegram update handlers.
// Chain order: recover → logger → loadUser → banCheck → i18n → handler.
// Context types and update helpers live in update_helpers.go.
package bot

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
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

// loadUser upserts the Telegram user in Postgres and attaches a BotUser to ctx.
//
// Failure semantics are FAIL-CLOSED (H1): on any DB error, no next() call is made
// so that banCheck cannot be bypassed by a transient DB blip.
//
// Dev-mode (deps.Pool == nil): a synthetic BotUser with safe defaults is attached
// so handlers work normally in local dev without a database.
func loadUser(deps *Deps) func(HandlerFunc) HandlerFunc {
	return func(next HandlerFunc) HandlerFunc {
		return func(ctx context.Context, bot *tgbotapi.BotAPI, update tgbotapi.Update) error {
			if deps.Pool == nil {
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

			// Derive username if available.
			var username *string
			if update.Message != nil && update.Message.From != nil && update.Message.From.UserName != "" {
				s := update.Message.From.UserName
				username = &s
			}

			var user BotUser
			err := deps.Pool.QueryRow(ctx, `
				INSERT INTO users (telegram_id, telegram_username, language, updated_at)
				VALUES ($1, $2, 'vi', NOW())
				ON CONFLICT (telegram_id) DO UPDATE
					SET telegram_username = EXCLUDED.telegram_username,
					    updated_at = NOW()
				RETURNING id, language, is_banned
			`, tgID, username).Scan(&user.ID, &user.Language, &user.IsBanned)
			if err != nil {
				// Fail closed: do NOT call next() — a banned user must not reach the handler.
				deps.Log.Warn("loadUser: upsert failed, failing closed",
					zap.Int64("tg_id", tgID), zap.Error(err))
				msg := tgbotapi.NewMessage(updateChatID(update),
					"⚠️ Hệ thống tạm thời gặp sự cố. Vui lòng thử lại sau.")
				_, _ = bot.Send(msg)
				return nil
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
