package userflow

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Compile-time interface check.
var _ WebSocketFlowAdapter = (*GorillaWebSocketAdapter)(nil)

func TestGorillaWebSocketAdapter_Available_NotConnected(
	t *testing.T,
) {
	adapter := NewGorillaWebSocketAdapter()
	available := adapter.Available(context.Background())
	assert.False(t, available)
}

func TestGorillaWebSocketAdapter_Close_NotConnected(
	t *testing.T,
) {
	adapter := NewGorillaWebSocketAdapter()
	err := adapter.Close(context.Background())
	assert.NoError(t, err)
}

func TestGorillaWebSocketAdapter_Send_NotConnected(
	t *testing.T,
) {
	adapter := NewGorillaWebSocketAdapter()
	err := adapter.Send(
		context.Background(), []byte("test"),
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestGorillaWebSocketAdapter_Receive_NotConnected(
	t *testing.T,
) {
	adapter := NewGorillaWebSocketAdapter()
	msg, err := adapter.Receive(
		context.Background(), time.Second,
	)
	assert.Error(t, err)
	assert.Nil(t, msg)
	assert.Contains(t, err.Error(), "not connected")
}

func TestGorillaWebSocketAdapter_ReceiveAll_NotConnected(
	t *testing.T,
) {
	adapter := NewGorillaWebSocketAdapter()
	msgs, err := adapter.ReceiveAll(
		context.Background(), time.Second,
	)
	assert.Error(t, err)
	assert.Nil(t, msgs)
	assert.Contains(t, err.Error(), "not connected")
}

func TestGorillaWebSocketAdapter_SendAndReceive_NotConnected(
	t *testing.T,
) {
	adapter := NewGorillaWebSocketAdapter()
	msg, err := adapter.SendAndReceive(
		context.Background(),
		[]byte("test"),
		time.Second,
	)
	assert.Error(t, err)
	assert.Nil(t, msg)
}

func TestGorillaWebSocketAdapter_Connect_AlreadyConnected(
	t *testing.T,
) {
	// Start a WebSocket echo server.
	upgrader := websocket.Upgrader{
		CheckOrigin: func(_ *http.Request) bool {
			return true
		},
	}
	srv := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				conn, err := upgrader.Upgrade(
					w, r, nil,
				)
				if err != nil {
					return
				}
				defer conn.Close()
				// Keep alive until client disconnects.
				for {
					_, _, err := conn.ReadMessage()
					if err != nil {
						return
					}
				}
			},
		),
	)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(
		srv.URL, "http",
	)

	adapter := NewGorillaWebSocketAdapter()
	err := adapter.Connect(
		context.Background(), wsURL, nil,
	)
	require.NoError(t, err)
	defer adapter.Close(context.Background())

	// Second connect should fail.
	err = adapter.Connect(
		context.Background(), wsURL, nil,
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already connected")
}

func TestGorillaWebSocketAdapter_Connect_InvalidURL(
	t *testing.T,
) {
	adapter := NewGorillaWebSocketAdapter()
	err := adapter.Connect(
		context.Background(),
		"ws://localhost:19999/nonexistent",
		nil,
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "websocket connect")
}

func TestGorillaWebSocketAdapter_Connect_WithHeaders(
	t *testing.T,
) {
	var receivedHeaders http.Header
	upgrader := websocket.Upgrader{
		CheckOrigin: func(_ *http.Request) bool {
			return true
		},
	}
	srv := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				receivedHeaders = r.Header
				conn, err := upgrader.Upgrade(
					w, r, nil,
				)
				if err != nil {
					return
				}
				defer conn.Close()
				for {
					_, _, err := conn.ReadMessage()
					if err != nil {
						return
					}
				}
			},
		),
	)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(
		srv.URL, "http",
	)

	adapter := NewGorillaWebSocketAdapter()
	headers := map[string]string{
		"X-Custom-Header": "test-value",
		"Authorization":   "Bearer token123",
	}
	err := adapter.Connect(
		context.Background(), wsURL, headers,
	)
	require.NoError(t, err)
	defer adapter.Close(context.Background())

	assert.Equal(
		t,
		"test-value",
		receivedHeaders.Get("X-Custom-Header"),
	)
	assert.Equal(
		t,
		"Bearer token123",
		receivedHeaders.Get("Authorization"),
	)
}

