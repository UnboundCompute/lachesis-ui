// Package mcp is a minimal stdio JSON-RPC client for the lachesis MCP server.
//
// The server (lachesis/nav/mcp_server.py) speaks line-delimited JSON-RPC on
// stdout and writes human logs to stderr. It is single-threaded: one request,
// one response line, matched by id. We spawn it once against a prebuilt graph
// and keep it warm for the session, so every query after the initial load is
// instant.
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Client owns the engine subprocess and serializes requests to it.
type Client struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader

	mu     sync.Mutex // one in-flight request at a time
	nextID int

	logs   *ringBuffer // last N stderr lines, for a debug pane
	Server ServerInfo
}

// ServerInfo is what the engine reports at initialize.
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type rpcResponse struct {
	ID     int             `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *rpcError       `json:"error"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Spawn starts the engine against graphPath and completes the MCP handshake.
// python is the interpreter that can import lachesis (see engine.Discover).
func Spawn(python, graphPath string) (*Client, error) {
	cmd := exec.Command(python, "-m", "lachesis.nav.mcp_server", graphPath)
	configureProcess(cmd)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start engine (%s): %w", python, err)
	}

	c := &Client{
		cmd:    cmd,
		stdin:  stdin,
		stdout: bufio.NewReaderSize(stdout, 1<<20),
		logs:   newRingBuffer(200),
	}
	go c.drainStderr(stderr)

	timeout := startupTimeout()
	ready := make(chan error, 1)
	go func() { ready <- c.initialize() }()
	select {
	case err := <-ready:
		if err == nil {
			return c, nil
		}
		err = withEngineLogs(err, c.Logs())
		_ = c.Close()
		return nil, err
	case <-time.After(timeout):
		diagnostic := withEngineLogs(
			fmt.Errorf("engine initialization timed out after %s", timeout),
			c.Logs(),
		)
		_ = c.Close()
		return nil, diagnostic
	}
}

// withEngineLogs keeps startup failures actionable without putting stderr on the
// MCP stdout channel. In particular, a wrong interpreter/cwd and an unreadable
// graph otherwise look identical to a client as a silent process exit.
func withEngineLogs(err error, logs []string) error {
	if len(logs) == 0 {
		return err
	}
	const maxLines = 20
	if len(logs) > maxLines {
		logs = logs[len(logs)-maxLines:]
	}
	return fmt.Errorf("%w\nengine stderr:\n%s", err, strings.Join(logs, "\n"))
}

const defaultStartupTimeout = 5 * time.Minute

func startupTimeout() time.Duration {
	value := os.Getenv("LACHESIS_UI_STARTUP_TIMEOUT")
	if value == "" {
		return defaultStartupTimeout
	}
	timeout, err := time.ParseDuration(value)
	if err != nil || timeout <= 0 {
		return defaultStartupTimeout
	}
	return timeout
}

func (c *Client) initialize() error {
	raw, err := c.request("initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"clientInfo":      map[string]any{"name": "lachesis-ui", "version": Version},
		"capabilities":    map[string]any{},
	})
	if err != nil {
		return fmt.Errorf("initialize: %w", err)
	}
	var res struct {
		ServerInfo ServerInfo `json:"serverInfo"`
	}
	_ = json.Unmarshal(raw, &res)
	c.Server = res.ServerInfo

	// Best-effort per the spec; the server ignores the notification's absence.
	_ = c.notify("notifications/initialized", map[string]any{})
	return nil
}

// Call invokes a tool and returns the decoded JSON payload the tool emitted.
//
// The server wraps tool output as {result:{content:[{type:"text",text:...}]}}.
// We always request format:"json", so that text is itself a JSON document —
// this returns it as RawMessage for the typed wrappers in tools.go to decode.
func (c *Client) Call(tool string, args map[string]any) (json.RawMessage, error) {
	if args == nil {
		args = map[string]any{}
	}
	if _, ok := args["format"]; !ok {
		args["format"] = "json"
	}
	raw, err := c.requestBounded("tools/call", map[string]any{
		"name":      tool,
		"arguments": args,
	}, tool)
	if err != nil {
		return nil, err
	}

	var res struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		return nil, fmt.Errorf("%s: decode envelope: %w", tool, err)
	}
	if len(res.Content) == 0 {
		return nil, fmt.Errorf("%s: empty response", tool)
	}
	text := res.Content[0].Text
	if res.IsError {
		return nil, fmt.Errorf("%s: %s", tool, strings.TrimPrefix(text, "error: "))
	}
	return json.RawMessage(text), nil
}

const defaultRequestTimeout = 2 * time.Minute

