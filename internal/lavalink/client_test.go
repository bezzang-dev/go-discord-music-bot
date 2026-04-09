package lavalink

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
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
