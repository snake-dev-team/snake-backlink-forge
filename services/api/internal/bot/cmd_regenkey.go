// cmd_regenkey.go — /regenkey command + confirmation FSM + Redis rate-limit.
//
// Flow:
//  1. PEEK rate limit: GET regenkey_rate_limit:<userID> (read-only, NO INCR).
//     If count >= 3 → reply rate-limit message, return (no FSM, no Issue).
//  2. Reply confirmation prompt with inline keyboard [Confirm | Cancel].
//     Set FSM state "awaiting_regen_confirm" (5-minute TTL).
//  3. On key:regen:confirm callback:
//     a. Revalidate FSM state (M4) — reject stale/replayed callbacks.
//     b. PEEK rate limit again (guard against window expiry edge case).
//     c. Call KeyService.Issue(userID).
//     d. On Issue success: INCR + EXPIRE pipelined (counter = actual rotations).
//     e. Reply new plaintext key ONCE with bold save-now warning.
//     f. Clear FSM state.
//  4. On key:regen:cancel callback:
//     a. Revalidate FSM state (M4) — clear + edit message.
//
// Rate limit semantics: counter reflects ACTUAL rotations (successful Issue calls)
// in last 24h, not click-intent. Users can preview /regenkey confirm dialog freely.
//
// FSM state name: "awaiting_regen_confirm" (5-minute TTL).
// Rate-limit Redis key: "regenkey_rate_limit:<uuid-string>" — resets after 86400s (one calendar day).
package bot

import (
	"context"
	"fmt"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	goredis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	// stateAwaitingRegenConfirm is the FSM state name while waiting for confirm/cancel.
	stateAwaitingRegenConfirm = "awaiting_regen_confirm"

	// regenConfirmTTL is how long the confirmation prompt waits before expiring.
	regenConfirmTTL = 5 * time.Minute

	// regenRateLimitPerDay is the maximum successful key rotations allowed per user per 24h.
	regenRateLimitPerDay = 3

	// regenRateLimitTTL is the Redis key TTL for the rate-limit counter.
	regenRateLimitTTL = 24 * time.Hour
)

// regenRLKey returns the Redis rate-limit key for a user.
// Renamed from regen_rl: → regenkey_rate_limit: for searchability (H2 fix).
func regenRLKey(userID string) string {
	return "regenkey_rate_limit:" + userID
}