func TestGorillaWebSocketAdapter_Available_Connected(
	t *testing.T,
) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(_ *http.Request) bool {
			return true
		},
	}
	srv := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				conn, err := upgrader.Upgrade(
					w, r, nil,
				)
				if err != nil {
					return
				}
				defer conn.Close()
				for {
					_, _, err := conn.ReadMessage()
					if err != nil {
						return
					}
				}
			},
		),
	)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(
		srv.URL, "http",
	)

	adapter := NewGorillaWebSocketAdapter()
	err := adapter.Connect(
		context.Background(), wsURL, nil,
	)
	require.NoError(t, err)
	defer adapter.Close(context.Background())

	available := adapter.Available(
		context.Background(),
	)
	assert.True(t, available)
}

func TestGorillaWebSocketAdapter_SendAndReceive_Echo(
	t *testing.T,
) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(_ *http.Request) bool {
			return true
		},
	}
	srv := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				conn, err := upgrader.Upgrade(
					w, r, nil,
				)
				if err != nil {
					return
				}
				defer conn.Close()
				for {
					mt, msg, err := conn.ReadMessage()
					if err != nil {
						return
					}
					// Echo back.
					_ = conn.WriteMessage(mt, msg)
				}
			},
		),
	)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(
		srv.URL, "http",
	)

	adapter := NewGorillaWebSocketAdapter()
	err := adapter.Connect(
		context.Background(), wsURL, nil,
	)
	require.NoError(t, err)
	defer adapter.Close(context.Background())

	resp, err := adapter.SendAndReceive(
		context.Background(),
		[]byte("hello"),
		5*time.Second,
	)
	require.NoError(t, err)
	assert.Equal(t, []byte("hello"), resp)
}

func TestGorillaWebSocketAdapter_Send_And_Receive(
	t *testing.T,
) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(_ *http.Request) bool {
			return true
		},
	}
	srv := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				conn, err := upgrader.Upgrade(
					w, r, nil,
				)
				if err != nil {
					return
				}
				defer conn.Close()
				for {
					mt, msg, err := conn.ReadMessage()
					if err != nil {
						return
					}
					resp := fmt.Sprintf(
						"echo: %s", string(msg),
					)
					_ = conn.WriteMessage(
						mt, []byte(resp),
					)
				}
			},
		),
	)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(
		srv.URL, "http",
	)

	adapter := NewGorillaWebSocketAdapter()
	err := adapter.Connect(
		context.Background(), wsURL, nil,
	)
	require.NoError(t, err)
	defer adapter.Close(context.Background())

	// Send.
	err = adapter.Send(
		context.Background(), []byte("test"),
	)
	require.NoError(t, err)

	// Receive.
	msg, err := adapter.Receive(
		context.Background(), 5*time.Second,
	)
	require.NoError(t, err)
	assert.Equal(t, "echo: test", string(msg))
}

func TestGorillaWebSocketAdapter_Receive_Timeout(
	t *testing.T,
) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(_ *http.Request) bool {
			return true
		},
	}
	srv := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				conn, err := upgrader.Upgrade(
					w, r, nil,
				)
				if err != nil {
					return
				}
				defer conn.Close()
				// Never send anything; wait for close.
				for {
					_, _, err := conn.ReadMessage()
					if err != nil {
						return
					}
				}
			},
		),
	)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(
		srv.URL, "http",
	)

	adapter := NewGorillaWebSocketAdapter()
	err := adapter.Connect(
		context.Background(), wsURL, nil,
	)
	require.NoError(t, err)
	defer adapter.Close(context.Background())

	// Receive with very short timeout should fail.
	_, err = adapter.Receive(
		context.Background(), 50*time.Millisecond,
	)
	assert.Error(t, err)
}

