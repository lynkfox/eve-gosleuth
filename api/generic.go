package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"math"
	"net/http"
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
}

func (c APIConnection) UpdateHeaders(newHeaders map[string]string) {
	maps.Copy(c.Headers, newHeaders)
}

// Request performs an HTTP request with retries and backoff
func (c APIConnection) Request(method, endpoint string, body interface{}) (*http.Response, error) {
	// Build the full URL
	url := fmt.Sprintf("%s/%s", c.BaseURL, endpoint)

	// Serialize the body if provided
	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			comm.Logger.WithError(err).Error("Failed to serialize body of request")
			return nil, fmt.Errorf("%w: %w", ErrRequestFailedToSend, err)
		}
		bodyReader = bytes.NewBuffer(jsonBody)
	}

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
		resp, err = c.HTTPClient.Do(req)
		if err == nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
			comm.Logger.WithFields(defaultFields.AddTemporaryDebugOnlyFields(logrus.Fields{"status": resp.StatusCode})).Debug("Request succeeded")

			return resp, nil
		}

		// Close response body if available to avoid resource leaks
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}

		if attempt != c.MaxRetries {
			backoffDuration := c.Backoff * time.Duration(math.Pow(2, float64(attempt-1)))

			comm.Logger.WithFields(defaultFields.AddTemporaryDebugOnlyFields(logrus.Fields{"body": body, "status": resp.StatusCode,
				"nextBackoff": backoffDuration})).WithError(err).Warn("Request failed, retrying")
			time.Sleep(backoffDuration)
		}

	}

	comm.Logger.WithError(err).Error("Could not complete request")
	return nil, fmt.Errorf("%w after %d retries: %w", ErrRequestFailedToComplete, c.MaxRetries, err)
}
