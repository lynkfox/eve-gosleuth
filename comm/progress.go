package comm

import (
	"fmt"
	"time"
)

// ProgressReporter receives human readable status updates during long running work, such as the GUI ticker
type ProgressReporter func(message string)

var progressReporter ProgressReporter

// SetProgressReporter registers the reporter that ReportProgress forwards to. Pass nil to stop forwarding.
func SetProgressReporter(reporter ProgressReporter) {
	progressReporter = reporter
}

// ReportProgress logs a status update at Info level and forwards it to the progress reporter, if one is set
func ReportProgress(format string, args ...any) {
	message := fmt.Sprintf(format, args...)

	Logger.Info(message)

	if progressReporter != nil {
		progressReporter(message)
	}
}

// RateLimitReporter is told when the ESI has rate limited requests, and until when they are held, such as to show the GUI's warning
type RateLimitReporter func(until time.Time)

var rateLimitReporter RateLimitReporter

// SetRateLimitReporter registers the reporter that ReportRateLimited forwards to. Pass nil to stop forwarding.
func SetRateLimitReporter(reporter RateLimitReporter) {
	rateLimitReporter = reporter
}

// ReportRateLimited forwards that the ESI has rate limited requests until a time to the rate limit reporter, if one is set
func ReportRateLimited(until time.Time) {
	if rateLimitReporter != nil {
		rateLimitReporter(until)
	}
}
