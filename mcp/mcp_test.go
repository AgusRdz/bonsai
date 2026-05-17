package mcp

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// runServer feeds newline-separated JSON messages to a server and returns all
// decoded responses.
func runServer(t *testing.T, messages ...string) []map[string]any {
	t.Helper()
	input := strings.Join(messages, "\n") + "\n"
	var out bytes.Buffer
	s := &server{version: "test"}
	s.serve(strings.NewReader(input), &out)

	var results []map[string]any
	dec := json.NewDecoder(&out)
	for dec.More() {
		var m map[string]any
		if err := dec.Decode(&m); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		results = append(results, m)
	}
	return results
}

func TestInitialize(t *testing.T) {
	resps := runServer(t, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)
	if len(resps) != 1 {
		t.Fatalf("expected 1 response, got %d", len(resps))
	}
	result, ok := resps[0]["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected result object, got %v", resps[0])
	}
	if result["protocolVersion"] != "2024-11-05" {
		t.Errorf("protocolVersion = %v, want 2024-11-05", result["protocolVersion"])
	}
	info, _ := result["serverInfo"].(map[string]any)
	if info["name"] != "bonsai" {
		t.Errorf("serverInfo.name = %v, want bonsai", info["name"])
	}
	if info["version"] != "test" {
		t.Errorf("serverInfo.version = %v, want test", info["version"])
	}
}

func TestToolsList(t *testing.T) {
	resps := runServer(t, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	if len(resps) != 1 {
		t.Fatalf("expected 1 response, got %d", len(resps))
	}
	result, ok := resps[0]["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected result object, got %v", resps[0])
	}
	tools, ok := result["tools"].([]any)
	if !ok {
		t.Fatalf("expected tools array, got %T", result["tools"])
	}
	if len(tools) < 9 {
		t.Errorf("expected at least 9 tools, got %d", len(tools))
	}

	names := make(map[string]bool)
	for _, raw := range tools {
		tool, _ := raw.(map[string]any)
		if n, _ := tool["name"].(string); n != "" {
			names[n] = true
		}
	}
	for _, want := range []string{
		"git_context", "git_status", "git_log", "git_diff",
		"git_show", "git_blame", "git_branches", "git_stash_list", "git_review",
	} {
		if !names[want] {
			t.Errorf("tool %q not found in tools/list", want)
		}
	}
}

func TestUnknownMethod(t *testing.T) {
	resps := runServer(t, `{"jsonrpc":"2.0","id":3,"method":"no/such/method"}`)
	if len(resps) != 1 {
		t.Fatalf("expected 1 response, got %d", len(resps))
	}
	rpcErr, ok := resps[0]["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error object, got %v", resps[0])
	}
	if rpcErr["code"].(float64) != -32601 {
		t.Errorf("error code = %v, want -32601", rpcErr["code"])
	}
}

func TestParseError(t *testing.T) {
	resps := runServer(t, `not valid json`)
	if len(resps) != 1 {
		t.Fatalf("expected 1 response, got %d", len(resps))
	}
	rpcErr, ok := resps[0]["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error object, got %v", resps[0])
	}
	if rpcErr["code"].(float64) != -32700 {
		t.Errorf("error code = %v, want -32700", rpcErr["code"])
	}
}

func TestNotificationNoResponse(t *testing.T) {
	// Notifications have no id - server must not emit a response.
	resps := runServer(t, `{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	if len(resps) != 0 {
		t.Errorf("expected no response for notification, got %d", len(resps))
	}
}

func TestToolsCallUnknownTool(t *testing.T) {
	msg := `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"no_such_tool","arguments":{}}}`
	resps := runServer(t, msg)
	if len(resps) != 1 {
		t.Fatalf("expected 1 response, got %d", len(resps))
	}
	result, ok := resps[0]["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected result, got %v", resps[0])
	}
	if result["isError"] != true {
		t.Errorf("expected isError=true for unknown tool")
	}
}

func TestToolsCallMissingRequiredArg(t *testing.T) {
	// git_blame requires 'file'
	msg := `{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"git_blame","arguments":{}}}`
	resps := runServer(t, msg)
	if len(resps) != 1 {
		t.Fatalf("expected 1 response, got %d", len(resps))
	}
	result, ok := resps[0]["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected result, got %v", resps[0])
	}
	if result["isError"] != true {
		t.Errorf("expected isError=true for missing required arg")
	}
}

func TestMultipleRequests(t *testing.T) {
	resps := runServer(t,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
	)
	// Notification produces no response, so 2 responses for 3 messages.
	if len(resps) != 2 {
		t.Fatalf("expected 2 responses, got %d", len(resps))
	}
}

func TestIDEchoedBack(t *testing.T) {
	resps := runServer(t, `{"jsonrpc":"2.0","id":"my-string-id","method":"tools/list"}`)
	if len(resps) != 1 {
		t.Fatalf("expected 1 response, got %d", len(resps))
	}
	if resps[0]["id"] != "my-string-id" {
		t.Errorf("id = %v, want my-string-id", resps[0]["id"])
	}
}
