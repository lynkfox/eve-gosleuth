package api

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lynkfox/go_eve_sleuth/comm"
)

const (
	// ESI_MAX_IN_FLIGHT is the most requests sent to the ESI at once, across every search and lookup
	ESI_MAX_IN_FLIGHT = 20
	// ESI_ERROR_RESERVE is how many of the ESI's errors a minute are left unused. Below it, every request waits for the error window to reset.
	ESI_ERROR_RESERVE = 20
	// ESI_RATE_RESERVE is the share of a rate limit group's tokens left unused. Below it, the group's requests are spaced to the rate its tokens come back.
	ESI_RATE_RESERVE = 0.1
	// ESI_SUCCESS_TOKENS is what the ESI charges a rate limit group for a 2xx response, the most common cost
	ESI_SUCCESS_TOKENS = 2
)

/*
RateGuard keeps requests within the ESI's limits. One is shared by every request an ESIConnection makes, so parallel lookups can not overrun them together.

  - At most maxInFlight requests are sent at once.
  - The error limit (X-Esi-Error-Limit-Remain, 100 errors a minute): below ESI_ERROR_RESERVE remaining, every request waits until X-Esi-Error-Limit-Reset.
  - Rate limit groups (X-Ratelimit-Group, ie "killmail" with a limit of "3600/15m", a floating window): below ESI_RATE_RESERVE of the limit remaining,
    the group's requests are spaced so they use tokens no faster than the window gives them back.
  - A 429 pauses the group, and a 420 pauses everything, for the Retry-After the ESI asks for.
*/
type RateGuard struct {
	slots chan struct{}

	mu          sync.Mutex
	pausedUntil time.Time             // every request waits until then, after the error limit or a 420
	routeGroups map[string]string     // a route, ie killmails/{}/{}, to the rate limit group the ESI said it is in
	groups      map[string]*rateGroup // by group name
}

// rateGroup is what the ESI last said about one rate limit group
type rateGroup struct {
	limit       int           // tokens in the window
	window      time.Duration // ie 15m
	remaining   int
	pausedUntil time.Time // after a 429
	nextStart   time.Time // while spacing, the earliest the next request may start
}

// NewRateGuard creates a guard that lets at most maxInFlight requests run at once
func NewRateGuard(maxInFlight int) *RateGuard {
	return &RateGuard{
		slots:       make(chan struct{}, maxInFlight),
		routeGroups: map[string]string{},
		groups:      map[string]*rateGroup{},
	}
}

// idSegment matches a path segment that is an id or a hash, so killmails/1/abc and killmails/2/def share the route killmails/{}/{}
var idSegment = regexp.MustCompile(`^([0-9]+|[0-9a-f]{32,})$`)

// routeOf names an endpoint's route, with its ids and hashes replaced, ie characters/{}/corporationhistory
func routeOf(endpoint string) string {
	path, _, _ := strings.Cut(endpoint, "?")
	segments := strings.Split(strings.Trim(path, "/"), "/")
	for index, segment := range segments {
		if idSegment.MatchString(segment) {
			segments[index] = "{}"
		}
	}
	return strings.Join(segments, "/")
}

/*
	wait takes one of the in-flight slots, then waits out any pause or spacing for the route's group

Parameters:
  - route: the request's route, from routeOf

Returns:
  - func(): gives the slot back, called once the response has arrived
*/
func (g *RateGuard) wait(route string) func() {
	g.slots <- struct{}{}

	g.mu.Lock()
	now := time.Now()
	start := g.pausedUntil
	if group := g.groups[g.routeGroups[route]]; group != nil {
		if group.pausedUntil.After(start) {
			start = group.pausedUntil
		}
		if group.spacing() > 0 {
			// each request takes the next spaced start, so parallel requests line up behind each other
			if group.nextStart.After(start) {
				start = group.nextStart
			}
			if now.After(start) {
				start = now
			}
			group.nextStart = start.Add(group.spacing())
		}
	}
	g.mu.Unlock()

	if delay := time.Until(start); delay > 0 {
		comm.Logger.Debugf("[ESI] Waiting %s before %s to stay within the rate limits", delay.Round(time.Millisecond), route)
		time.Sleep(delay)
	}

	return func() { <-g.slots }
}

