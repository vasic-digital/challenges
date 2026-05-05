package userflow

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Compile-time interface check.
var _ BrowserAdapter = (*PlaywrightHTTPServerAdapter)(nil)

func TestPlaywrightHTTPServerAdapter_Constructor(
	t *testing.T,
) {
	adapter := NewPlaywrightHTTPServerAdapter(
		"http://localhost:9500",
	)
	require.NotNil(t, adapter)
	assert.Equal(
		t, "http://localhost:9500", adapter.serverURL,
	)
	assert.NotNil(t, adapter.httpClient)
	assert.False(t, adapter.initialized)
}

func TestPlaywrightHTTPServerAdapter_Constructor_Variants(
	t *testing.T,
) {
	tests := []struct {
		name string
		url  string
	}{
		{
			name: "localhost",
			url:  "http://localhost:9500",
		},
		{
			name: "remote_host",
			url:  "http://192.168.1.100:8080",
		},
		{
			name: "https",
			url:  "https://secure.host:443",
		},
		{
			name: "empty_url",
			url:  "",
		},
		{
			name: "with_path",
			url:  "http://host:9500/api/v1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := NewPlaywrightHTTPServerAdapter(tt.url)
			require.NotNil(t, a)
			assert.Equal(t, tt.url, a.serverURL)
			assert.False(t, a.initialized)
		})
	}
}

func TestPlaywrightHTTPServerAdapter_HTTPClientTimeout(
	t *testing.T,
) {
	adapter := NewPlaywrightHTTPServerAdapter(
		"http://localhost:9500",
	)
	assert.Equal(
		t, 60*time.Second, adapter.httpClient.Timeout,
	)
}

