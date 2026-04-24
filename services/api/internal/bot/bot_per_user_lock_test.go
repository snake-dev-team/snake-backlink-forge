package bot

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestPerUserLock_SameIDReturnsSamePointer verifies that two calls with the
// same tgID return the exact same *userLock pointer (LoadOrStore semantics).
func TestPerUserLock_SameIDReturnsSamePointer(t *testing.T) {
	b := &Bot{}

	lk1 := b.perUserLock(1001)
	lk2 := b.perUserLock(1001)

	if lk1 != lk2 {
		t.Errorf("expected same *userLock pointer for tgID=1001, got different pointers")
	}
}

// TestPerUserLock_DifferentIDsReturnDifferentPointers verifies isolation between users.
func TestPerUserLock_DifferentIDsReturnDifferentPointers(t *testing.T) {
	b := &Bot{}

	lk1 := b.perUserLock(1001)
	lk2 := b.perUserLock(1002)

	if lk1 == lk2 {
		t.Errorf("expected different *userLock pointers for different tgIDs, got same")
	}
}

// TestPerUserLock_SerializesConcurrentUpdates verifies that two goroutines
// processing updates for the same tgID run sequentially, not in parallel.
//
// Design: goroutine A acquires the lock and sleeps 50ms; goroutine B should
// not start its critical section until A releases. We capture timestamps of
// each goroutine's start-of-critical-section and assert they do not overlap.
func TestPerUserLock_SerializesConcurrentUpdates(t *testing.T) {
	b := &Bot{}
	const tgID = int64(42)

	var (
		startA, endA atomic.Int64 // unix nanoseconds
		startB, endB atomic.Int64
		wg           sync.WaitGroup
	)

	wg.Add(2)

	// Goroutine A: holds lock for 60ms.
	go func() {
		defer wg.Done()
		lk := b.perUserLock(tgID)
		lk.mu.Lock()
		defer lk.mu.Unlock()

		startA.Store(time.Now().UnixNano())
		time.Sleep(60 * time.Millisecond)
		endA.Store(time.Now().UnixNano())
	}()

	// Give goroutine A a head start so it acquires the lock first.
	time.Sleep(5 * time.Millisecond)

	// Goroutine B: should block until A releases.
	go func() {
		defer wg.Done()
		lk := b.perUserLock(tgID)
		lk.mu.Lock()
		defer lk.mu.Unlock()

		startB.Store(time.Now().UnixNano())
		time.Sleep(5 * time.Millisecond)
		endB.Store(time.Now().UnixNano())
	}()

	wg.Wait()

	sA, eA := startA.Load(), endA.Load()
	sB := startB.Load()

	// B must not start before A ends — i.e. no overlap.
	if sB < eA {
		t.Errorf("goroutines ran concurrently: A [%d..%d], B started at %d (before A ended)",
			sA, eA, sB)
	}
}

// TestPerUserLock_DifferentUsersRunConcurrently verifies that two updates for
// DIFFERENT tgIDs are NOT serialized — they should overlap.
func TestPerUserLock_DifferentUsersRunConcurrently(t *testing.T) {
	b := &Bot{}

	var (
		startA atomic.Int64
		startB atomic.Int64
		wg     sync.WaitGroup
	)

	wg.Add(2)

	go func() {
		defer wg.Done()
		lk := b.perUserLock(int64(100))
		lk.mu.Lock()
		defer lk.mu.Unlock()
		startA.Store(time.Now().UnixNano())
		time.Sleep(50 * time.Millisecond)
	}()

	go func() {
		defer wg.Done()
		lk := b.perUserLock(int64(200))
		lk.mu.Lock()
		defer lk.mu.Unlock()
		startB.Store(time.Now().UnixNano())
		time.Sleep(50 * time.Millisecond)
	}()

	wg.Wait()

	sA, sB := startA.Load(), startB.Load()
	diff := sB - sA
	if diff < 0 {
		diff = -diff
	}

	// Both goroutines should have started within 30ms of each other (concurrent).
	const maxDiffNs = 30 * int64(time.Millisecond)
	if diff > maxDiffNs {
		t.Errorf("different users were unexpectedly serialized: startA=%d startB=%d diff=%dms",
			sA, sB, diff/int64(time.Millisecond))
	}
}

// TestPersistOffset_MaxSemantics verifies that persistOffset keeps the maximum
// value even when called out of order (simulating goroutine reordering).
func TestPersistOffset_MaxSemantics(t *testing.T) {
	b := &Bot{}

	b.persistOffset(5)
	b.persistOffset(3) // lower — must not overwrite
	b.persistOffset(7)
	b.persistOffset(6) // lower than 7 — must not overwrite

	if got := b.maxOffset.Load(); got != 7 {
		t.Errorf("maxOffset: got %d, want 7", got)
	}
}
