package tfutil

import (
	"os"
	"time"
)

// PollIntervalEnv overrides the resource state-polling interval with a Go
// duration string (e.g. "2s"). It exists for acceptance tests against the local
// testmode stack, where slices and POAs converge in seconds and the production
// 15s cadence only adds dead wait between polls. It is not a documented provider
// input; production deployments leave it unset and keep the built-in default.
const PollIntervalEnv = "FABRIC_POLL_INTERVAL"

// PollInterval returns the slice/POA poll interval: FABRIC_POLL_INTERVAL when it
// parses to a positive duration, otherwise def. Unset, malformed, and
// non-positive values all fall back to def so a bad override can never disable
// polling or busy-loop.
func PollInterval(def time.Duration) time.Duration {
	v := os.Getenv(PollIntervalEnv)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		return def
	}
	return d
}