// spacing is the time between requests that keeps the group from running out, or 0 while it has tokens to spare
func (group *rateGroup) spacing() time.Duration {
	if group.limit == 0 || float64(group.remaining) > float64(group.limit)*ESI_RATE_RESERVE {
		return 0
	}
	return group.window * ESI_SUCCESS_TOKENS / time.Duration(group.limit)
}

// observe reads the limits from a response's headers, and pauses after a 420 or 429
func (g *RateGuard) observe(route string, resp *http.Response) {
	now := time.Now()
	header := resp.Header

	g.mu.Lock()
	defer g.mu.Unlock()

	var group *rateGroup
	if name := header.Get("X-Ratelimit-Group"); name != "" {
		g.routeGroups[route] = name
		group = g.groups[name]
		if group == nil {
			group = &rateGroup{}
			g.groups[name] = group
		}
		if limit, window, ok := parseRateLimit(header.Get("X-Ratelimit-Limit")); ok {
			group.limit, group.window = limit, window
		}
		if remaining, err := strconv.Atoi(header.Get("X-Ratelimit-Remaining")); err == nil {
			group.remaining = remaining
		}
	}

	if remain, err := strconv.Atoi(header.Get("X-Esi-Error-Limit-Remain")); err == nil && remain < ESI_ERROR_RESERVE {
		g.pauseAll(now.Add(secondsHeader(header, "X-Esi-Error-Limit-Reset")), "the error limit is nearly used up")
	}

	switch resp.StatusCode {
	case http.StatusTooManyRequests:
		until := now.Add(secondsHeader(header, "Retry-After"))
		if group != nil {
			group.pausedUntil = until
			comm.Logger.Warnf("[ESI] Rate limited on %s, pausing it until %s", route, until.Format(time.TimeOnly))
		} else {
			g.pauseAll(until, "rate limited")
		}
	case 420: // the ESI's own status for an exceeded error limit
		retry := secondsHeader(header, "Retry-After")
		if retry == 0 {
			retry = secondsHeader(header, "X-Esi-Error-Limit-Reset")
		}
		g.pauseAll(now.Add(retry), "the error limit was reached")
	}
}

// pauseAll holds every request until a time, keeping the later pause if one is already set. The caller must hold g.mu.
func (g *RateGuard) pauseAll(until time.Time, why string) {
	if until.After(g.pausedUntil) {
		g.pausedUntil = until
		comm.Logger.Warnf("[ESI] Pausing all requests until %s, %s", until.Format(time.TimeOnly), why)
	}
}

// parseRateLimit reads an X-Ratelimit-Limit, ie "3600/15m" is 3600 tokens a 15 minute window
func parseRateLimit(value string) (int, time.Duration, bool) {
	tokens, window, found := strings.Cut(value, "/")
	if !found {
		return 0, 0, false
	}
	limit, err := strconv.Atoi(tokens)
	if err != nil || limit <= 0 {
		return 0, 0, false
	}
	duration, err := time.ParseDuration(window)
	if err != nil || duration <= 0 {
		return 0, 0, false
	}
	return limit, duration, true
}

// secondsHeader reads a header that counts seconds, ie Retry-After. Missing or unreadable is a second, so a pause is never skipped.
func secondsHeader(header http.Header, name string) time.Duration {
	seconds, err := strconv.Atoi(header.Get(name))
	if err != nil || seconds < 1 {
		return time.Second
	}
	return time.Duration(seconds) * time.Second
}

// retryable reports whether a failed status may succeed if sent again. Other 4xx answers are final, and retrying them would only spend the ESI's error limit.
func retryable(status int) bool {
	return status >= 500 || status == http.StatusRequestTimeout || status == http.StatusTooManyRequests || status == 420
}
