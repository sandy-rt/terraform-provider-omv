// Package client is a minimal JSON-RPC client for the OpenMediaVault API.
//
// OMV exposes a single endpoint at <endpoint>/rpc.php and expects
// POST bodies of the form {"service","method","params"} with session-cookie
// auth established via Session.login. Config changes are staged and must be
// activated with Config.applyChanges.
package client

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"time"
)

// NewObjectUUID is OMV's sentinel UUID meaning "this is a new object" on create.
const NewObjectUUID = "fa4b1c66-ef79-11e5-87a0-0002b3a176b4"

// UndefinedUUID is OMV's "empty reference" UUID. Some RPCs require a uuidv4
// reference field to be present even when the server computes the real value
// itself (e.g. NFS.setShare's mntentref on create, which OMV overwrites).
const UndefinedUUID = "00000000-0000-0000-0000-000000000000"

type Client struct {
	Endpoint string
	http     *http.Client
	username string
	password string
}

type rpcRequest struct {
	Service string      `json:"service"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcResponse struct {
	Response json.RawMessage `json:"response"`
	Error    *rpcError       `json:"error"`
}

// New builds a client with a cookie jar (for the session) and a lenient TLS
// config so self-signed OMV HTTPS endpoints work.
func New(endpoint string) (*Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	tr := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	return &Client{
		Endpoint: endpoint,
		// Generous timeout: Config.applyChanges regenerates configs and
		// restarts services, which can take well over a minute.
		http: &http.Client{Jar: jar, Transport: tr, Timeout: 300 * time.Second},
	}, nil
}

// Login authenticates and stores the session cookie in the jar. The
// credentials are retained so the client can transparently re-login if the
// session expires during a long-running apply.
func (c *Client) Login(username, password string) error {
	c.username = username
	c.password = password
	_, err := c.Call("Session", "login", map[string]string{
		"username": username,
		"password": password,
	})
	return err
}

// Call performs a single JSON-RPC request and returns the raw response value.
func (c *Client) Call(service, method string, params interface{}) (json.RawMessage, error) {
	body, err := json.Marshal(rpcRequest{Service: service, Method: method, Params: params})
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Post(c.Endpoint+"/rpc.php", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("%s.%s: request failed: %w", service, method, err)
	}
	defer resp.Body.Close()

	var rr rpcResponse
	if err := json.NewDecoder(resp.Body).Decode(&rr); err != nil {
		return nil, fmt.Errorf("%s.%s: decode failed: %w", service, method, err)
	}
	if rr.Error != nil {
		return nil, fmt.Errorf("%s.%s: %s", service, method, rr.Error.Message)
	}
	return rr.Response, nil
}

// ApplyChangesAndWait activates staged configuration changes and blocks until
// the OMV background job finishes (or the timeout elapses).
//
// IMPORTANT OMV behaviour: applying changes can take a very long time (minutes
// to ~1 hour on low-powered hardware), and if the background job is interrupted
// OMV reverts the staged changes. So this must wait for completion and must not
// give up prematurely. The synchronous Config.applyChanges would also exceed
// the nginx proxy timeout, hence the background variant + polling.
//
// force=false: only deploy modules that actually have pending changes, so a
// call with nothing dirty returns quickly instead of redeploying everything.
func (c *Client) ApplyChangesAndWait(timeout, pollInterval time.Duration) error {
	raw, err := c.Call("Config", "applyChangesBg", map[string]interface{}{
		"modules": []string{},
		"force":   false,
	})
	if err != nil {
		return err
	}
	var filename string
	if err := json.Unmarshal(raw, &filename); err != nil || filename == "" {
		// No background job id returned: nothing to wait on.
		return nil
	}

	deadline := time.Now().Add(timeout)
	transientErrors := 0
	for time.Now().Before(deadline) {
		running, err := c.execIsRunning(filename)
		if err != nil {
			msg := err.Error()
			switch {
			case isAuthError(msg):
				// The session expired during a long apply: re-login and keep
				// polling the same background job (which is still running).
				_ = c.Login(c.username, c.password)
				time.Sleep(pollInterval)
				continue
			case isTransient(msg):
				transientErrors++
				if transientErrors >= 12 {
					return fmt.Errorf("applyChanges: repeated status-check failures for job %s: %w", filename, err)
				}
				time.Sleep(pollInterval)
				continue
			default:
				// A failed background job surfaces its error (the apply log) here.
				return fmt.Errorf("OMV apply failed: %s", msg)
			}
		}
		transientErrors = 0
		if !running {
			return nil
		}
		time.Sleep(pollInterval)
	}
	return fmt.Errorf("applyChanges: background job %s did not finish within %s", filename, timeout)
}

func isAuthError(msg string) bool {
	m := strings.ToLower(msg)
	return strings.Contains(m, "session expired") ||
		strings.Contains(m, "not authenticated") ||
		strings.Contains(m, "authentication")
}

func isTransient(msg string) bool {
	m := strings.ToLower(msg)
	return strings.Contains(m, "request failed") || // transport error
		strings.Contains(m, "decode failed") ||
		strings.Contains(m, "timeout") ||
		strings.Contains(m, "eof")
}

// execIsRunning reports whether a background job is still running. OMV has
// returned this as either a bare boolean or {"running": bool}, so handle both.
func (c *Client) execIsRunning(filename string) (bool, error) {
	raw, err := c.Call("Exec", "isRunning", map[string]interface{}{"filename": filename})
	if err != nil {
		return false, err
	}
	var asBool bool
	if json.Unmarshal(raw, &asBool) == nil {
		return asBool, nil
	}
	var wrapped struct {
		Running bool `json:"running"`
	}
	if err := json.Unmarshal(raw, &wrapped); err != nil {
		return false, err
	}
	return wrapped.Running, nil
}