// HandleRegenKey handles the /regenkey command and the key:regen callback button.
// Rate-limit is PEEKED (read-only GET) synchronously before FSM transition.
// No INCR here — counter increments only on successful key issuance.
func HandleRegenKey(ctx context.Context, deps *Deps, api *tgbotapi.BotAPI, update tgbotapi.Update) error {
	chatID := updateChatID(update)

	user, ok := UserFromCtx(ctx)
	if !ok || user.ID.String() == "00000000-0000-0000-0000-000000000000" {
		replyText(api, update, "Vui lòng dùng /start để khởi tạo tài khoản trước.")
		return nil
	}

	// PEEK rate limit (read-only — no INCR).
	if deps.Rdb != nil {
		limited, err := peekRegenRateLimit(ctx, deps, user.ID.String())
		if err != nil {
			// Redis failure: fail open (allow the action) but log.
			deps.Log.Warn("HandleRegenKey: rate-limit peek failed, allowing",
				zap.String("user_id", user.ID.String()), zap.Error(err))
		} else if limited {
			msg := tgbotapi.NewMessage(chatID,
				renderTplCtx(ctx, deps, tplRegenRateLimited, nil))
			_, err = api.Send(msg)
			return err
		}
	}

	// Save FSM state so the confirm/cancel callback can be validated.
	if deps.Rdb != nil {
		stateStore := NewStore(deps.Rdb)
		if saveErr := stateStore.Save(ctx, updateTelegramID(update),
			State{Name: stateAwaitingRegenConfirm}, regenConfirmTTL); saveErr != nil {
			deps.Log.Warn("HandleRegenKey: failed to save FSM state",
				zap.String("user_id", user.ID.String()), zap.Error(saveErr))
			// Non-fatal: proceed anyway; confirm callback will still work without FSM guard.
		}
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Xác nhận", "key:regen:confirm"),
			tgbotapi.NewInlineKeyboardButtonData("❌ Huỷ", "key:regen:cancel"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, renderTplCtx(ctx, deps, tplKeyRegenConfirm, nil))
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = keyboard

	_, err := api.Send(msg)
	return err
}

// handleRegenConfirmCallback processes the "✅ Xác nhận" inline button.
// Flow: FSM revalidate → peek rate limit → Issue → INCR on success → show plaintext ONCE.
func handleRegenConfirmCallback(ctx context.Context, deps *Deps, api *tgbotapi.BotAPI, update tgbotapi.Update) error {
	if update.CallbackQuery == nil {
		return nil
	}

	// Always answer the callback to remove the loading spinner.
	cb := tgbotapi.NewCallback(update.CallbackQuery.ID, "")
	_, _ = api.Request(cb)

	tgID := updateTelegramID(update)

	// M4: Revalidate FSM state — reject stale or replayed callbacks.
	if deps.Rdb != nil {
		stateStore := NewStore(deps.Rdb)
		state, err := stateStore.Load(ctx, tgID)
		if err != nil || state.Name != stateAwaitingRegenConfirm {
			// Stale callback or replay attack — user never entered confirm flow.
			_, _ = api.Request(tgbotapi.NewCallback(update.CallbackQuery.ID, "Phiên xác nhận đã hết hạn"))
			replyText(api, update, "⏱ Phiên xác nhận đã hết hạn. Vui lòng /regenkey lại.")
			return nil
		}
	}

	user, ok := UserFromCtx(ctx)
	if !ok || deps.KeyService == nil {
		replyText(api, update, "⚠️ Hệ thống gặp sự cố. Vui lòng thử lại sau.")
		return nil
	}

	// PEEK rate limit again before issuing (guards edge case: window expired between
	// entry point peek and confirmation, then user spammed confirm rapidly).
	if deps.Rdb != nil {
		limited, err := peekRegenRateLimit(ctx, deps, user.ID.String())
		if err != nil {
			deps.Log.Warn("handleRegenConfirmCallback: rate-limit peek failed, allowing",
				zap.String("user_id", user.ID.String()), zap.Error(err))
		} else if limited {
			// Clear FSM — user must restart flow after rate limit expires.
			stateStore := NewStore(deps.Rdb)
			_ = stateStore.Clear(ctx, tgID)
			chatID := updateChatID(update)
			msg := tgbotapi.NewMessage(chatID,
				renderTplCtx(ctx, deps, tplRegenRateLimited, nil))
			_, err = api.Send(msg)
			return err
		}
	}

	plaintext, prefix, err := deps.KeyService.Issue(ctx, user.ID)
	if err != nil {
		deps.Log.Error("handleRegenConfirmCallback: Issue failed",
			zap.String("user_id", user.ID.String()), zap.Error(err))
		// Clear FSM so user can retry cleanly.
		if deps.Rdb != nil {
			stateStore := NewStore(deps.Rdb)
			_ = stateStore.Clear(ctx, tgID)
		}
		replyText(api, update, "⚠️ Hệ thống gặp sự cố khi tạo key. Vui lòng thử lại sau.")
		return err
	}

	// Issue succeeded: NOW increment rate-limit counter (reflects actual rotations).
	if deps.Rdb != nil {
		if incrErr := incrRegenRateLimit(ctx, deps, user.ID.String()); incrErr != nil {
			// Fail open: key was issued successfully; just log the counter failure.
			deps.Log.Warn("handleRegenConfirmCallback: rate-limit INCR failed",
				zap.String("user_id", user.ID.String()), zap.Error(incrErr))
		}
	}

	// Clear FSM state after successful issuance.
	if deps.Rdb != nil {
		stateStore := NewStore(deps.Rdb)
		_ = stateStore.Clear(ctx, tgID)
	}

	// Log regeneration event — prefix only, NEVER plaintext.
	deps.Log.Info("api key regenerated",
		zap.String("user_id", user.ID.String()),
		zap.String("key_prefix", prefix),
	)

	// Show plaintext ONCE with prominent warning. Switching to Markdown:
	// the template wraps {{.Key}} in backticks (code-block) which is safer
	// than raw HTML for an alphanumeric key (no escape needed for sbf_live_*).
	chatID := updateChatID(update)
	text := renderTplCtx(ctx, deps, tplKeyRegenDone, struct {
		Key string
	}{
		Key: plaintext,
	})

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	_, err = api.Send(msg)
	return err
}

// handleRegenCancelCallback processes the "❌ Huỷ" inline button.
// M4: Revalidates FSM state before clearing, then edits the original message to "Đã huỷ."
func handleRegenCancelCallback(ctx context.Context, deps *Deps, api *tgbotapi.BotAPI, update tgbotapi.Update) error {
	if update.CallbackQuery == nil {
		return nil
	}

	cb := tgbotapi.NewCallback(update.CallbackQuery.ID, "")
	_, _ = api.Request(cb)

	tgID := updateTelegramID(update)

	// M4: Revalidate FSM — stale cancel is harmless but we still clear state if found.
	if deps.Rdb != nil {
		stateStore := NewStore(deps.Rdb)
		state, err := stateStore.Load(ctx, tgID)
		if err != nil || state.Name != stateAwaitingRegenConfirm {
			// Stale callback — just return; no state to clear.
			return nil
		}
		_ = stateStore.Clear(ctx, tgID)
	}

	// Edit the original message in-place to "Đã huỷ." to keep the chat clean.
	// update.CallbackQuery.Message is guaranteed non-nil by dispatchCallback guard (H1).
	if update.CallbackQuery.Message != nil {
		edit := tgbotapi.NewEditMessageText(
			update.CallbackQuery.Message.Chat.ID,
			update.CallbackQuery.Message.MessageID,
			renderTplCtx(ctx, deps, tplKeyRegenCancelled, nil),
		)
		if _, err := api.Send(edit); err != nil {
			// Editing may fail on old/forwarded messages — not critical.
			deps.Log.Debug("handleRegenCancelCallback: edit message failed", zap.Error(err))
		}
	}

	return nil
}

// peekRegenRateLimit reads the current counter without incrementing.
// Returns (true, nil) if the limit is already reached, (false, nil) if allowed,
// (false, err) on Redis failure (caller decides fail-open or fail-closed).
func peekRegenRateLimit(ctx context.Context, deps *Deps, userID string) (limited bool, err error) {
	key := regenRLKey(userID)
	val, err := deps.Rdb.Get(ctx, key).Int()
	if err == goredis.Nil {
		// Key does not exist → counter = 0 → not limited.
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("peekRegenRateLimit: GET %s: %w", key, err)
	}
	return val >= regenRateLimitPerDay, nil
}

// incrRegenRateLimit increments the per-user daily counter after a successful Issue.
// Uses INCR + EXPIRE pipelined: INCR is atomic; EXPIRE is idempotent (only sets TTL
// if the key is new, preventing TTL reset on subsequent increments).
func incrRegenRateLimit(ctx context.Context, deps *Deps, userID string) error {
	key := regenRLKey(userID)
	_, err := deps.Rdb.Pipelined(ctx, func(p goredis.Pipeliner) error {
		p.Incr(ctx, key)
		p.Expire(ctx, key, regenRateLimitTTL)
		return nil
	})
	if err != nil {
		return fmt.Errorf("incrRegenRateLimit: pipeline: %w", err)
	}
	return nil
}
