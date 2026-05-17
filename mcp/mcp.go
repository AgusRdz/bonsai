package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/AgusRdz/bonsai/agent"
	"github.com/AgusRdz/bonsai/git"
)

// Run starts the MCP stdio server and blocks until stdin is closed.
func Run(version string) {
	s := &server{g: git.New(), version: version}
	s.serve(os.Stdin, os.Stdout)
}

type server struct {
	g       *git.Runner
	version string
}

// rpcRequest is a JSON-RPC 2.0 request or notification.
// ID is RawMessage so we can distinguish absent (notification) from explicit null.
type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type toolContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type toolResult struct {
	Content []toolContent `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

func (s *server) serve(r io.Reader, w io.Writer) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	enc := json.NewEncoder(w)

	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}

		var req rpcRequest
		if err := json.Unmarshal(line, &req); err != nil {
			_ = enc.Encode(rpcResponse{
				JSONRPC: "2.0",
				Error:   &rpcError{Code: -32700, Message: "parse error: " + err.Error()},
			})
			continue
		}

		// Notifications have no id field - handle but don't respond.
		if len(req.ID) == 0 {
			continue
		}

		result, rpcErr := s.dispatch(req.Method, req.Params)
		resp := rpcResponse{JSONRPC: "2.0", ID: req.ID}
		if rpcErr != nil {
			resp.Error = rpcErr
		} else {
			resp.Result = result
		}
		_ = enc.Encode(resp)
	}
}

func (s *server) dispatch(method string, params json.RawMessage) (any, *rpcError) {
	switch method {
	case "initialize":
		return map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "bonsai", "version": s.version},
			"instructions": "When performing code review tasks, use bonsai tools instead of running git commands directly. " +
				"bonsai tools return structured, AI-optimized output. " +
				"Use git_review to compare changes against a base branch, " +
				"git_diff for staged/unstaged changes, " +
				"git_log for commit history, " +
				"git_show for a specific commit, " +
				"git_blame for line authorship, " +
				"git_status for working-tree state, " +
				"git_context for a full repo snapshot (status + diff + log combined), " +
				"git_branches for branch list, " +
				"git_stash_list for stash entries. " +
				"These tools are read-only. For write operations (commit, push, pull, merge, rebase) use git directly.",
		}, nil
	case "tools/list":
		return map[string]any{"tools": toolDefs()}, nil
	case "tools/call":
		return s.handleToolsCall(params)
	default:
		return nil, &rpcError{Code: -32601, Message: "method not found: " + method}
	}
}

func (s *server) handleToolsCall(raw json.RawMessage) (any, *rpcError) {
	var p struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, &rpcError{Code: -32602, Message: "invalid params: " + err.Error()}
	}
	if p.Arguments == nil {
		p.Arguments = map[string]any{}
	}

	ctx := context.Background()
	text, err := s.callTool(ctx, p.Name, p.Arguments)
	if err != nil {
		return toolResult{
			Content: []toolContent{{Type: "text", Text: err.Error()}},
			IsError: true,
		}, nil
	}
	return toolResult{Content: []toolContent{{Type: "text", Text: text}}}, nil
}

func (s *server) callTool(ctx context.Context, name string, args map[string]any) (string, error) {
	switch name {
	case "git_context":
		out, err := agent.BuildContext(ctx, s.g, intArg(args, "limit", 10), boolArg(args, "detailed"))
		if err != nil {
			return "", err
		}
		return agent.FormatMarkdown(out), nil

	case "git_status":
		out, err := agent.BuildStatus(ctx, s.g)
		if err != nil {
			return "", err
		}
		return agent.FormatMarkdown(out), nil

	case "git_log":
		out, err := agent.BuildLog(ctx, s.g, agent.LogParams{
			Limit: intArg(args, "limit", 20),
			Since: stringArg(args, "since"),
			Until: stringArg(args, "until"),
		})
		if err != nil {
			return "", err
		}
		return agent.FormatMarkdown(out), nil

	case "git_diff":
		out, err := agent.BuildDiff(ctx, s.g,
			stringArg(args, "file"),
			boolArg(args, "staged"),
			boolArg(args, "unstaged"),
			boolArg(args, "untracked"),
			boolArg(args, "detailed"),
		)
		if err != nil {
			return "", err
		}
		return agent.FormatMarkdown(out), nil

	case "git_show":
		ref := stringArg(args, "ref")
		if ref == "" {
			ref = "HEAD"
		}
		out, err := agent.BuildShow(ctx, s.g, ref, boolArg(args, "detailed"))
		if err != nil {
			return "", err
		}
		return agent.FormatMarkdown(out), nil

	case "git_blame":
		file := stringArg(args, "file")
		if file == "" {
			return "", fmt.Errorf("git_blame requires 'file' argument")
		}
		out, err := agent.BuildBlame(ctx, s.g, file)
		if err != nil {
			return "", err
		}
		return agent.FormatMarkdown(out), nil

	case "git_branches":
		out, err := agent.BuildBranches(ctx, s.g)
		if err != nil {
			return "", err
		}
		return agent.FormatMarkdown(out), nil

	case "git_stash_list":
		out, err := agent.BuildStashList(ctx, s.g)
		if err != nil {
			return "", err
		}
		return agent.FormatMarkdown(out), nil

	case "git_review":
		out, err := agent.BuildReview(ctx, s.g, stringArg(args, "base"), boolArg(args, "detailed"))
		if err != nil {
			return "", err
		}
		return agent.FormatMarkdown(out), nil

	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}
}

// ---------------------------------------------------------------------------
// Argument helpers
// ---------------------------------------------------------------------------

func intArg(args map[string]any, key string, def int) int {
	v, ok := args[key]
	if !ok {
		return def
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case string:
		if i, err := strconv.Atoi(n); err == nil {
			return i
		}
	}
	return def
}

func boolArg(args map[string]any, key string) bool {
	v, ok := args[key]
	if !ok {
		return false
	}
	b, _ := v.(bool)
	return b
}

func stringArg(args map[string]any, key string) string {
	v, ok := args[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

// ---------------------------------------------------------------------------
// Tool definitions
// ---------------------------------------------------------------------------

type propMap = map[string]map[string]any

func schema(properties propMap, required []string) map[string]any {
	s := map[string]any{"type": "object", "properties": properties}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

func toolDefs() []map[string]any {
	return []map[string]any{
		{
			"name":        "git_context",
			"description": "Full repository snapshot: branch status, working-tree diff, and recent commits. Call this first to understand the current state of the repo.",
			"inputSchema": schema(propMap{
				"limit":    {"type": "integer", "description": "Number of recent commits (default 10)"},
				"detailed": {"type": "boolean", "description": "Include full patch hunks in the diff"},
			}, nil),
		},
		{
			"name":        "git_status",
			"description": "Current repository status: branch, upstream tracking, staged/unstaged/untracked files, conflicts, and stash count.",
			"inputSchema": schema(propMap{}, nil),
		},
		{
			"name":        "git_log",
			"description": "Recent commit history with hash, subject, author, and date.",
			"inputSchema": schema(propMap{
				"limit": {"type": "integer", "description": "Maximum commits to return (default 20)"},
				"since": {"type": "string", "description": "Start date or expression, e.g. 'yesterday', '1 week ago', '2026-05-01'"},
				"until": {"type": "string", "description": "End date, e.g. '2026-05-17'"},
			}, nil),
		},
		{
			"name":        "git_diff",
			"description": "Working-tree changes grouped into staged, unstaged, and untracked. Returns file-level counts by default; use detailed=true for patch hunks.",
			"inputSchema": schema(propMap{
				"staged":    {"type": "boolean", "description": "Include staged changes (default: all scopes)"},
				"unstaged":  {"type": "boolean", "description": "Include unstaged changes"},
				"untracked": {"type": "boolean", "description": "Include untracked files"},
				"detailed":  {"type": "boolean", "description": "Include patch hunks"},
				"file":      {"type": "string", "description": "Filter to a single file path"},
			}, nil),
		},
		{
			"name":        "git_show",
			"description": "Details for a single commit: metadata, changed file counts, and optionally full patch hunks.",
			"inputSchema": schema(propMap{
				"ref":      {"type": "string", "description": "Commit ref (default HEAD)"},
				"detailed": {"type": "boolean", "description": "Include patch hunks"},
			}, nil),
		},
		{
			"name":        "git_blame",
			"description": "Line-by-line blame for a file: each line annotated with commit hash, author, and date.",
			"inputSchema": schema(propMap{
				"file": {"type": "string", "description": "File path to blame"},
			}, []string{"file"}),
		},
		{
			"name":        "git_branches",
			"description": "All local branches with current marker and upstream tracking info.",
			"inputSchema": schema(propMap{}, nil),
		},
		{
			"name":        "git_stash_list",
			"description": "All stash entries with ref and description.",
			"inputSchema": schema(propMap{}, nil),
		},
		{
			"name":        "git_review",
			"description": "Diff and commit context for code review, comparing HEAD against a base branch.",
			"inputSchema": schema(propMap{
				"base":     {"type": "string", "description": "Base branch or ref to compare against (e.g. 'main')"},
				"detailed": {"type": "boolean", "description": "Include patch hunks"},
			}, nil),
		},
	}
}
