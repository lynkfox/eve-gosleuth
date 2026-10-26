package comm

import "fmt"

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
