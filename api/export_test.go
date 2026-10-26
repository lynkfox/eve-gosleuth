package api

import "time"

// SetZkillRateLimitPause shortens the pause after zKillboard rate limits a page, for tests. Call the returned func to restore it.
func SetZkillRateLimitPause(pause time.Duration) func() {
	previous := zkillRateLimitPause
	zkillRateLimitPause = pause
	return func() { zkillRateLimitPause = previous }
}
