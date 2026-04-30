// retry_consumer.go — Phase 06 [round-3]: BRPOP-based retry queue consumer.
//
// Consumes from Redis list "sepay_retry_queue" populated by classifyWebhookError when
// ProcessPaidTransaction returns a lock-contention or deadline error.
//
// Policy:
//   - Max 3 attempts per envelope; on 4th failure → sepay_dead_letter + admin alert.
//   - Backoff: min(60s, 2^attempts) seconds before replay (0 on first attempt).
//   - Backlog alert: LLEN > 400 (80% of 500 cap) → alert; reset when LLEN < 200.
//   - Dead-letter alert dedup: one alert per 5-minute bucket (time.Now().Unix()/300).
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/kekuta/snake-backlink-forge/services/api/internal/integration/sepay"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/notify"
	goredis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// RetryEnvelope is the JSON shape stored in sepay_retry_queue and sepay_dead_letter.
type RetryEnvelope struct {
	Payload    sepay.Payload `json:"payload"`
	Attempts   int           `json:"attempts"`
	OriginalTS int64         `json:"original_ts"`
}

// min64 returns the smaller of a and b. Explicit helper for clarity (Go 1.21 has built-in min,
// but being explicit avoids confusion with integer vs float overloads in older toolchains).
func min64(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// RetryQueueConsumer is a supervisor loop that calls runRetryConsumerOnce in a recover
// wrapper. If the inner function panics, the supervisor logs it and restarts.
// Exits only when ctx is cancelled (runRetryConsumerOnce returns false).
// [M1] Spec risk-table line 580: "on panic, recover() + restart loop".
// Exported so main.go can spawn it as a goroutine.
func RetryQueueConsumer(
	ctx context.Context,
	rdb *goredis.Client,
	svc *WebhookService,
	alertCh chan<- notify.AdminAlert,
	log *zap.Logger,
) {
	for {
		if !runRetryConsumerOnce(ctx, rdb, svc, alertCh, log) {
			return // ctx cancelled — clean exit
		}
		// runRetryConsumerOnce returned true only via panic-recover; loop restarts.
	}
}

// runRetryConsumerOnce runs the BRPOP loop body, recovering from panics.
// Returns false when ctx is cancelled (caller should stop).
// Returns true when a panic was recovered (caller should restart).
func runRetryConsumerOnce(
	ctx context.Context,
	rdb *goredis.Client,
	svc *WebhookService,
	alertCh chan<- notify.AdminAlert,
	log *zap.Logger,
) (continueLoop bool) {
	defer func() {
		if r := recover(); r != nil {
			log.Error("retry consumer: panic recovered — restarting loop",
				zap.Any("panic", r),
			)
			continueLoop = true
		}
	}()

	backlogAlertedHigh := false               // true after >400 alert; reset when LLEN < 200
	deadLetterAlerted := make(map[int64]bool) // 5-min bucket dedup for retry_dead_letter alerts

	for {
		if ctx.Err() != nil {
			return false
		}

		brpopCtx, cancel := context.WithCancel(ctx)
		resCh := make(chan []string, 1)
		errCh := make(chan error, 1)
		go func() {
			res, err := rdb.BRPop(brpopCtx, time.Second, "sepay_retry_queue").Result()
			if err != nil {
				errCh <- err
				return
			}
			resCh <- res
		}()

		var res []string
		select {
		case <-ctx.Done():
			cancel()
			return false
		case err := <-errCh:
			cancel()
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, goredis.Nil) || ctx.Err() != nil {
				continue
			}
			// Transient Redis error — log and loop (will retry BRPop).
			log.Warn("retry consumer: BRPOP failed", zap.Error(err))
			continue
		case res = <-resCh:
			cancel()
		}

		var env RetryEnvelope
		if err := json.Unmarshal([]byte(res[1]), &env); err != nil {
			log.Warn("retry consumer: envelope decode failed",
				zap.String("raw", res[1]),
				zap.Error(err),
			)
			continue // drop malformed envelope
		}

		// Exponential backoff: 0s on first attempt, then min(60s, 2^attempts).
		if env.Attempts > 0 {
			backoff := time.Duration(min64(60, 1<<env.Attempts)) * time.Second
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return false
			}
		}

		// Extract order code from content — must match to replay.
		match := sepay.OrderCodeRe.FindStringSubmatch(env.Payload.Content)
		if match == nil {
			log.Warn("retry consumer: no order code in envelope content — dropping",
				zap.String("content", env.Payload.Content),
			)
			continue
		}
		orderCode := strings.ToUpper(match[1])

		// Replay: sentinel short-circuits without DB call (test hook).
		var procErr error
		if orderCode == DeadLetterSentinel {
			procErr = ErrDeadLetterSentinel
		} else {
			replayCtx, replayCancel := context.WithTimeout(context.Background(), 30*time.Second)
			_, procErr = svc.ProcessPaidTransaction(replayCtx, orderCode, env.Payload)
			replayCancel()
		}

		if procErr == nil {
			log.Info("retry consumer: replay succeeded", zap.String("order_code", orderCode))
			continue
		}
		if ctx.Err() != nil {
			return false
		}

		// Failure: increment attempts and branch on max-retries.
		env.Attempts++
		if env.Attempts >= 3 {
			handleDeadLetter(ctx, rdb, alertCh, log, env, orderCode, procErr, deadLetterAlerted)
			continue
		}

		// Re-enqueue with incremented attempts; cap at 500.
		// Use background context: re-enqueue must persist regardless of consumer ctx state.
		data, _ := json.Marshal(env)
		_ = rdb.LPush(context.Background(), "sepay_retry_queue", data).Err()
		_ = rdb.LTrim(context.Background(), "sepay_retry_queue", 0, 499).Err()

		log.Warn("retry consumer: replay failed — re-enqueued",
			zap.String("order_code", orderCode),
			zap.Int("attempts", env.Attempts),
			zap.Error(procErr),
		)

		// Backlog alert: check LLEN after re-enqueue.
		backlogAlertedHigh = checkBacklogAlert(ctx, rdb, alertCh, log, backlogAlertedHigh)
	}
}

