//
// Copyright (c) 2026 Red Hat, Inc.
// Licensed under the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	ToolCreateWorkspace    = "create_workspace"
	ToolDeleteWorkspace    = "delete_workspace"
	ToolGetWorkspaceStatus = "get_workspace_status"
	ToolExecInWorkspace    = "exec_in_workspace"
	ToolLaunchCodingAgent  = "launch_coding_agent"
	ToolGetAgentStatus     = "get_agent_status"
	ToolGetAgentOutput     = "get_agent_output"
	ToolReadTerminalOutput = "read_terminal_output"

	AgentClaude = "claude-code"

	ClaudeStatusRunning  ClaudeStatus = "Running"
	ClaudeStatusFinished ClaudeStatus = "Finished"
	ClaudeStatusLost     ClaudeStatus = "Lost"
	ClaudeStatusIdle     ClaudeStatus = "Idle"
	ClaudeStatusUnknown  ClaudeStatus = "Unknown"
)

type ClaudeStatus string

type Client struct {
	serverUrl  string
	mu         sync.Mutex
	sessionId  string
	httpClient *http.Client
	currentId  atomic.Int32
}

func New(serverUrl string) *Client {
	client := &Client{
		serverUrl: serverUrl,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		currentId: atomic.Int32{},
	}
	client.currentId.Store(1)

	return client
}

type JsonRpcRequest struct {
	JsonRpc string      `json:"jsonrpc"`
	Id      *int        `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type JsonRpcResponse struct {
	JsonRpc string          `json:"jsonrpc"`
	Id      *int            `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JsonRpcError   `json:"error,omitempty"`
}

type JsonRpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type JsonRpcToolResult struct {
	IsError bool `json:"isError"`
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
}

type WorkspaceStatus struct {
	Phase string `json:"phase"`
}

type TaskStatus struct {
	Workspace string `json:"workspace"`
	Phase     string `json:"phase"`
}

type TaskOutput struct {
	Output        string `json:"output"`
	LinesReturned int    `json:"lines_returned"`
}

type ExecResult struct {
	Output      string `json:"output"`
	SessionName string `json:"session_name"`
	Note        string `json:"note"`
}

type TerminalOutput struct {
	Output        string `json:"output"`
	LinesReturned int    `json:"lines_returned"`
}

type toolCallParams struct {
	Name      string      `json:"name"`
	Arguments interface{} `json:"arguments"`
}

func (c *Client) ensureInitialized(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.sessionId != "" {
		return nil
	}

	initializeId := 1

	req := JsonRpcRequest{
		JsonRpc: "2.0",
		Id:      &initializeId,
		Method:  "initialize",
		Params: map[string]interface{}{
			"protocolVersion": "2025-03-26",
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]string{
				"name":    "che-ai-assistant",
				"version": "1.0.0",
			},
		},
	}

	httpResponse, rpcResponse, err := c.doPost(ctx, req, "")
	if err != nil {
		return fmt.Errorf("failed to initialize MCP client: %w", err)
	}
	if rpcResponse == nil {
		return fmt.Errorf("failed to initialize MCP client: response is nil")
	}
	if httpResponse.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to initialize MCP client: HTTP %d", httpResponse.StatusCode)
	}
	if rpcResponse.Error != nil {
		return fmt.Errorf("failed to initialize MCP client: error %v", rpcResponse.Error)
	}

	c.sessionId = httpResponse.Header.Get("Mcp-Session-Id")
	if c.sessionId == "" {
		return fmt.Errorf("failed to initialize MCP client: no session id found")
	}

	notification := JsonRpcRequest{
		JsonRpc: "2.0",
		Method:  "notifications/initialized",
	}

	httpResponse, _, err = c.doPost(ctx, notification, c.sessionId)
	if err != nil {
		return fmt.Errorf("failed to initialize MCP client: %w", err)
	}
	if httpResponse.StatusCode != http.StatusAccepted {
		return fmt.Errorf("failed to initialize MCP client: HTTP %d", httpResponse.StatusCode)
	}

	return nil
}

