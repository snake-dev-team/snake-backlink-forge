// admin_service_lookup.go — Phase 08: /admin lookup heuristic search + result type.
// Imported by admin_service.go (same package). Kept separate to stay under 200 lines.
package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	sqlcdb "github.com/kekuta/snake-backlink-forge/services/api/internal/db/sqlc"
)

// LookupResult is the rich user summary returned by /admin lookup.
type LookupResult struct {
	User        sqlcdb.User
	Wallet      sqlcdb.Wallet
	ActiveKey   *sqlcdb.ApiKey   // nil if user has no active key
	LastTx      []sqlcdb.Transaction
	LastLedger  []sqlcdb.Ledger
	MaskedPhone string           // "+84***1234" or "" if no phone
}

// Lookup resolves a user by telegram_id, phone (E.164 prefix "+"), or key_prefix ("sbf_live_").
// Returns ErrAdminUserNotFound when no match.
func (s *AdminService) Lookup(ctx context.Context, identifier string) (LookupResult, error) {
	kind, val := parseLookupIdent(identifier)

	var user sqlcdb.User
	var err error

	switch kind {
	case "telegram_id":
		tgID, _ := strconv.ParseInt(val, 10, 64)
		user, err = s.q.GetUserByTelegramID(ctx, tgID)
	case "phone":
		user, err = s.q.GetUserByPhone(ctx, &val)
	case "key_prefix":
		user, err = s.q.GetUserByKeyPrefix(ctx, val)
	default:
		return LookupResult{}, fmt.Errorf("admin_service.Lookup: unrecognised identifier %q", identifier)
	}

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return LookupResult{}, ErrAdminUserNotFound
		}
		return LookupResult{}, fmt.Errorf("admin_service.Lookup: query user: %w", err)
	}

	result := LookupResult{User: user}

	// Wallet
	if result.Wallet, err = s.q.GetWalletByUser(ctx, user.ID); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return LookupResult{}, fmt.Errorf("admin_service.Lookup: wallet: %w", err)
	}

	// Active key (best-effort; key may not exist)
	key, keyErr := s.q.GetActiveKeyByUser(ctx, user.ID)
	if keyErr == nil {
		result.ActiveKey = &key
	}

	// Last 5 transactions
	result.LastTx, err = s.q.GetTxByUserPage(ctx, sqlcdb.GetTxByUserPageParams{
		UserID: user.ID,
		Limit:  5,
		Offset: 0,
	})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return LookupResult{}, fmt.Errorf("admin_service.Lookup: tx: %w", err)
	}

	// Last 5 ledger events
	result.LastLedger, err = s.q.GetLedgerPage(ctx, sqlcdb.GetLedgerPageParams{
		UserID: user.ID,
		Limit:  5,
		Offset: 0,
	})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return LookupResult{}, fmt.Errorf("admin_service.Lookup: ledger: %w", err)
	}

	// Masked phone: "+84***1234"
	if user.PhoneE164 != nil && len(*user.PhoneE164) >= 8 {
		p := *user.PhoneE164
		result.MaskedPhone = p[:4] + "***" + p[len(p)-4:]
	}

	return result, nil
}

// parseLookupIdent classifies an identifier string into (kind, value) for Lookup dispatch.
// - Pure int64 string → "telegram_id"
// - Starts with "+" → "phone"
// - Starts with "sbf_live_" → "key_prefix" (truncated to 12 chars)
// - Otherwise → "unknown"
func parseLookupIdent(s string) (kind, val string) {
	if _, err := strconv.ParseInt(s, 10, 64); err == nil {
		return "telegram_id", s
	}
	if strings.HasPrefix(s, "+") {
		return "phone", s
	}
	if strings.HasPrefix(s, "sbf_live_") {
		prefix := s
		if len(prefix) > 12 {
			prefix = prefix[:12]
		}
		return "key_prefix", prefix
	}
	return "unknown", s
}

// nowMinus24h returns the timestamp 24 hours before now.
// Shared by admin_service.go (Stats) and tests.
func nowMinus24h() time.Time {
	return time.Now().Add(-24 * time.Hour)
}
