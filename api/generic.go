package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"math"
	"net/http"
	"net/url"
	"time"

	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/sirupsen/logrus"
)

type RequestError struct {
	Message string
}

func (e *RequestError) Error() string {
	return e.Message
}

var ErrRequestFailedToSend = &RequestError{Message: "request failed to send"}
var ErrRequestFailedToComplete = &RequestError{Message: "request failed to complete"}

// StatusError is a request the server answered with an error status, ie the ESI's 404 for a deleted character
type StatusError struct {
	Host    string // ie esi.evetech.net
	Status  int
	Message string // the server's own explanation, when it sent one, ie Character not found
}

func (e *StatusError) Error() string {
	text := fmt.Sprintf("%s returned %d %s", e.Host, e.Status, http.StatusText(e.Status))
	if e.Message != "" {
		text += ": " + e.Message
	}
	return text
}

// newStatusError reads why a request failed from its response. The ESI and zKillboard explain errors as {"error": "..."}.
func newStatusError(req *http.Request, resp *http.Response) *StatusError {
	statusErr := &StatusError{Host: req.URL.Host, Status: resp.StatusCode}

	explanation := struct {
		Error string `json:"error"`
	}{}
	if body, err := io.ReadAll(io.LimitReader(resp.Body, 4096)); err == nil && json.Unmarshal(body, &explanation) == nil {
		statusErr.Message = explanation.Error
	}
	return statusErr
}

// IsNotFound reports whether a request failed because the server has no such entity, a 404, as opposed to failing to answer
func IsNotFound(err error) bool {
	var statusErr *StatusError
	return errors.As(err, &statusErr) && statusErr.Status == http.StatusNotFound
}

/*
	FailureReason says in a few words why a request failed, for progress messages

Parameters:
  - err: the error from a request, possibly wrapped

Returns:
  - string: ie "esi.evetech.net returned 502 Bad Gateway", "timed out reaching esi.evetech.net", or the error itself if it is not a request error
*/
func FailureReason(err error) string {
	var statusErr *StatusError
	if errors.As(err, &statusErr) {
		return statusErr.Error()
	}

	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		host := urlErr.URL
		if parsed, parseErr := url.Parse(urlErr.URL); parseErr == nil {
			host = parsed.Host
		}
		if urlErr.Timeout() {
			return "timed out reaching " + host
		}
		return "could not reach " + host
	}

	return err.Error()
}

type APIClient interface {
	Request(method, endpoint string, body interface{}) (*http.Response, error)
	UpdateHeaders(newHeaders map[string]string)
}

type APIConnection struct {
	BaseURL    string
	HTTPClient *http.Client
	Headers    map[string]string
	MaxRetries int
	Backoff    time.Duration
	Guard      *RateGuard // keeps requests within the server's rate limits, nil for none
}

func (c APIConnection) UpdateHeaders(newHeaders map[string]string) {
	maps.Copy(c.Headers, newHeaders)
}

// rateLimitHeaders are the response headers that say how close the app is to a server's rate limits, kept in the debug log
var rateLimitHeaders = []string{"X-Esi-Error-Limit-Remain", "X-Esi-Error-Limit-Reset", "X-Ratelimit-Group", "X-Ratelimit-Limit", "X-Ratelimit-Remaining", "X-Ratelimit-Used", "Retry-After"}

// responseFields are a response's log fields for the debug log: the method, its status, how long it took, and any rate limit headers. resp may be nil.
func responseFields(method string, resp *http.Response, sent time.Time) logrus.Fields {
	fields := logrus.Fields{"method": method, "ms": time.Since(sent).Milliseconds()}
	if resp == nil {
		return fields
	}
	fields["status"] = resp.StatusCode
	for _, header := range rateLimitHeaders {
		if value := resp.Header.Get(header); value != "" {
			fields[header] = value
		}
	}
	return fields
}