func (c *Client) CallTool(
	ctx context.Context,
	tool string,
	arguments interface{},
) (string, error) {
	if err := c.ensureInitialized(ctx); err != nil {
		return "", err
	}

	result, err := c.callToolOnce(ctx, tool, arguments)
	if err != nil && c.isSessionNotFound(err) {
		c.resetSession()
		if err := c.ensureInitialized(ctx); err != nil {
			return "", err
		}
		return c.callToolOnce(ctx, tool, arguments)
	}
	return result, err
}

func (c *Client) isSessionNotFound(err error) bool {
	return strings.Contains(err.Error(), "Session not found")
}

func (c *Client) resetSession() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sessionId = ""
}

func (c *Client) callToolOnce(
	ctx context.Context,
	tool string,
	arguments interface{},
) (string, error) {
	currentId := int(c.currentId.Add(1))

	rpcRequest := JsonRpcRequest{
		JsonRpc: "2.0",
		Id:      &currentId,
		Method:  "tools/call",
		Params: toolCallParams{
			Name:      tool,
			Arguments: arguments,
		},
	}

	c.mu.Lock()
	sid := c.sessionId
	c.mu.Unlock()

	httpResponse, rpcResponse, err := c.doPost(ctx, rpcRequest, sid)
	if err != nil {
		return "", fmt.Errorf("failed to call MCP tools %s: %w", tool, err)
	}
	if rpcResponse == nil {
		return "", fmt.Errorf("failed to call MCP tools %s: response is nil", tool)
	}
	if rpcResponse.Error != nil {
		return "", fmt.Errorf("failed to call MCP tools %s: response error: %v", tool, rpcResponse.Error)
	}
	if httpResponse.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to call MCP tools, HTTP %d", httpResponse.StatusCode)
	}

	var rpcToolResult JsonRpcToolResult
	if err := json.Unmarshal(rpcResponse.Result, &rpcToolResult); err != nil {
		return "", fmt.Errorf("failed to call MCP tools %s: response json unmarshal error: %w", tool, err)
	}

	output := ""
	for _, content := range rpcToolResult.Content {
		output += content.Text
	}

	if rpcToolResult.IsError {
		return "", fmt.Errorf("failed to call MCP tools %s: response error: %s", tool, output)
	}

	return output, nil
}

func (c *Client) doPost(
	ctx context.Context,
	body interface{},
	sessionID string,
) (*http.Response, *JsonRpcResponse, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, nil, err
	}

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, c.serverUrl, bytes.NewReader(data))
	if err != nil {
		return nil, nil, err
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Accept", "application/json, text/event-stream")
	if sessionID != "" {
		httpRequest.Header.Set("Mcp-Session-Id", sessionID)
	}

	httpResponse, err := c.httpClient.Do(httpRequest)
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		_ = httpResponse.Body.Close()
	}()

	if httpResponse.StatusCode == http.StatusAccepted {
		return httpResponse, &JsonRpcResponse{}, nil
	}

	contentType := httpResponse.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "text/event-stream") {
		rpcResponse, err := parseSSE(httpResponse.Body)
		if err != nil {
			return httpResponse, nil, err
		}
		return httpResponse, rpcResponse, nil
	}

	var rpcResponse JsonRpcResponse
	if err := json.NewDecoder(httpResponse.Body).Decode(&rpcResponse); err != nil {
		return httpResponse, nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return httpResponse, &rpcResponse, nil
}

func parseSSE(body interface{ Read([]byte) (int, error) }) (*JsonRpcResponse, error) {
	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")
		var rpcResp JsonRpcResponse
		if err := json.Unmarshal([]byte(payload), &rpcResp); err != nil {
			continue
		}
		return &rpcResp, nil
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading SSE stream: %w", err)
	}

	return nil, fmt.Errorf("no JSON-RPC response found in SSE stream")
}

func ParseClaudeStatus(output string) ClaudeStatus {
	normalized := strings.TrimSpace(strings.ToLower(output))

	switch {
	case strings.Contains(normalized, "running"):
		return ClaudeStatusRunning
	case strings.Contains(normalized, "finished"):
		return ClaudeStatusFinished
	case strings.Contains(normalized, "lost"):
		return ClaudeStatusLost
	case strings.Contains(normalized, "idle"):
		return ClaudeStatusIdle
	default:
		return ClaudeStatusUnknown
	}
}