func requestTimeout() time.Duration {
	value := os.Getenv("LACHESIS_UI_REQUEST_TIMEOUT")
	if value == "" {
		return defaultRequestTimeout
	}
	timeout, err := time.ParseDuration(value)
	if err != nil || timeout <= 0 {
		return defaultRequestTimeout
	}
	return timeout
}

// requestBounded prevents a stalled engine query from leaving the terminal UI
// permanently waiting. Once a request exceeds the bound, the engine is closed so
// queued calls cannot continue against an unresponsive process; the stderr tail is
// retained in the returned error for an actionable diagnosis.
func (c *Client) requestBounded(method string, params any, label string) (json.RawMessage, error) {
	result := make(chan struct {
		raw json.RawMessage
		err error
	}, 1)
	go func() {
		raw, err := c.request(method, params)
		result <- struct {
			raw json.RawMessage
			err error
		}{raw: raw, err: err}
	}()
	timeout := requestTimeout()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case r := <-result:
		return r.raw, r.err
	case <-timer.C:
		err := withEngineLogs(fmt.Errorf("%s timed out after %s", label, timeout), c.Logs())
		_ = c.Close()
		return nil, err
	}
}

// request performs one synchronous JSON-RPC round trip.
func (c *Client) request(method string, params any) (json.RawMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.nextID++
	id := c.nextID
	payload, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  params,
	})
	if err != nil {
		return nil, err
	}
	if _, err := c.stdin.Write(append(payload, '\n')); err != nil {
		return nil, fmt.Errorf("write %s: %w", method, err)
	}

	// Read lines until the response whose id matches ours. The server is
	// single-threaded so interleaving cannot happen, but we stay defensive.
	for {
		line, err := c.stdout.ReadBytes('\n')
		if err != nil {
			if len(line) == 0 {
				return nil, withEngineLogs(fmt.Errorf("read %s: %w", method, err), c.Logs())
			}
		}
		line = trimLine(line)
		if len(line) == 0 {
			continue
		}
		var resp rpcResponse
		if err := json.Unmarshal(line, &resp); err != nil {
			continue // a stray non-JSON line; skip
		}
		if resp.ID != id {
			continue
		}
		if resp.Error != nil {
			return nil, fmt.Errorf("%s: rpc error %d: %s", method, resp.Error.Code, resp.Error.Message)
		}
		return resp.Result, nil
	}
}

func (c *Client) notify(method string, params any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	payload, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
	})
	if err != nil {
		return err
	}
	_, err = c.stdin.Write(append(payload, '\n'))
	return err
}

func (c *Client) drainStderr(r io.Reader) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		c.logs.push(sc.Text())
	}
}

// Logs returns a snapshot of recent engine stderr lines.
func (c *Client) Logs() []string { return c.logs.snapshot() }

// Close shuts down the engine, giving it a moment to exit cleanly.
func (c *Client) Close() error {
	if c.stdin != nil {
		_ = c.stdin.Close()
	}
	if c.cmd == nil || c.cmd.Process == nil {
		return nil
	}
	done := make(chan error, 1)
	go func() { done <- c.cmd.Wait() }()
	select {
	case err := <-done:
		return err
	case <-time.After(2 * time.Second):
		_ = killProcessTree(c.cmd)
		return nil
	}
}

// WaitReady issues a ping so the caller can surface load progress; the first
// call returns only once the graph has finished loading in the engine.
func (c *Client) WaitReady(ctx context.Context) error {
	type result struct{ err error }
	ch := make(chan result, 1)
	go func() {
		_, err := c.request("ping", map[string]any{})
		ch <- result{err}
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case r := <-ch:
		return r.err
	}
}

func trimLine(b []byte) []byte {
	for len(b) > 0 && (b[len(b)-1] == '\n' || b[len(b)-1] == '\r' || b[len(b)-1] == ' ') {
		b = b[:len(b)-1]
	}
	return b
}

// ringBuffer is a tiny fixed-size log tail, safe for concurrent push/snapshot.
type ringBuffer struct {
	mu   sync.Mutex
	buf  []string
	max  int
	next int
	full bool
}

func newRingBuffer(max int) *ringBuffer { return &ringBuffer{buf: make([]string, max), max: max} }

func (r *ringBuffer) push(s string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.buf[r.next] = s
	r.next = (r.next + 1) % r.max
	if r.next == 0 {
		r.full = true
	}
}

func (r *ringBuffer) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.full {
		out := make([]string, r.next)
		copy(out, r.buf[:r.next])
		return out
	}
	out := make([]string, 0, r.max)
	out = append(out, r.buf[r.next:]...)
	out = append(out, r.buf[:r.next]...)
	return out
}
