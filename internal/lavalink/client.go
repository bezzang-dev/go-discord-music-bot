package lavalink

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
)

const clientName = "hnmo-discord-bot/0.1.0"

type Client struct {
	baseURL    string
	password   string
	httpClient *http.Client

	mu           sync.RWMutex
	sessionID    string
	wsConn       *websocket.Conn
	eventHandler func(Event)
}

type Track struct {
	Encoded    string                 `json:"encoded"`
	Info       TrackInfo              `json:"info"`
	PluginInfo map[string]interface{} `json:"pluginInfo"`
	UserData   map[string]interface{} `json:"userData"`
}

type TrackInfo struct {
	Identifier string `json:"identifier"`
	Author     string `json:"author"`
	Length     int64  `json:"length"`
	Title      string `json:"title"`
	URI        string `json:"uri"`
	ArtworkURL string `json:"artworkUrl"`
	SourceName string `json:"sourceName"`
}

type VoiceState struct {
	Token     string `json:"token"`
	Endpoint  string `json:"endpoint"`
	SessionID string `json:"sessionId"`
	ChannelID string `json:"channelId"`
}

type loadResult struct {
	LoadType string          `json:"loadType"`
	Data     json.RawMessage `json:"data"`
}

type trackListData struct {
	Info struct {
		Name          string `json:"name"`
		SelectedTrack int    `json:"selectedTrack"`
	} `json:"info"`
	Tracks []Track `json:"tracks"`
}

type loadErrorData struct {
	Message string `json:"message"`
}

type readyPayload struct {
	Op        string `json:"op"`
	Resumed   bool   `json:"resumed"`
	SessionID string `json:"sessionId"`
}

type opPayload struct {
	Op string `json:"op"`
}

type Event struct {
	Op      string `json:"op"`
	Type    string `json:"type"`
	GuildID string `json:"guildId"`
	Track   Track  `json:"track"`
	Reason  string `json:"reason"`
}

type playerUpdateRequest struct {
	Track *struct {
		Encoded string `json:"encoded,omitempty"`
	} `json:"track,omitempty"`
	Voice *VoiceState `json:"voice,omitempty"`
}

func NewClient(baseURL, password string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		password:   password,
		httpClient: httpClient,
	}
}

func (c *Client) Version(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/version", nil)
	if err != nil {
		return "", fmt.Errorf("build lavalink version request: %w", err)
	}

	req.Header.Set("Authorization", c.password)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request lavalink version: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return "", fmt.Errorf("lavalink rejected credentials for %s/version: check LAVALINK_PASSWORD", c.baseURL)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 2048))
		if readErr != nil {
			return "", fmt.Errorf("lavalink version request failed with status %d", resp.StatusCode)
		}
		return "", fmt.Errorf("lavalink version request failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2048))
	if err != nil {
		return "", fmt.Errorf("read lavalink version response: %w", err)
	}

	version := strings.TrimSpace(string(body))
	if version == "" {
		return "", fmt.Errorf("lavalink returned an empty version response")
	}

	return version, nil
}

func (c *Client) Connect(ctx context.Context, userID string) error {
	wsURL, err := websocketURL(c.baseURL)
	if err != nil {
		return err
	}

	headers := http.Header{}
	headers.Set("Authorization", c.password)
	headers.Set("User-Id", userID)
	headers.Set("Client-Name", clientName)

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL+"/v4/websocket", headers)
	if err != nil {
		return fmt.Errorf("connect lavalink websocket: %w", err)
	}

	readyCh := make(chan error, 1)
	go c.readLoop(conn, readyCh)

	select {
	case err := <-readyCh:
		if err != nil {
			_ = conn.Close()
			return err
		}
		c.mu.Lock()
		c.wsConn = conn
		c.mu.Unlock()
		return nil
	case <-ctx.Done():
		_ = conn.Close()
		return fmt.Errorf("wait for lavalink ready: %w", ctx.Err())
	}
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.wsConn == nil {
		return nil
	}

	err := c.wsConn.Close()
	c.wsConn = nil
	c.sessionID = ""
	return err
}

func (c *Client) SetEventHandler(handler func(Event)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.eventHandler = handler
}

func (c *Client) SessionID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.sessionID
}

// LoadTrack normalizes Lavalink's different load result types into a single track the bot can enqueue.
func (c *Client) LoadTrack(ctx context.Context, identifier string) (Track, error) {
	endpoint := c.baseURL + "/v4/loadtracks?identifier=" + url.QueryEscape(identifier)

	var result loadResult
	if err := c.doJSON(ctx, http.MethodGet, endpoint, nil, &result); err != nil {
		return Track{}, err
	}

	// Lavalink returns different payload shapes for direct tracks, searches, and playlists.
	switch result.LoadType {
	case "track":
		var track Track
		if err := json.Unmarshal(result.Data, &track); err != nil {
			return Track{}, fmt.Errorf("decode lavalink track result: %w", err)
		}
		return track, nil
	case "search":
		var tracks []Track
		if err := json.Unmarshal(result.Data, &tracks); err != nil {
			return Track{}, fmt.Errorf("decode lavalink search result: %w", err)
		}
		if len(tracks) == 0 {
			return Track{}, fmt.Errorf("lavalink returned an empty search result")
		}
		return tracks[0], nil
	case "playlist":
		var playlist trackListData
		if err := json.Unmarshal(result.Data, &playlist); err != nil {
			return Track{}, fmt.Errorf("decode lavalink playlist result: %w", err)
		}
		if len(playlist.Tracks) == 0 {
			return Track{}, fmt.Errorf("lavalink returned an empty playlist")
		}
		if playlist.Info.SelectedTrack >= 0 && playlist.Info.SelectedTrack < len(playlist.Tracks) {
			return playlist.Tracks[playlist.Info.SelectedTrack], nil
		}
		return playlist.Tracks[0], nil
	case "empty":
		return Track{}, fmt.Errorf("no matches found for %q", identifier)
	case "error":
		var loadErr loadErrorData
		if err := json.Unmarshal(result.Data, &loadErr); err != nil {
			return Track{}, fmt.Errorf("lavalink returned a track load error")
		}
		return Track{}, fmt.Errorf("lavalink track load failed: %s", loadErr.Message)
	default:
		return Track{}, fmt.Errorf("unsupported lavalink load type %q", result.LoadType)
	}
}

