package screenshot

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
)

var devtoolsURLRe = regexp.MustCompile(`ws://[^\s]+`)

type viewport struct {
	Width  int
	Height int
	Mobile bool
}

var (
	desktopViewport = viewport{Width: 1280, Height: 800, Mobile: false}
	mobileViewport  = viewport{Width: 390, Height: 844, Mobile: true}
)

type cdpMessage struct {
	ID     int             `json:"id"`
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *cdpError       `json:"error,omitempty"`
}

type cdpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type cdpClient struct {
	conn   *websocket.Conn
	nextID int
}

func capturePage(ctx context.Context, chromePath, pageURL, userDataDir string, vp viewport) ([]byte, error) {
	debugPort, err := freePort()
	if err != nil {
		return nil, err
	}

	args := []string{
		"--headless=new",
		"--disable-gpu",
		"--no-sandbox",
		"--hide-scrollbars",
		"--disable-dev-shm-usage",
		"--allow-file-access-from-files",
		"--remote-debugging-port=" + strconv.Itoa(debugPort),
		"--user-data-dir=" + userDataDir,
	}

	cmd := exec.CommandContext(ctx, chromePath, args...)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start chrome: %w", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	if _, err := waitForDevTools(ctx, stderr, debugPort); err != nil {
		return nil, err
	}

	wsURL, err := createPageWS(ctx, debugPort, "about:blank")
	if err != nil {
		return nil, err
	}

	client, err := connectCDP(wsURL)
	if err != nil {
		return nil, err
	}
	defer client.close()

	if err := client.call(ctx, "Page.enable", nil); err != nil {
		return nil, err
	}
	if err := client.call(ctx, "Emulation.setDeviceMetricsOverride", map[string]any{
		"width":             vp.Width,
		"height":            vp.Height,
		"deviceScaleFactor": 1,
		"mobile":            vp.Mobile,
	}); err != nil {
		return nil, err
	}
	if err := client.navigateAndWait(ctx, pageURL); err != nil {
		return nil, err
	}

	time.Sleep(500 * time.Millisecond)

	result, err := client.callResult(ctx, "Page.captureScreenshot", map[string]any{
		"format": "png",
	})
	if err != nil {
		return nil, err
	}

	var shot struct {
		Data string `json:"data"`
	}
	if err := json.Unmarshal(result, &shot); err != nil {
		return nil, err
	}
	return base64.StdEncoding.DecodeString(shot.Data)
}

func freePort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return port, nil
}

func waitForDevTools(ctx context.Context, stderr io.Reader, debugPort int) (string, error) {
	urlCh := make(chan string, 1)

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			if m := devtoolsURLRe.FindString(line); m != "" {
				urlCh <- m
				return
			}
		}
	}()

	deadline := time.After(15 * time.Second)
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case url := <-urlCh:
			return url, nil
		case <-deadline:
			return fetchBrowserWS(ctx, debugPort)
		case <-ticker.C:
			if url, err := fetchBrowserWS(ctx, debugPort); err == nil {
				return url, nil
			}
		}
	}
}

func fetchBrowserWS(ctx context.Context, port int) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/json/version", port), nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("devtools version HTTP %d", resp.StatusCode)
	}
	var payload struct {
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	if payload.WebSocketDebuggerURL == "" {
		return "", fmt.Errorf("empty webSocketDebuggerUrl")
	}
	return payload.WebSocketDebuggerURL, nil
}

func createPageWS(ctx context.Context, port int, pageURL string) (string, error) {
	u := fmt.Sprintf("http://127.0.0.1:%d/json/new?%s", port, url.QueryEscape(pageURL))
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, u, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("create page HTTP %d", resp.StatusCode)
	}
	var target struct {
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&target); err != nil {
		return "", err
	}
	if target.WebSocketDebuggerURL == "" {
		return "", fmt.Errorf("empty page webSocketDebuggerUrl")
	}
	return target.WebSocketDebuggerURL, nil
}

func connectCDP(wsURL string) (*cdpClient, error) {
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("cdp connect: %w", err)
	}
	return &cdpClient{conn: conn}, nil
}

func (c *cdpClient) close() {
	if c.conn != nil {
		_ = c.conn.Close()
	}
}

func (c *cdpClient) callResult(ctx context.Context, method string, params any) (json.RawMessage, error) {
	c.nextID++
	id := c.nextID
	msg := cdpMessage{ID: id, Method: method}
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			return nil, err
		}
		msg.Params = b
	}
	if err := c.conn.WriteJSON(msg); err != nil {
		return nil, err
	}

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		_ = c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		var resp cdpMessage
		if err := c.conn.ReadJSON(&resp); err != nil {
			return nil, err
		}
		if resp.ID != id {
			continue
		}
		if resp.Error != nil {
			return nil, fmt.Errorf("cdp %s: %s", method, resp.Error.Message)
		}
		return resp.Result, nil
	}
}

func (c *cdpClient) call(ctx context.Context, method string, params any) error {
	_, err := c.callResult(ctx, method, params)
	return err
}

func (c *cdpClient) navigateAndWait(ctx context.Context, url string) error {
	c.nextID++
	navID := c.nextID
	navMsg := cdpMessage{
		ID:     navID,
		Method: "Page.navigate",
		Params: mustJSON(map[string]any{"url": url}),
	}
	if err := c.conn.WriteJSON(navMsg); err != nil {
		return err
	}

	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		_ = c.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		var evt map[string]any
		if err := c.conn.ReadJSON(&evt); err != nil {
			continue
		}
		if evt["method"] == "Page.loadEventFired" {
			return nil
		}
		if respID, ok := evt["id"].(float64); ok && int(respID) == navID {
			if errObj, ok := evt["error"].(map[string]any); ok {
				return fmt.Errorf("navigate: %v", errObj["message"])
			}
		}
	}
	return fmt.Errorf("page load timeout")
}

func mustJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func userDataDir(dataDir string) (string, error) {
	dir := filepath.Join(dataDir, "chrome-profile")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func fileURL(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	u := &url.URL{
		Scheme: "file",
		Path:   filepath.ToSlash(abs),
	}
	return u.String(), nil
}

func decodePNG(data []byte) (image.Image, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	return img, err
}
