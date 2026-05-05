// retry.go: Exponential backoff calculator for transient WP REST failures.
// Formula: 30s * 2^retryCount, capped at 30 minutes.
package worker

import "time"

const (
	backoffBase = 30 * time.Second
	backoffCap  = 30 * time.Minute
)

// Backoff returns the wait duration before re-queuing a job that failed with
// a transient error. retryCount is the value AFTER incrementing (i.e. 1 on
// first retry, 2 on second, etc.).
//
// Schedule:
//   retry 1 → 60s  (30 * 2^1)
//   retry 2 → 120s (30 * 2^2)
//   retry 3 → not called — job moves to DLQ instead
func Backoff(retryCount int) time.Duration {
	if retryCount <= 0 {
		return backoffBase
	}
	d := backoffBase
	for i := 0; i < retryCount; i++ {
		d *= 2
		if d >= backoffCap {
			return backoffCap
		}
	}
	return d
}

// BackoffSeconds returns Backoff as an integer number of seconds,
// suitable for passing to SQL INTERVAL arithmetic.
func BackoffSeconds(retryCount int) int {
	return int(Backoff(retryCount).Seconds())
}