func TestPlaywrightHTTPServerAdapter_Initialize_Success(
	t *testing.T,
) {
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/initialize", r.URL.Path)
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"status":"ok"}`))
			},
		),
	)
	defer server.Close()

	adapter := NewPlaywrightHTTPServerAdapter(server.URL)
	err := adapter.Initialize(
		context.Background(), BrowserConfig{},
	)
	require.NoError(t, err)
	assert.True(t, adapter.initialized)
}

func TestPlaywrightHTTPServerAdapter_Initialize_ServerError(
	t *testing.T,
) {
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(
					http.StatusInternalServerError,
				)
			},
		),
	)
	defer server.Close()

	adapter := NewPlaywrightHTTPServerAdapter(server.URL)
	err := adapter.Initialize(
		context.Background(), BrowserConfig{},
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "initialize")
	assert.False(t, adapter.initialized)
}

func TestPlaywrightHTTPServerAdapter_Initialize_ErrorField(
	t *testing.T,
) {
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(
					`{"error":"browser launch failed"}`,
				))
			},
		),
	)
	defer server.Close()

	adapter := NewPlaywrightHTTPServerAdapter(server.URL)
	err := adapter.Initialize(
		context.Background(), BrowserConfig{},
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "browser launch")
	assert.False(t, adapter.initialized)
}

func TestPlaywrightHTTPServerAdapter_Initialize_NoServer(
	t *testing.T,
) {
	adapter := NewPlaywrightHTTPServerAdapter(
		"http://localhost:19998",
	)
	err := adapter.Initialize(
		context.Background(), BrowserConfig{},
	)
	assert.Error(t, err)
	assert.False(t, adapter.initialized)
}

func TestPlaywrightHTTPServerAdapter_Navigate_Success(
	t *testing.T,
) {
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/navigate", r.URL.Path)
				body, _ := io.ReadAll(r.Body)
				var payload map[string]string
				_ = json.Unmarshal(body, &payload)
				assert.Equal(
					t, "https://example.com",
					payload["url"],
				)
				_, _ = w.Write([]byte(`{}`))
			},
		),
	)
	defer server.Close()

	adapter := NewPlaywrightHTTPServerAdapter(server.URL)
	err := adapter.Navigate(
		context.Background(), "https://example.com",
	)
	assert.NoError(t, err)
}

func TestPlaywrightHTTPServerAdapter_Navigate_Error(
	t *testing.T,
) {
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
			},
		),
	)
	defer server.Close()

	adapter := NewPlaywrightHTTPServerAdapter(server.URL)
	err := adapter.Navigate(
		context.Background(), "invalid://url",
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "400")
}

func TestPlaywrightHTTPServerAdapter_Click_Success(
	t *testing.T,
) {
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/click", r.URL.Path)
				body, _ := io.ReadAll(r.Body)
				var payload map[string]string
				_ = json.Unmarshal(body, &payload)
				assert.Equal(
					t, "#submit-btn", payload["selector"],
				)
				_, _ = w.Write([]byte(`{}`))
			},
		),
	)
	defer server.Close()

	adapter := NewPlaywrightHTTPServerAdapter(server.URL)
	err := adapter.Click(
		context.Background(), "#submit-btn",
	)
	assert.NoError(t, err)
}

func TestPlaywrightHTTPServerAdapter_Fill_Success(
	t *testing.T,
) {
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/fill", r.URL.Path)
				body, _ := io.ReadAll(r.Body)
				var payload map[string]string
				_ = json.Unmarshal(body, &payload)
				assert.Equal(
					t, "#email", payload["selector"],
				)
				assert.Equal(
					t, "test@example.com",
					payload["value"],
				)
				_, _ = w.Write([]byte(`{}`))
			},
		),
	)
	defer server.Close()

	adapter := NewPlaywrightHTTPServerAdapter(server.URL)
	err := adapter.Fill(
		context.Background(),
		"#email",
		"test@example.com",
	)
	assert.NoError(t, err)
}

func TestPlaywrightHTTPServerAdapter_Wait_Success(
	t *testing.T,
) {
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/wait", r.URL.Path)
				body, _ := io.ReadAll(r.Body)
				var payload map[string]interface{}
				_ = json.Unmarshal(body, &payload)
				assert.Equal(
					t, ".loading-done",
					payload["selector"],
				)
				assert.Equal(
					t, float64(5000),
					payload["timeout"],
				)
				_, _ = w.Write([]byte(`{}`))
			},
		),
	)
	defer server.Close()

	adapter := NewPlaywrightHTTPServerAdapter(server.URL)
	err := adapter.Wait(
		context.Background(),
		".loading-done",
		5*time.Second,
	)
	assert.NoError(t, err)
}

func TestPlaywrightHTTPServerAdapter_WaitForURL_Success(
	t *testing.T,
) {
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(
					t, "/waitForURL", r.URL.Path,
				)
				body, _ := io.ReadAll(r.Body)
				var payload map[string]interface{}
				_ = json.Unmarshal(body, &payload)
				assert.Equal(
					t, "**/dashboard",
					payload["url"],
				)
				assert.Equal(
					t, float64(10000),
					payload["timeout"],
				)
				_, _ = w.Write([]byte(`{}`))
			},
		),
	)
	defer server.Close()

	adapter := NewPlaywrightHTTPServerAdapter(server.URL)
	err := adapter.WaitForURL(
		context.Background(),
		"**/dashboard",
		10*time.Second,
	)
	assert.NoError(t, err)
}

func TestPlaywrightHTTPServerAdapter_Screenshot_WithData(
	t *testing.T,
) {
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(
					t, "/screenshot", r.URL.Path,
				)
				assert.Equal(
					t, http.MethodGet, r.Method,
				)
				w.Header().Set(
					"Content-Type", "application/json",
				)
				_, _ = w.Write([]byte(
					`{"data":"iVBORw0KGgo="}`,
				))
			},
		),
	)
	defer server.Close()

	adapter := NewPlaywrightHTTPServerAdapter(server.URL)
	data, err := adapter.Screenshot(
		context.Background(),
	)
	require.NoError(t, err)
	assert.Equal(t, []byte("iVBORw0KGgo="), data)
}

func TestPlaywrightHTTPServerAdapter_Screenshot_NoData(
	t *testing.T,
) {
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{"status":"ok"}`))
			},
		),
	)
	defer server.Close()

	adapter := NewPlaywrightHTTPServerAdapter(server.URL)
	data, err := adapter.Screenshot(
		context.Background(),
	)
	require.NoError(t, err)
	assert.Nil(t, data)
}

func TestPlaywrightHTTPServerAdapter_Screenshot_Error(
	t *testing.T,
) {
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(
					http.StatusInternalServerError,
				)
			},
		),
	)
	defer server.Close()

	adapter := NewPlaywrightHTTPServerAdapter(server.URL)
	data, err := adapter.Screenshot(
		context.Background(),
	)
	assert.Error(t, err)
	assert.Nil(t, data)
}

