package analytics

import "time"

const (
	// UsageTrackingUrl is the URL to send usage tracking events to.
	UsageTrackingUrl = "https://t.bentoml.com"

	// EnvYataiDoNotTrack is the environment variable that can be set to disable tracking
	EnvYataiDoNotTrack = "YATAI_T_DO_NOT_TRACK"

	// EnvYataiDebugUsage is the environment variable that can be set to enable debug usage, should only be used during development
	EnvYataiDebugUsage = "YATAI_T_DEBUG_USAGE"
)

// YataiTrackingIntervalSeconds is the interval at which to send tracking
var YataiTrackingIntervalSeconds = 8 * 60 * 60 * time.Second
