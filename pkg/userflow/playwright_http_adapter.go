package userflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// PlaywrightHTTPServerAdapter connects to a local HTTP server
// that maintains a persistent Playwright browser connection.
type PlaywrightHTTPServerAdapter struct {
	serverURL   string
	httpClient  *http.Client
	initialized bool
}

// Compile-time interface check.
var _ BrowserAdapter = (*PlaywrightHTTPServerAdapter)(nil)

// NewPlaywrightHTTPServerAdapter creates an adapter that connects
// to a local HTTP server maintaining the Playwright browser.
func NewPlaywrightHTTPServerAdapter(
	serverURL string,
) *PlaywrightHTTPServerAdapter {
	return &PlaywrightHTTPServerAdapter{
		serverURL:  serverURL,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

func (a *PlaywrightHTTPServerAdapter) postJSON(endpoint string, payload interface{}) (string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal payload: %w", err)
	}

	resp, err := a.httpClient.Post(
		a.serverURL+endpoint,
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return "", fmt.Errorf("HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	if errMsg, ok := result["error"].(string); ok && errMsg != "" {
		return "", fmt.Errorf("server error: %s", errMsg)
	}

	return "", nil
}

func (a *PlaywrightHTTPServerAdapter) getJSON(endpoint string) (map[string]interface{}, error) {
	resp, err := a.httpClient.Get(a.serverURL + endpoint)
	if err != nil {
		return nil, fmt.Errorf("HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return result, nil
}

// Initialize connects to the browser via the HTTP server.
func (a *PlaywrightHTTPServerAdapter) Initialize(
	ctx context.Context, config BrowserConfig,
) error {
	_, err := a.postJSON("/initialize", map[string]interface{}{})
	if err != nil {
		return fmt.Errorf("initialize: %w", err)
	}
	a.initialized = true
	return nil
}

// Navigate loads the given URL.
func (a *PlaywrightHTTPServerAdapter) Navigate(
	ctx context.Context, url string,
) error {
	_, err := a.postJSON("/navigate", map[string]string{"url": url})
	return err
}

// Click performs a click on the element.
func (a *PlaywrightHTTPServerAdapter) Click(
	ctx context.Context, selector string,
) error {
	_, err := a.postJSON("/click", map[string]string{"selector": selector})
	return err
}

// Fill types a value into an input.
func (a *PlaywrightHTTPServerAdapter) Fill(
	ctx context.Context, selector, value string,
) error {
	_, err := a.postJSON("/fill", map[string]string{
		"selector": selector,
		"value":    value,
	})
	return err
}

// Wait waits for a selector to appear.
func (a *PlaywrightHTTPServerAdapter) Wait(
	ctx context.Context, selector string, timeout time.Duration,
) error {
	_, err := a.postJSON("/wait", map[string]interface{}{
		"selector": selector,
		"timeout":  int(timeout.Milliseconds()),
	})
	return err
}

// WaitForURL waits for the URL to match a pattern.
func (a *PlaywrightHTTPServerAdapter) WaitForURL(
	ctx context.Context, pattern string, timeout time.Duration,
) error {
	_, err := a.postJSON("/waitForURL", map[string]interface{}{
		"url":     pattern,
		"timeout": int(timeout.Milliseconds()),
	})
	return err
}

// Screenshot captures a screenshot.
func (a *PlaywrightHTTPServerAdapter) Screenshot(
	ctx context.Context,
) ([]byte, error) {
	result, err := a.getJSON("/screenshot")
	if err != nil {
		return nil, err
	}
	if data, ok := result["data"].(string); ok {
		return []byte(data), nil
	}
	return nil, nil
}

// EvaluateJS executes JavaScript.
func (a *PlaywrightHTTPServerAdapter) EvaluateJS(
	ctx context.Context, script string,
) (string, error) {
	result, err := a.postJSON("/eval", map[string]string{"script": script})
	return result, err
}

// NetworkIntercept is not supported by the HTTP server adapter.
// Use a direct Playwright connection for network interception.
func (a *PlaywrightHTTPServerAdapter) NetworkIntercept(
	ctx context.Context,
	pattern string,
	handler func(req *InterceptedRequest),
) error {
	return fmt.Errorf("NetworkIntercept not supported by HTTP adapter (use direct Playwright connection)")
}

// IsVisible checks if element is visible.
func (a *PlaywrightHTTPServerAdapter) IsVisible(
	ctx context.Context, selector string,
) (bool, error) {
	result, err := a.getJSON("/visible?selector=" + selector)
	if err != nil {
		return false, err
	}
	if visible, ok := result["visible"].(bool); ok {
		return visible, nil
	}
	return false, nil
}

// SelectOption selects an option in dropdown.
func (a *PlaywrightHTTPServerAdapter) SelectOption(
	ctx context.Context, selector, value string,
) error {
	_, err := a.postJSON("/select", map[string]string{
		"selector": selector,
		"value":    value,
	})
	return err
}

// GetText gets text content.
func (a *PlaywrightHTTPServerAdapter) GetText(
	ctx context.Context, selector string,
) (string, error) {
	result, err := a.getJSON("/text?selector=" + selector)
	if err != nil {
		return "", err
	}
	if text, ok := result["text"].(string); ok {
		return text, nil
	}
	return "", nil
}

// GetAttribute gets element attribute.
func (a *PlaywrightHTTPServerAdapter) GetAttribute(
	ctx context.Context, selector, attr string,
) (string, error) {
	result, err := a.getJSON(fmt.Sprintf("/attr?selector=%s&attr=%s", selector, attr))
	if err != nil {
		return "", err
	}
	if val, ok := result["value"].(string); ok {
		return val, nil
	}
	return "", nil
}

// WaitForSelector waits for element.
func (a *PlaywrightHTTPServerAdapter) WaitForSelector(
	ctx context.Context, selector string, timeout time.Duration,
) error {
	_, err := a.postJSON("/wait", map[string]interface{}{
		"selector": selector,
		"timeout":  int(timeout.Milliseconds()),
	})
	return err
}

// Close shuts down the browser connection.
func (a *PlaywrightHTTPServerAdapter) Close(
	ctx context.Context,
) error {
	a.postJSON("/close", nil)
	a.initialized = false
	return nil
}

// Available checks if the server is reachable.
func (a *PlaywrightHTTPServerAdapter) Available(
	ctx context.Context,
) bool {
	_, err := a.getJSON("/currentURL")
	return err == nil
}

// SetRecorder is not used in this adapter (HTTP server handles recording).
func (a *PlaywrightHTTPServerAdapter) SetRecorder(_ RecorderAdapter) {
	_ = a // no-op: HTTP adapter does not support recording
}

// BrowserAdapter returns itself for chaining.
func (a *PlaywrightHTTPServerAdapter) BrowserAdapter() *PlaywrightHTTPServerAdapter {
	return a
}