func TestPlaywrightHTTPServerAdapter_EvaluateJS_Success(
	t *testing.T,
) {
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/eval", r.URL.Path)
				body, _ := io.ReadAll(r.Body)
				var payload map[string]string
				_ = json.Unmarshal(body, &payload)
				assert.Equal(
					t, "document.title",
					payload["script"],
				)
				_, _ = w.Write([]byte(`{}`))
			},
		),
	)
	defer server.Close()

	adapter := NewPlaywrightHTTPServerAdapter(server.URL)
	result, err := adapter.EvaluateJS(
		context.Background(), "document.title",
	)
	require.NoError(t, err)
	// postJSON always returns "" on success.
	assert.Equal(t, "", result)
}

func TestPlaywrightHTTPServerAdapter_EvaluateJS_Error(
	t *testing.T,
) {
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
			},
		),
	)
	defer server.Close()

	adapter := NewPlaywrightHTTPServerAdapter(server.URL)
	_, err := adapter.EvaluateJS(
		context.Background(), "bad()",
	)
	assert.Error(t, err)
}

func TestPlaywrightHTTPServerAdapter_NetworkIntercept_NoOp(
	t *testing.T,
) {
	adapter := NewPlaywrightHTTPServerAdapter(
		"http://localhost:9500",
	)
	err := adapter.NetworkIntercept(
		context.Background(),
		"**/api/**",
		func(_ *InterceptedRequest) {},
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "NetworkIntercept not supported by HTTP adapter")
}

func TestPlaywrightHTTPServerAdapter_NetworkIntercept_NilHandler(
	t *testing.T,
) {
	adapter := NewPlaywrightHTTPServerAdapter(
		"http://localhost:9500",
	)
	err := adapter.NetworkIntercept(
		context.Background(),
		"",
		nil,
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "NetworkIntercept not supported by HTTP adapter")
}

func TestPlaywrightHTTPServerAdapter_IsVisible_True(
	t *testing.T,
) {
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				assert.Contains(
					t, r.URL.RawQuery, "selector=",
				)
				_, _ = w.Write(
					[]byte(`{"visible":true}`),
				)
			},
		),
	)
	defer server.Close()

	adapter := NewPlaywrightHTTPServerAdapter(server.URL)
	visible, err := adapter.IsVisible(
		context.Background(), "#dialog",
	)
	require.NoError(t, err)
	assert.True(t, visible)
}

func TestPlaywrightHTTPServerAdapter_IsVisible_False(
	t *testing.T,
) {
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write(
					[]byte(`{"visible":false}`),
				)
			},
		),
	)
	defer server.Close()

	adapter := NewPlaywrightHTTPServerAdapter(server.URL)
	visible, err := adapter.IsVisible(
		context.Background(), "#hidden",
	)
	require.NoError(t, err)
	assert.False(t, visible)
}

func TestPlaywrightHTTPServerAdapter_IsVisible_MissingField(
	t *testing.T,
) {
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{}`))
			},
		),
	)
	defer server.Close()

	adapter := NewPlaywrightHTTPServerAdapter(server.URL)
	visible, err := adapter.IsVisible(
		context.Background(), "#missing",
	)
	require.NoError(t, err)
	assert.False(t, visible)
}

func TestPlaywrightHTTPServerAdapter_IsVisible_ServerError(
	t *testing.T,
) {
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(
					http.StatusInternalServerError,
				)
			},
		),
	)
	defer server.Close()

	adapter := NewPlaywrightHTTPServerAdapter(server.URL)
	visible, err := adapter.IsVisible(
		context.Background(), "#err",
	)
	assert.Error(t, err)
	assert.False(t, visible)
}

func TestPlaywrightHTTPServerAdapter_SelectOption_Success(
	t *testing.T,
) {
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/select", r.URL.Path)
				body, _ := io.ReadAll(r.Body)
				var payload map[string]string
				_ = json.Unmarshal(body, &payload)
				assert.Equal(
					t, "#country", payload["selector"],
				)
				assert.Equal(
					t, "US", payload["value"],
				)
				_, _ = w.Write([]byte(`{}`))
			},
		),
	)
	defer server.Close()

	adapter := NewPlaywrightHTTPServerAdapter(server.URL)
	err := adapter.SelectOption(
		context.Background(), "#country", "US",
	)
	assert.NoError(t, err)
}

func TestPlaywrightHTTPServerAdapter_GetText_Success(
	t *testing.T,
) {
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				assert.Contains(
					t, r.URL.RawQuery, "selector=",
				)
				_, _ = w.Write(
					[]byte(`{"text":"Hello World"}`),
				)
			},
		),
	)
	defer server.Close()

	adapter := NewPlaywrightHTTPServerAdapter(server.URL)
	text, err := adapter.GetText(
		context.Background(), "h1",
	)
	require.NoError(t, err)
	assert.Equal(t, "Hello World", text)
}

func TestPlaywrightHTTPServerAdapter_GetText_Empty(
	t *testing.T,
) {
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{}`))
			},
		),
	)
	defer server.Close()

	adapter := NewPlaywrightHTTPServerAdapter(server.URL)
	text, err := adapter.GetText(
		context.Background(), ".empty",
	)
	require.NoError(t, err)
	assert.Equal(t, "", text)
}

