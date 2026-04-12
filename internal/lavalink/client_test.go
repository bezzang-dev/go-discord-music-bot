package lavalink

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestVersionSuccess(t *testing.T) {
	httpClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if got := r.Header.Get("Authorization"); got != "password" {
				t.Fatalf("unexpected authorization header: %q", got)
			}
			if r.URL.String() != "http://lavalink.local/version" {
				t.Fatalf("unexpected url: %s", r.URL.String())
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("4.2.2")),
				Header:     make(http.Header),
			}, nil
		}),
	}

	client := NewClient("http://lavalink.local", "password", httpClient)

	version, err := client.Version(context.Background())
	if err != nil {
		t.Fatalf("Version returned error: %v", err)
	}
	if version != "4.2.2" {
		t.Fatalf("unexpected version %q", version)
	}
}

func TestLoadTrackSelectsFirstSearchResult(t *testing.T) {
	payload := loadResult{
		LoadType: "search",
		Data: mustJSON(t, []Track{
			{
				Encoded: "encoded-track",
				Info: TrackInfo{
					Title:  "Song Title",
					Author: "Artist",
				},
			},
		}),
	}

	httpClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if !strings.Contains(r.URL.String(), "/v4/loadtracks?identifier=") {
				t.Fatalf("unexpected url: %s", r.URL.String())
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(string(mustJSON(t, payload)))),
				Header:     make(http.Header),
			}, nil
		}),
	}

	client := NewClient("http://lavalink.local", "password", httpClient)

	track, err := client.LoadTrack(context.Background(), "ytsearch:test")
	if err != nil {
		t.Fatalf("LoadTrack returned error: %v", err)
	}
	if track.Encoded != "encoded-track" {
		t.Fatalf("unexpected encoded track %q", track.Encoded)
	}
}

func TestLoadTrackReturnsEmptyError(t *testing.T) {
	payload := loadResult{
		LoadType: "empty",
		Data:     mustJSON(t, map[string]interface{}{}),
	}

	httpClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(string(mustJSON(t, payload)))),
				Header:     make(http.Header),
			}, nil
		}),
	}

	client := NewClient("http://lavalink.local", "password", httpClient)

	_, err := client.LoadTrack(context.Background(), "ytsearch:none")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "no matches found") {
		t.Fatalf("unexpected error %q", err.Error())
	}
}

func mustJSON(t *testing.T, value interface{}) json.RawMessage {
	t.Helper()

	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal JSON: %v", err)
	}

	return data
}

func TestVersionUnauthorized(t *testing.T) {
	httpClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusUnauthorized,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     make(http.Header),
			}, nil
		}),
	}

	client := NewClient("http://lavalink.local", "wrong", httpClient)

	_, err := client.Version(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "LAVALINK_PASSWORD") {
		t.Fatalf("expected password hint, got %q", err.Error())
	}
}

func TestVersionEmptyBody(t *testing.T) {
	httpClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     make(http.Header),
			}, nil
		}),
	}

	client := NewClient("http://lavalink.local", "password", httpClient)

	_, err := client.Version(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "empty version response") {
		t.Fatalf("unexpected error %q", err.Error())
	}
}

func TestStatsSuccess(t *testing.T) {
	httpClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.String() != "http://lavalink.local/v4/stats" {
				t.Fatalf("unexpected url: %s", r.URL.String())
			}
			payload := `{
				"players": 2,
				"playingPlayers": 1,
				"uptime": 123000,
				"memory": {
					"free": 1,
					"used": 2,
					"allocated": 3,
					"reservable": 4
				},
				"cpu": {
					"cores": 8,
					"systemLoad": 0.5,
					"lavalinkLoad": 0.25
				}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(payload)),
				Header:     make(http.Header),
			}, nil
		}),
	}

	client := NewClient("http://lavalink.local", "password", httpClient)

	stats, err := client.Stats(context.Background())
	if err != nil {
		t.Fatalf("Stats returned error: %v", err)
	}
	if stats.Players != 2 || stats.PlayingPlayers != 1 || stats.Uptime != 123000 {
		t.Fatalf("unexpected player stats: %+v", stats)
	}
	if stats.Memory.Used != 2 || stats.Memory.Reservable != 4 {
		t.Fatalf("unexpected memory stats: %+v", stats.Memory)
	}
	if stats.CPU.Cores != 8 || stats.CPU.SystemLoad != 0.5 || stats.CPU.LavalinkLoad != 0.25 {
		t.Fatalf("unexpected cpu stats: %+v", stats.CPU)
	}
}

func TestStatsUnauthorized(t *testing.T) {
	httpClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusUnauthorized,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     make(http.Header),
			}, nil
		}),
	}

	client := NewClient("http://lavalink.local", "wrong", httpClient)

	_, err := client.Stats(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "LAVALINK_PASSWORD") {
		t.Fatalf("expected password hint, got %q", err.Error())
	}
}

func TestClearConnectionMarksClientDisconnected(t *testing.T) {
	client := NewClient("http://lavalink.local", "password", nil)
	conn := &websocket.Conn{}
	client.mu.Lock()
	client.wsConn = conn
	client.sessionID = "session"
	client.mu.Unlock()

	if !client.Connected() {
		t.Fatal("expected connected client")
	}

	client.clearConnection(conn)

	if client.Connected() {
		t.Fatal("expected disconnected client")
	}
	if client.SessionID() != "" {
		t.Fatalf("expected empty session ID, got %q", client.SessionID())
	}
}
