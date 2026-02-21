package main

import (
	"bufio"
	"context"
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
)

//go:embed files/FILE1
var file1Content []byte

//go:embed files/FILE2
var file2Content []byte

// EmbeddedFile pairs embedded content with its destination filename.
type EmbeddedFile struct {
	Content  []byte
	DestName string
}

// TODO: Replace these destination filenames with the actual names you want.
var embeddedFiles = []EmbeddedFile{
	{Content: file1Content, DestName: "LICENSE"},
	{Content: file2Content, DestName: "CONTRIBUTING.md"},
}

// Exit codes for CLI mode
const (
	ExitSuccess = 0
	ExitError   = 1
)

// Result holds the outcome of an init operation.
type Result struct {
	Directory    string   `json:"directory"`
	FilesCreated []string `json:"files_created"`
}

// MCP JSON-RPC types

type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type JSONRPCResponse struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id"`
	Result  any    `json:"result,omitempty"`
	Error   *Error `json:"error,omitempty"`
}

type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type InitializeResult struct {
	ProtocolVersion string       `json:"protocolVersion"`
	ServerInfo      ServerInfo   `json:"serverInfo"`
	Capabilities    Capabilities `json:"capabilities"`
}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type Capabilities struct {
	Tools map[string]bool `json:"tools"`
}

type ToolsListResult struct {
	Tools []Tool `json:"tools"`
}

type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema InputSchema `json:"inputSchema"`
}

type InputSchema struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties"`
	Required   []string            `json:"required"`
}

type Property struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

type ToolCallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

type ToolCallResult struct {
	Content []ContentItem `json:"content"`
}

type ContentItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func main() {
	cliMode := flag.Bool("cli", false, "Run in CLI mode (default is MCP server mode)")
	directory := flag.String("directory", "", "Absolute path to the target directory")

	flag.Parse()

	if *cliMode {
		runCLI(*directory)
		return
	}

	runMCPServer()
}

func runCLI(directory string) {
	if directory == "" {
		fmt.Fprintln(os.Stderr, "Error: --directory is required in CLI mode")
		os.Exit(ExitError)
	}

	result, err := writeFiles(directory)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(ExitError)
	}

	output, err := json.Marshal(result)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling result: %v\n", err)
		os.Exit(ExitError)
	}

	fmt.Println(string(output))
}

func writeFiles(directory string) (*Result, error) {
	info, err := os.Stat(directory)
	if err != nil {
		return nil, fmt.Errorf("checking directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("not a directory: %s", directory)
	}

	var created []string

	for _, ef := range embeddedFiles {
		destPath := filepath.Join(directory, ef.DestName)

		if _, err := os.Stat(destPath); err == nil {
			return nil, fmt.Errorf("file already exists, refusing to overwrite: %s", destPath)
		}

		if err := os.WriteFile(destPath, ef.Content, 0644); err != nil {
			return nil, fmt.Errorf("writing %s: %w", ef.DestName, err)
		}

		created = append(created, destPath)
	}

	return &Result{
		Directory:    directory,
		FilesCreated: created,
	}, nil
}

func runMCPServer() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Fprintln(os.Stderr, "Received shutdown signal, exiting gracefully...")
		cancel()
	}()

	scanner := bufio.NewScanner(os.Stdin)

	lineChan := make(chan string)
	errChan := make(chan error, 1)

	go func() {
		for scanner.Scan() {
			lineChan <- scanner.Text()
		}
		if err := scanner.Err(); err != nil {
			errChan <- err
		}
		close(lineChan)
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case err := <-errChan:
			fmt.Fprintf(os.Stderr, "Scanner error: %v\n", err)
			return
		case line, ok := <-lineChan:
			if !ok {
				return
			}
			if line == "" {
				continue
			}

			var req JSONRPCRequest
			if err := json.Unmarshal([]byte(line), &req); err != nil {
				sendError(nil, -32700, "Parse error")
				continue
			}

			handleRequest(req)
		}
	}
}

func handleRequest(req JSONRPCRequest) {
	switch req.Method {
	case "initialize":
		handleInitialize(req)
	case "tools/list":
		handleToolsList(req)
	case "tools/call":
		handleToolsCall(req)
	default:
		sendError(req.ID, -32601, "Method not found")
	}
}

func handleInitialize(req JSONRPCRequest) {
	result := InitializeResult{
		ProtocolVersion: "2024-11-05",
		ServerInfo: ServerInfo{
			Name:    "init",
			Version: "1.0.0",
		},
		Capabilities: Capabilities{
			Tools: map[string]bool{
				"list": true,
				"call": true,
			},
		},
	}
	sendResponse(req.ID, result)
}

func handleToolsList(req JSONRPCRequest) {
	result := ToolsListResult{
		Tools: []Tool{
			{
				Name:        "init",
				Description: "Write embedded template files to a target directory. Refuses to overwrite existing files.",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"directory": {
							Type:        "string",
							Description: "Absolute path to the directory where files will be created",
						},
					},
					Required: []string{"directory"},
				},
			},
		},
	}
	sendResponse(req.ID, result)
}

func handleToolsCall(req JSONRPCRequest) {
	var params ToolCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		sendError(req.ID, -32602, "Invalid params")
		return
	}

	if params.Name != "init" {
		sendError(req.ID, -32602, "Unknown tool")
		return
	}

	directory, ok := params.Arguments["directory"].(string)
	if !ok || directory == "" {
		sendError(req.ID, -32602, "Missing or invalid 'directory' parameter")
		return
	}

	result, err := writeFiles(directory)
	if err != nil {
		sendError(req.ID, -32603, fmt.Sprintf("Init failed: %v", err))
		return
	}

	jsonResult, err := json.Marshal(result)
	if err != nil {
		sendError(req.ID, -32603, "Failed to marshal result")
		return
	}

	response := ToolCallResult{
		Content: []ContentItem{
			{
				Type: "text",
				Text: string(jsonResult),
			},
		},
	}

	sendResponse(req.ID, response)
}

func sendResponse(id any, result any) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	data, err := json.Marshal(resp)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to marshal response: %v\n", err)
		return
	}
	fmt.Println(string(data))
}

func sendError(id any, code int, message string) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &Error{
			Code:    code,
			Message: message,
		},
	}
	data, err := json.Marshal(resp)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to marshal error response: %v\n", err)
		return
	}
	fmt.Println(string(data))
}

