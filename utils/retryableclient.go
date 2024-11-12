package utils

import (
	"bytes"
	"io"
	"math"
	"net/http"
	"time"
)

func backoff(retries int) time.Duration {
	return time.Duration(math.Pow(2, float64(retries))) * time.Second
}

func shouldRetry(err error, resp *http.Response) bool {
	if err != nil {
		return true
	}

	if resp.StatusCode == http.StatusBadGateway ||
		resp.StatusCode == http.StatusServiceUnavailable ||
		resp.StatusCode == http.StatusGatewayTimeout {
		return true
	}
	return false
}

func drainBody(resp *http.Response) {
	if resp == nil {
		return
	}
	if resp.Body != nil {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}

type retryableTransport struct {
	retries   int
	transport http.RoundTripper
}

func (t *retryableTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Clone the request body
	var bodyBytes []byte
	if req.Body != nil {
		bodyBytes, _ = io.ReadAll(req.Body)
		req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	}
	// Send the request
	resp, err := t.transport.RoundTrip(req)
	// Retry logic
	retries := 0
	for shouldRetry(err, resp) && retries < t.retries {
		// Wait for the specified backoff period
		time.Sleep(backoff(retries))
		// We're going to retry, consume any response to reuse the connection.
		drainBody(resp)
		// Clone the request body again
		if req.Body != nil {
			req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}
		// Retry the request
		resp, err = t.transport.RoundTrip(req)
		retries++
	}
	// Return the response
	return resp, err
}

func NewRetryableClient(times int) *http.Client {
	transport := &retryableTransport{
		retries: times,
		transport: &http.Transport{
			MaxIdleConns:      0,
			IdleConnTimeout:   0,
			DisableKeepAlives: false,
		},
	}

	return &http.Client{
		Transport: transport,
	}
}
