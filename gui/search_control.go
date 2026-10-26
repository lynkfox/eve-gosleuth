package gui

import (
	"time"

	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// The questions a running search asks the frontend, each answered with AnswerPrompt
const (
	largeFetchEvent = "large-fetch" // a search that has fetched retrieval.MaxUncachedKillmails killmails may fetch the rest, see LargeFetch
	dateLimitEvent  = "date-limit"  // the newest killmail is older than the date limit, may the search ignore it, see DateLimit
)

// rateLimitedEvent tells the frontend the ESI has rate limited requests, with the time they are held until
const rateLimitedEvent = "rate-limited"

// LargeFetch is what the frontend's warning shows: how many killmails the search has fetched, and how many more not cached are left
type LargeFetch struct {
	Fetched   int `json:"fetched"`
	Remaining int `json:"remaining"`
}

// DateLimit is what the frontend's date limit question shows: the newest killmail's time, and the limit it is older than
type DateLimit struct {
	Newest    time.Time `json:"newest"`
	NewerThan time.Time `json:"newerThan"`
}

/*
	ask raises a question for the frontend and waits for AnswerPrompt. It is called while the search holds a.mu.

Parameters:
  - event: the question's Wails event, ie largeFetchEvent
  - details: what the frontend shows with it

Returns:
  - bool: the answer, false when the app closes first or the search is stopped
*/
func (a *App) ask(event string, details any) bool {
	// an answer left over from a question that was never asked, ie from StopSearch, is not this one's
	select {
	case <-a.promptAnswers:
	default:
	}
	if a.stopRequested.Load() {
		return false
	}

	runtime.EventsEmit(a.ctx, event, details)
	select {
	case answer := <-a.promptAnswers:
		return answer
	case <-a.ctx.Done():
		return false
	}
}

// confirmLargeFetch is CollectKillmails's ConfirmOverLimit: true to fetch the remaining killmails not cached, past the ESI's rate limit
func (a *App) confirmLargeFetch(remaining int) bool {
	comm.ReportProgress("Waiting to confirm fetching %d more killmails not cached", remaining)
	return a.ask(largeFetchEvent, LargeFetch{Fetched: retrieval.MaxUncachedKillmails, Remaining: remaining})
}

// confirmIgnoreDate is CollectKillmails's ConfirmIgnoreDate: true to collect without the date limit, since even the newest killmail is older than it
func (a *App) confirmIgnoreDate(newerThan time.Time) func(newest time.Time) bool {
	return func(newest time.Time) bool {
		comm.ReportProgress("The newest killmail, %s, is older than the %s date limit, waiting to confirm ignoring it", api.FormatShortDate(newest), api.FormatShortDate(newerThan))
		return a.ask(dateLimitEvent, DateLimit{Newest: newest, NewerThan: newerThan})
	}
}

/*
	AnswerPrompt answers the question a search raised, ie largeFetchEvent.
	Unlike the other App methods it does not take a.mu, since the waiting search holds it. It only hands the answer over.

Parameters:
  - yes: true to go ahead, ie fetch the rest or ignore the date limit
*/
func (a *App) AnswerPrompt(yes bool) {
	select {
	case a.promptAnswers <- yes:
	default: // a search asks one question at a time, so an answer is already waiting
	}
}

// reportRateLimited is the comm.RateLimitReporter that tells the frontend, which shows a warning
func (a *App) reportRateLimited(until time.Time) {
	runtime.EventsEmit(a.ctx, rateLimitedEvent, until)
}

/*
StopSearch stops the running search wherever it is: no more ESI requests are sent for it, so its killmail collection ends with those collected,
and the rest of the report is built from what is cached. Like AnswerPrompt it does not take a.mu, which the running search holds.
*/
func (a *App) StopSearch() {
	a.stopRequested.Store(true)
	a.esiGuard.Halt()
	comm.ReportProgress("Stopping the search")
	// a search waiting on a question gets no, and stops with what it has
	a.AnswerPrompt(false)
}