func TestPlaywrightHTTPServerAdapter_GetAttribute_Success(
	t *testing.T,
) {
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				q := r.URL.Query()
				assert.NotEmpty(t, q.Get("selector"))
				assert.NotEmpty(t, q.Get("attr"))
				_, _ = w.Write(
					[]byte(`{"value":"https://link"}`),
				)
			},
		),
	)
	defer server.Close()

	adapter := NewPlaywrightHTTPServerAdapter(server.URL)
	val, err := adapter.GetAttribute(
		context.Background(), "a.link", "href",
	)
	require.NoError(t, err)
	assert.Equal(t, "https://link", val)
}

func TestPlaywrightHTTPServerAdapter_GetAttribute_Missing(
	t *testing.T,
) {
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{}`))
			},
		),
	)
	defer server.Close()

	adapter := NewPlaywrightHTTPServerAdapter(server.URL)
	val, err := adapter.GetAttribute(
		context.Background(), "div", "data-x",
	)
	require.NoError(t, err)
	assert.Equal(t, "", val)
}

func TestPlaywrightHTTPServerAdapter_WaitForSelector_Success(
	t *testing.T,
) {
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/wait", r.URL.Path)
				body, _ := io.ReadAll(r.Body)
				var payload map[string]interface{}
				_ = json.Unmarshal(body, &payload)
				assert.Equal(
					t, ".ready",
					payload["selector"],
				)
				assert.Equal(
					t, float64(3000),
					payload["timeout"],
				)
				_, _ = w.Write([]byte(`{}`))
			},
		),
	)
	defer server.Close()

	adapter := NewPlaywrightHTTPServerAdapter(server.URL)
	err := adapter.WaitForSelector(
		context.Background(),
		".ready",
		3*time.Second,
	)
	assert.NoError(t, err)
}

func TestPlaywrightHTTPServerAdapter_Close_Success(
	t *testing.T,
) {
	var called atomic.Bool
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/close" {
					called.Store(true)
				}
				_, _ = w.Write([]byte(`{}`))
			},
		),
	)
	defer server.Close()

	adapter := NewPlaywrightHTTPServerAdapter(server.URL)
	adapter.initialized = true

	err := adapter.Close(context.Background())
	assert.NoError(t, err)
	assert.False(t, adapter.initialized)
	assert.True(t, called.Load())
}

func TestPlaywrightHTTPServerAdapter_Close_AlreadyClosed(
	t *testing.T,
) {
	adapter := NewPlaywrightHTTPServerAdapter(
		"http://localhost:19998",
	)
	// Close when not initialized — still returns nil.
	err := adapter.Close(context.Background())
	assert.NoError(t, err)
	assert.False(t, adapter.initialized)
}

func TestPlaywrightHTTPServerAdapter_Available_Reachable(
	t *testing.T,
) {
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(
					t, "/currentURL", r.URL.Path,
				)
				_, _ = w.Write(
					[]byte(`{"url":"about:blank"}`),
				)
			},
		),
	)
	defer server.Close()

	adapter := NewPlaywrightHTTPServerAdapter(server.URL)
	assert.True(t, adapter.Available(
		context.Background(),
	))
}

func TestPlaywrightHTTPServerAdapter_Available_Unreachable(
	t *testing.T,
) {
	adapter := NewPlaywrightHTTPServerAdapter(
		"http://localhost:19998",
	)
	assert.False(t, adapter.Available(
		context.Background(),
	))
}

func TestPlaywrightHTTPServerAdapter_Available_ServerError(
	t *testing.T,
) {
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(
					http.StatusInternalServerError,
				)
			},
		),
	)
	defer server.Close()

	adapter := NewPlaywrightHTTPServerAdapter(server.URL)
	assert.False(t, adapter.Available(
		context.Background(),
	))
}

func TestPlaywrightHTTPServerAdapter_SetRecorder_NoOp(
	t *testing.T,
) {
	adapter := NewPlaywrightHTTPServerAdapter(
		"http://localhost:9500",
	)
	// SetRecorder should not panic even with nil.
	adapter.SetRecorder(nil)
	assert.NotNil(t, adapter)
}

func TestPlaywrightHTTPServerAdapter_BrowserAdapter(
	t *testing.T,
) {
	adapter := NewPlaywrightHTTPServerAdapter(
		"http://localhost:9500",
	)
	got := adapter.BrowserAdapter()
	assert.Same(t, adapter, got)
}

func TestPlaywrightHTTPServerAdapter_PostJSON_ServerErrorField(
	t *testing.T,
) {
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(
					`{"error":"element not found"}`,
				))
			},
		),
	)
	defer server.Close()

	adapter := NewPlaywrightHTTPServerAdapter(server.URL)
	err := adapter.Click(
		context.Background(), "#nonexistent",
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "element not found")
}

func TestPlaywrightHTTPServerAdapter_PostJSON_EmptyErrorField(
	t *testing.T,
) {
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{"error":""}`))
			},
		),
	)
	defer server.Close()

	adapter := NewPlaywrightHTTPServerAdapter(server.URL)
	err := adapter.Click(
		context.Background(), "#btn",
	)
	// Empty error field should not produce an error.
	assert.NoError(t, err)
}