// handleDeadLetter moves the envelope to sepay_dead_letter and fires a deduped admin alert.
// Uses context.Background() for Redis writes so shutdown/cancel does not lose dead-letter entries.
func handleDeadLetter(
	_ context.Context, // consumer ctx intentionally unused — see background ctx comment
	rdb *goredis.Client,
	alertCh chan<- notify.AdminAlert,
	log *zap.Logger,
	env RetryEnvelope,
	orderCode string,
	procErr error,
	alerted map[int64]bool,
) {
	data, _ := json.Marshal(env)
	// Use background context: dead-letter writes MUST persist even when consumer ctx is cancelled.
	_ = rdb.LPush(context.Background(), "sepay_dead_letter", data).Err()

	log.Error("retry consumer: exhausted → dead-letter",
		zap.String("order_code", orderCode),
		zap.Int("attempts", env.Attempts),
		zap.Error(procErr),
	)

	// 5-minute bucket dedup prevents alert flood during retry storms.
	bucket := time.Now().Unix() / 300
	if !alerted[bucket] {
		sendRetryAlert(alertCh, notify.AdminAlert{
			Kind:      "retry_dead_letter",
			OrderCode: orderCode,
			Err:       fmt.Sprintf("attempts=%d err=%s", env.Attempts, procErr.Error()),
		})
		alerted[bucket] = true
		// GC buckets older than ~1 hour (12 × 5-min).
		for b := range alerted {
			if b < bucket-12 {
				delete(alerted, b)
			}
		}
	}
}

// checkBacklogAlert checks LLEN and fires/clears a backlog alert as appropriate.
// Uses context.Background() so it works even when consumer ctx is cancelled during shutdown.
// Returns updated backlogAlertedHigh flag.
func checkBacklogAlert(
	_ context.Context, // consumer ctx intentionally unused — use background for LLEN
	rdb *goredis.Client,
	alertCh chan<- notify.AdminAlert,
	log *zap.Logger,
	alreadyAlerting bool,
) bool {
	llen, err := rdb.LLen(context.Background(), "sepay_retry_queue").Result()
	if err != nil {
		return alreadyAlerting
	}
	if llen > 400 && !alreadyAlerting {
		log.Warn("retry queue backlog alert", zap.Int64("llen", llen))
		sendRetryAlert(alertCh, notify.AdminAlert{
			Kind: "retry_queue_backlog",
			Err:  fmt.Sprintf("queue len=%d (>80%% of 500 cap)", llen),
		})
		return true
	}
	if llen < 200 && alreadyAlerting {
		return false // reset flag when queue drains
	}
	return alreadyAlerting
}

// sendRetryAlert is a non-blocking send to alertCh; drops silently if full.
func sendRetryAlert(alertCh chan<- notify.AdminAlert, alert notify.AdminAlert) {
	if alertCh == nil {
		return
	}
	if alert.At.IsZero() {
		alert.At = time.Now()
	}
	select {
	case alertCh <- alert:
	default:
	}
}
