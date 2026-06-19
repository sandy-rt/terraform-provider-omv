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
	"time"
)

// NewObjectUUID is OMV's sentinel UUID meaning "this is a new object" on create.
const NewObjectUUID = "fa4b1c66-ef79-11e5-87a0-0002b3a176b4"

type Client struct {
	Endpoint string
	http     *http.Client
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

// Login authenticates and stores the session cookie in the jar.
func (c *Client) Login(username, password string) error {
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

// ApplyChanges activates all staged configuration changes.
//
// The synchronous Config.applyChanges call can exceed the OMV nginx proxy
// timeout (it regenerates configs and restarts services). So we use the
// background variant the web UI uses — Config.applyChangesBg returns a status
// file we poll via Exec.isRunning until the job finishes.
func (c *Client) ApplyChanges() error {
	raw, err := c.Call("Config", "applyChangesBg", map[string]interface{}{
		"modules": []string{},
		"force":   true,
	})
	if err != nil {
		return err
	}
	var filename string
	if err := json.Unmarshal(raw, &filename); err != nil || filename == "" {
		// No background job id returned: nothing to wait on.
		return nil
	}

	deadline := time.Now().Add(5 * time.Minute)
	for time.Now().Before(deadline) {
		running, err := c.execIsRunning(filename)
		if err != nil {
			// Transient error while the box is busy applying; back off and retry.
			time.Sleep(3 * time.Second)
			continue
		}
		if !running {
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("applyChanges: background job %s did not finish within timeout", filename)
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