// Request performs an HTTP request with retries and backoff
func (c APIConnection) Request(method, endpoint string, body interface{}) (*http.Response, error) {
	// Build the full URL
	url := fmt.Sprintf("%s/%s", c.BaseURL, endpoint)

	// Serialize the body if provided
	var jsonBody []byte
	if body != nil {
		var err error
		jsonBody, err = json.Marshal(body)
		if err != nil {
			comm.Logger.WithError(err).Error("Failed to serialize body of request")
			return nil, fmt.Errorf("%w: %w", ErrRequestFailedToSend, err)
		}
	}

	route := routeOf(endpoint)

	// Retry loop with backoff
	var resp *http.Response
	var err error
	defaultFields := comm.LogFields{
		"url": url,
	}

	comm.Logger.WithFields(defaultFields.AddTemporaryDebugOnlyFields(logrus.Fields{"body": body})).Debug("Sending request")

	for attempt := 1; attempt <= c.MaxRetries; attempt++ {

		defaultFields.AddDebugOnlyFields(logrus.Fields{
			"attempt":    attempt,
			"maxRetries": c.MaxRetries,
		})

		// a reader is consumed by sending, so each attempt needs a fresh one
		var bodyReader io.Reader
		if jsonBody != nil {
			bodyReader = bytes.NewReader(jsonBody)
		}

		req, reqErr := http.NewRequest(method, url, bodyReader)
		if reqErr != nil {
			comm.Logger.WithFields(logrus.Fields(defaultFields)).WithError(err).Error("Could not create request")
			return nil, fmt.Errorf("failed to create request: %w", reqErr)
		}

		// Add headers
		for key, value := range c.Headers {
			req.Header.Set(key, value)
		}

		// Send the request
		sent := time.Now()
		if c.Guard != nil {
			release, refused := c.Guard.wait(route)
			if refused != nil {
				return nil, fmt.Errorf("%w: %w", ErrRequestFailedToComplete, refused)
			}
			resp, err = c.HTTPClient.Do(req)
			if err == nil {
				c.Guard.observe(route, resp)
			}
			release()
		} else {
			resp, err = c.HTTPClient.Do(req)
		}
		if err == nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
			comm.Logger.WithFields(defaultFields.AddTemporaryDebugOnlyFields(responseFields(method, resp, sent))).Debug("Request succeeded")

			return resp, nil
		}

		// an answered request failed on its status, keep why for the error
		if err == nil {
			err = newStatusError(req, resp)
		}

		// Close response body if available to avoid resource leaks
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}

		// a 404 or other final answer will not change, so it is not sent again
		if resp != nil && !retryable(resp.StatusCode) {
			break
		}

		// the guard now holds the rate limited group, so sending it again would only fail
		if c.Guard != nil && resp != nil && resp.StatusCode == http.StatusTooManyRequests {
			comm.Logger.WithError(err).Error("Could not complete request, rate limited")
			return nil, fmt.Errorf("%w: %w: %w", ErrRequestFailedToComplete, ErrRateLimited, err)
		}

		if attempt != c.MaxRetries {
			backoffDuration := c.Backoff * time.Duration(math.Pow(2, float64(attempt-1)))

			status := 0
			if resp != nil {
				status = resp.StatusCode
			}

			failed := logrus.Fields{"body": body, "status": status, "nextBackoff": backoffDuration}
			maps.Copy(failed, responseFields(method, resp, sent))
			comm.Logger.WithFields(defaultFields.AddTemporaryDebugOnlyFields(failed)).WithError(err).Warn("Request failed, retrying")
			// after a 420 the guard waits as long as the server asked, instead
			if c.Guard == nil || status != 420 {
				time.Sleep(backoffDuration)
			}
		}

	}

	// a 404 is an answer, ie a biomassed pilot, which the caller handles or logs as a failure itself
	if IsNotFound(err) {
		comm.Logger.WithError(err).Info("Request answered not found")
	} else {
		comm.Logger.WithError(err).Error("Could not complete request")
	}
	return nil, fmt.Errorf("%w: %w", ErrRequestFailedToComplete, err)
}