func TestPlaywrightHTTPServerAdapter_PostJSON_InvalidJSON(
	t *testing.T,
) {
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`not json`))
			},
		),
	)
	defer server.Close()

	adapter := NewPlaywrightHTTPServerAdapter(server.URL)
	err := adapter.Navigate(
		context.Background(), "https://example.com",
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decode response")
}

func TestPlaywrightHTTPServerAdapter_GetJSON_InvalidJSON(
	t *testing.T,
) {
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{bad`))
			},
		),
	)
	defer server.Close()

	adapter := NewPlaywrightHTTPServerAdapter(server.URL)
	_, err := adapter.Screenshot(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decode response")
}

func TestPlaywrightHTTPServerAdapter_AllMethods_EndpointPaths(
	t *testing.T,
) {
	// Track which endpoints are hit.
	endpoints := make(map[string]bool)
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				endpoints[r.URL.Path] = true
				_, _ = w.Write([]byte(`{}`))
			},
		),
	)
	defer server.Close()

	adapter := NewPlaywrightHTTPServerAdapter(server.URL)
	ctx := context.Background()

	_ = adapter.Navigate(ctx, "https://x.com")
	_ = adapter.Click(ctx, "#a")
	_ = adapter.Fill(ctx, "#b", "v")
	_ = adapter.Wait(ctx, ".c", time.Second)
	_ = adapter.WaitForURL(ctx, "/d", time.Second)
	_, _ = adapter.EvaluateJS(ctx, "1+1")
	_ = adapter.SelectOption(ctx, "#e", "f")
	_ = adapter.WaitForSelector(ctx, ".g", time.Second)

	expected := []string{
		"/navigate", "/click", "/fill", "/wait",
		"/waitForURL", "/eval", "/select",
	}
	for _, ep := range expected {
		assert.True(
			t, endpoints[ep],
			"endpoint %s not called", ep,
		)
	}
}

func TestPlaywrightHTTPServerAdapter_GetJSON_EndpointPaths(
	t *testing.T,
) {
	paths := make(map[string]bool)
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				paths[r.URL.Path] = true
				_, _ = w.Write([]byte(`{}`))
			},
		),
	)
	defer server.Close()

	adapter := NewPlaywrightHTTPServerAdapter(server.URL)
	ctx := context.Background()

	_, _ = adapter.Screenshot(ctx)
	_, _ = adapter.IsVisible(ctx, "#x")
	_, _ = adapter.GetText(ctx, "p")
	_, _ = adapter.GetAttribute(ctx, "a", "href")

	expected := []string{
		"/screenshot", "/visible", "/text", "/attr",
	}
	for _, ep := range expected {
		assert.True(
			t, paths[ep],
			"endpoint %s not called", ep,
		)
	}
}
