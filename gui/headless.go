package gui

import (
	"context"

	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/sirupsen/logrus"
)

/*
NewHeadlessApp builds an App with no window, for a dev tool under cmd/ (ie cmd/tourgen) that wants the frontend's JSON exactly as the App's methods build it.
It reads the user's settings and cache as the app does. A search's questions are answered yes, and its progress and rate limits are logged.
A package function rather than a method, so Wails never offers it to the frontend.

Parameters:
  - version: the version the App reports, ie "dev"

Returns:
  - *App: ready to call, ie SearchTarget then CollectIntel
  - func(): saves and closes the cache, to call when done
*/
func NewHeadlessApp(version string) (*App, func()) {
	a := NewApp(version)
	a.headless = true
	a.ctx = context.Background()
	comm.InitLogger(logrus.InfoLevel)
	comm.SetProgressReporter(func(message string) { comm.Logger.Info(message) })
	comm.SetRateLimitReporter(a.reportRateLimited)
	a.load()
	a.markReady()
	return a, func() { a.shutdown(a.ctx) }
}