// UpdateVoiceState forwards Discord voice session details so Lavalink can attach its player to the guild voice connection.
func (c *Client) UpdateVoiceState(ctx context.Context, guildID string, voice VoiceState) error {
	request := playerUpdateRequest{
		Voice: &voice,
	}

	return c.updatePlayer(ctx, guildID, request)
}

// PlayTrack swaps the currently active Lavalink track for the guild.
func (c *Client) PlayTrack(ctx context.Context, guildID string, track Track) error {
	request := playerUpdateRequest{
		Track: &struct {
			Encoded string `json:"encoded,omitempty"`
		}{
			Encoded: track.Encoded,
		},
	}

	return c.updatePlayer(ctx, guildID, request)
}

// StopTrack clears the active Lavalink track without destroying the player session.
func (c *Client) StopTrack(ctx context.Context, guildID string) error {
	request := map[string]interface{}{
		"track": map[string]interface{}{
			// Lavalink clears the active track when encoded is explicitly set to null.
			"encoded": nil,
		},
	}

	return c.updatePlayerRaw(ctx, guildID, request)
}

func (c *Client) DestroyPlayer(ctx context.Context, guildID string) error {
	sessionID := c.SessionID()
	if sessionID == "" {
		return fmt.Errorf("lavalink session is not ready")
	}

	endpoint := fmt.Sprintf("%s/v4/sessions/%s/players/%s", c.baseURL, sessionID, guildID)
	if err := c.doJSON(ctx, http.MethodDelete, endpoint, nil, nil); err != nil {
		return fmt.Errorf("destroy lavalink player for guild %s: %w", guildID, err)
	}

	return nil
}

func (c *Client) updatePlayer(ctx context.Context, guildID string, request playerUpdateRequest) error {
	return c.updatePlayerRaw(ctx, guildID, request)
}

func (c *Client) updatePlayerRaw(ctx context.Context, guildID string, request interface{}) error {
	sessionID := c.SessionID()
	if sessionID == "" {
		return fmt.Errorf("lavalink session is not ready")
	}

	endpoint := fmt.Sprintf("%s/v4/sessions/%s/players/%s", c.baseURL, sessionID, guildID)
	if err := c.doJSON(ctx, http.MethodPatch, endpoint, request, nil); err != nil {
		return fmt.Errorf("update lavalink player for guild %s: %w", guildID, err)
	}

	return nil
}

func (c *Client) doJSON(ctx context.Context, method, endpoint string, body interface{}, out interface{}) error {
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal lavalink request: %w", err)
		}
		reader = strings.NewReader(string(payload))
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return fmt.Errorf("build lavalink request: %w", err)
	}

	req.Header.Set("Authorization", c.password)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request lavalink %s %s: %w", method, endpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("lavalink rejected credentials for %s: check LAVALINK_PASSWORD", endpoint)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 4096))
		if readErr != nil {
			return fmt.Errorf("lavalink request failed with status %d", resp.StatusCode)
		}
		return fmt.Errorf("lavalink request failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	if out == nil {
		return nil
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode lavalink response: %w", err)
	}

	return nil
}

// readLoop waits for Lavalink readiness once and then routes later websocket messages to the event handler.
func (c *Client) readLoop(conn *websocket.Conn, readyCh chan<- error) {
	readySent := false

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if !readySent {
				readyCh <- fmt.Errorf("read lavalink websocket ready payload: %w", err)
			}
			return
		}

		var op opPayload
		if err := json.Unmarshal(message, &op); err != nil {
			if !readySent {
				readyCh <- fmt.Errorf("decode lavalink websocket payload: %w", err)
			}
			return
		}

		// The websocket is usable only after Lavalink sends its ready frame with a session ID.
		if op.Op != "ready" {
			if readySent {
				c.handleEvent(message)
			}
			continue
		}

		var ready readyPayload
		if err := json.Unmarshal(message, &ready); err != nil {
			if !readySent {
				readyCh <- fmt.Errorf("decode lavalink ready payload: %w", err)
			}
			return
		}

		c.mu.Lock()
		c.sessionID = ready.SessionID
		c.mu.Unlock()

		if !readySent {
			readyCh <- nil
			readySent = true
		}
	}
}

func (c *Client) handleEvent(message []byte) {
	c.mu.RLock()
	handler := c.eventHandler
	c.mu.RUnlock()

	if handler == nil {
		return
	}

	var event Event
	if err := json.Unmarshal(message, &event); err != nil {
		return
	}

	handler(event)
}

func websocketURL(baseURL string) (string, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("parse lavalink base url %q: %w", baseURL, err)
	}

	switch parsed.Scheme {
	case "http":
		parsed.Scheme = "ws"
	case "https":
		parsed.Scheme = "wss"
	default:
		return "", fmt.Errorf("unsupported lavalink scheme %q", parsed.Scheme)
	}

	return strings.TrimRight(parsed.String(), "/"), nil
}