func TestGorillaWebSocketAdapter_ReceiveAll_Timeout(
	t *testing.T,
) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(_ *http.Request) bool {
			return true
		},
	}
	srv := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				conn, err := upgrader.Upgrade(
					w, r, nil,
				)
				if err != nil {
					return
				}
				defer conn.Close()
				// Send 2 messages then stop.
				_ = conn.WriteMessage(
					websocket.TextMessage,
					[]byte("msg1"),
				)
				_ = conn.WriteMessage(
					websocket.TextMessage,
					[]byte("msg2"),
				)
				// Wait for close.
				for {
					_, _, err := conn.ReadMessage()
					if err != nil {
						return
					}
				}
			},
		),
	)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(
		srv.URL, "http",
	)

	adapter := NewGorillaWebSocketAdapter()
	err := adapter.Connect(
		context.Background(), wsURL, nil,
	)
	require.NoError(t, err)
	defer adapter.Close(context.Background())

	// ReceiveAll should collect messages until timeout.
	msgs, err := adapter.ReceiveAll(
		context.Background(), 500*time.Millisecond,
	)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(msgs), 2)
	assert.Equal(t, "msg1", string(msgs[0]))
	assert.Equal(t, "msg2", string(msgs[1]))
}

func TestGorillaWebSocketAdapter_Close_ClearsConn(
	t *testing.T,
) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(_ *http.Request) bool {
			return true
		},
	}
	srv := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				conn, err := upgrader.Upgrade(
					w, r, nil,
				)
				if err != nil {
					return
				}
				defer conn.Close()
				for {
					_, _, err := conn.ReadMessage()
					if err != nil {
						return
					}
				}
			},
		),
	)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(
		srv.URL, "http",
	)

	adapter := NewGorillaWebSocketAdapter()
	err := adapter.Connect(
		context.Background(), wsURL, nil,
	)
	require.NoError(t, err)
	assert.NotNil(t, adapter.conn)

	err = adapter.Close(context.Background())
	assert.NoError(t, err)
	assert.Nil(t, adapter.conn)
}

func TestIsTimeoutError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil_error",
			err:  nil,
			want: false,
		},
		{
			name: "timeout_error",
			err:  fmt.Errorf("i/o timeout"),
			want: true,
		},
		{
			name: "deadline_exceeded",
			err:  fmt.Errorf("deadline exceeded"),
			want: true,
		},
		{
			name: "wrapped_timeout",
			err: fmt.Errorf(
				"read message: i/o timeout",
			),
			want: true,
		},
		{
			name: "not_a_timeout",
			err:  fmt.Errorf("connection refused"),
			want: false,
		},
		{
			name: "case_insensitive_timeout",
			err:  fmt.Errorf("I/O Timeout"),
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isTimeoutError(tt.err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestContains_CaseInsensitive(t *testing.T) {
	tests := []struct {
		name   string
		s      string
		substr string
		want   bool
	}{
		{
			name:   "exact_match",
			s:      "timeout",
			substr: "timeout",
			want:   true,
		},
		{
			name:   "case_mismatch",
			s:      "TIMEOUT",
			substr: "timeout",
			want:   true,
		},
		{
			name:   "not_found",
			s:      "connection",
			substr: "timeout",
			want:   false,
		},
		{
			name:   "empty_string",
			s:      "",
			substr: "timeout",
			want:   false,
		},
		{
			name:   "empty_substr",
			s:      "hello",
			substr: "",
			want:   true,
		},
		{
			name:   "substr_longer",
			s:      "hi",
			substr: "hello world",
			want:   false,
		},
		{
			name:   "partial_match",
			s:      "read: Deadline Exceeded here",
			substr: "deadline exceeded",
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := contains(tt.s, tt.substr)
			assert.Equal(t, tt.want, got)
		})
	}
}
