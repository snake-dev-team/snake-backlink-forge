// transaction_service_volume_test.go — Phase 05 volume + format tests.
// Kept separate from transaction_service_test.go to honour 200-line limit.
// TestProviderRefFormat_1000Samples: 1000 CreateTopupIntent calls → all provider_refs
// match ^[A-F0-9]{12}$. Uses insertTestUserLargeRange to avoid tgID collisions.
package service_test

import (
	"context"
	"testing"
)

// TestProviderRefFormat_1000Samples verifies [F2]: every generated order code is
// exactly 12 uppercase hex characters. Runs 1000 iterations across distinct users.
func TestProviderRefFormat_1000Samples(t *testing.T) {
	pool := newTestPool(t)
	svc := newTestTxService(t, pool)

	packages := []string{
		"standard_starter_50", "standard_basic_100", "standard_pro_200", "standard_max_300",
		"premium_starter_50", "premium_basic_100", "premium_pro_200", "premium_max_300",
		"combo_p100_s50", "combo_p200_s100",
	}

	const samples = 1000
	fails := 0

	for i := 0; i < samples; i++ {
		// Wide-range helper avoids tgID collision with other tests' 1M–2M range.
		userID := insertTestUserLargeRange(t, pool)
		pkgCode := packages[i%len(packages)]

		tx, _, err := svc.CreateTopupIntent(context.Background(), userID, pkgCode)
		if err != nil {
			t.Errorf("sample %d: CreateTopupIntent error: %v", i, err)
			fails++
			continue
		}
		if tx.ProviderRef == nil {
			t.Errorf("sample %d: ProviderRef is nil", i)
			fails++
			continue
		}
		if !providerRefRegex.MatchString(*tx.ProviderRef) {
			t.Errorf("sample %d: provider_ref %q does not match ^[A-F0-9]{12}$", i, *tx.ProviderRef)
			fails++
		}
	}

	if fails > 0 {
		t.Errorf("%d/%d samples failed provider_ref format check", fails, samples)
	}
}
