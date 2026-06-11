package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/ammiranda/otf_api/otf_api"
)

type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      any         `json:"id"`
	Result  any         `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type InitializeResult struct {
	ProtocolVersion string          `json:"protocolVersion"`
	Capabilities    ServerCapabilities `json:"capabilities"`
	ServerInfo      ServerInfo      `json:"serverInfo"`
}

type ServerCapabilities struct {
	Tools *ToolsCapability `json:"tools,omitempty"`
}

type ToolsCapability struct {
	ListChanged bool `json:"listChanged"`
}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

type CallToolResult struct {
	Content []ToolContent `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

type ToolContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

var version = "0.1.0"

func main() {
	log.SetFlags(0)
	log.SetOutput(os.Stderr)

	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Printf("otf-mcp v%s\n", version)
		return
	}

	server := &MCPServer{}
	if err := server.Run(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

type MCPServer struct {
	client  *otf_api.Client
	ctx     context.Context
}

func (s *MCPServer) Run() error {
	s.ctx = context.Background()

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var req JSONRPCRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			s.writeError(nil, -32700, "Parse error")
			continue
		}

		var id any
		if req.ID != nil {
			json.Unmarshal(req.ID, &id)
		}

		switch req.Method {
		case "initialize":
			s.handleInitialize(id, req.Params)
		case "notifications/initialized":
			// no response needed
		case "tools/list":
			s.handleToolsList(id)
		case "tools/call":
			s.handleToolCall(id, req.Params)
		default:
			if id != nil {
				s.writeError(id, -32601, fmt.Sprintf("Method not found: %s", req.Method))
			}
		}
	}

	return scanner.Err()
}

func (s *MCPServer) handleInitialize(id any, params json.RawMessage) {
	result := InitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities: ServerCapabilities{
			Tools: &ToolsCapability{ListChanged: false},
		},
		ServerInfo: ServerInfo{
			Name:    "otf-mcp",
			Version: version,
		},
	}
	s.writeResult(id, result)
}

func (s *MCPServer) ensureClient() *otf_api.Client {
	if s.client != nil {
		return s.client
	}

	client := otf_api.NewClient()

	config, cfgErr := loadConfig()

	if cfgErr == nil && config.Token != "" {
		client.SetToken(config.Token)
		client.RefreshToken = config.RefreshToken
		if !client.NeedAuth() {
			s.client = client
			return client
		}
	}

	username, password := credsFromConfig(config)
	if username == "" || password == "" {
		username = os.Getenv("OTF_USERNAME")
		password = os.Getenv("OTF_PASSWORD")
	}
	if username == "" || password == "" {
		log.Fatal("No credentials available. Authenticate via the CLI with 'otf-cli auth', or set OTF_USERNAME and OTF_PASSWORD.")
	}

	if err := client.Authenticate(s.ctx, username, password); err != nil {
		log.Fatalf("Error authenticating: %v", err)
	}

	config.Username = username
	config.Password = password
	config.Token = client.Token
	config.RefreshToken = client.RefreshToken
	if saveErr := saveConfig(config); saveErr != nil {
		log.Printf("Warning: could not cache credentials: %v", saveErr)
	}

	s.client = client
	return client
}

func credsFromConfig(config otf_api.CLIConfig) (string, string) {
	if config.Username != "" && config.Password != "" {
		return config.Username, config.Password
	}
	return "", ""
}

func (s *MCPServer) handleToolsList(id any) {
	tools := []ToolDefinition{
		{
			Name:        "get_schedules",
			Description: "Fetch class schedules for OTF studios. Provide studio_ids as comma-separated UUIDs, or omit to use your preferred studios from configuration.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"studio_ids": {
						"type": "string",
						"description": "Comma-separated studio UUIDs (optional, uses preferred studios from config if omitted)"
					}
				}
			}`),
		},
		{
			Name:        "list_bookings",
			Description: "List your current and upcoming OTF bookings.",
			InputSchema: json.RawMessage(`{"type": "object", "properties": {}}`),
		},
		{
			Name:        "cancel_booking",
			Description: "Cancel an OTF booking by its booking ID.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"booking_id": {
						"type": "string",
						"description": "The booking ID to cancel"
					}
				},
				"required": ["booking_id"]
			}`),
		},
		{
			Name:        "book_class",
			Description: "Book an OTF class by its class ID. Use waitlist=true if the class is full.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"class_id": {
						"type": "string",
						"description": "The class ID to book (from get_schedules)"
					},
					"waitlist": {
						"type": "boolean",
						"description": "Join waitlist if class is full (default: false)"
					}
				},
				"required": ["class_id"]
			}`),
		},
		{
			Name:        "search_studios",
			Description: "Search for OTF studios near a location. Returns studio names, UUIDs, and distances.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"lat": {
						"type": "number",
						"description": "Latitude (e.g. 40.7128)"
					},
					"long": {
						"type": "number",
						"description": "Longitude (e.g. -74.0060)"
					},
					"distance": {
						"type": "number",
						"description": "Search radius in miles (default: 10)",
						"default": 10
					}
				},
				"required": ["lat", "long"]
			}`),
		},
	}

	s.writeResult(id, map[string]any{"tools": tools})
}

func (s *MCPServer) handleToolCall(id any, params json.RawMessage) {
	var call struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(params, &call); err != nil {
		s.writeError(id, -32602, "Invalid tool call params")
		return
	}

	client := s.ensureClient()

	var result CallToolResult

	switch call.Name {
	case "get_schedules":
		result = s.getSchedules(client, call.Arguments)
	case "list_bookings":
		result = s.listBookings(client)
	case "cancel_booking":
		result = s.cancelBooking(client, call.Arguments)
	case "book_class":
		result = s.bookClass(client, call.Arguments)
	case "search_studios":
		result = s.searchStudios(client, call.Arguments)
	default:
		s.writeError(id, -32601, fmt.Sprintf("Unknown tool: %s", call.Name))
		return
	}

	s.writeResult(id, result)
}

func (s *MCPServer) getSchedules(client *otf_api.Client, args json.RawMessage) CallToolResult {
	var params struct {
		StudioIDs string `json:"studio_ids"`
	}
	json.Unmarshal(args, &params)

	var idsToFetch []string
	if params.StudioIDs != "" {
		idsToFetch = strings.Split(params.StudioIDs, ",")
	} else {
		config, err := loadConfig()
		if err != nil || len(config.PreferredStudioIDs) == 0 {
			return CallToolResult{
				IsError: true,
				Content: []ToolContent{{Type: "text", Text: "No studio IDs provided and no preferred studios configured. Use search_studios to find studios first, or pass studio_ids."}},
			}
		}
		idsToFetch = config.PreferredStudioIDs
	}

	schedules, err := client.GetStudiosSchedules(s.ctx, idsToFetch)
	if err != nil {
		return CallToolResult{IsError: true, Content: []ToolContent{{Type: "text", Text: fmt.Sprintf("Error fetching schedules: %v", err)}}}
	}

	data, _ := json.MarshalIndent(schedules, "", "  ")
	return CallToolResult{Content: []ToolContent{{Type: "text", Text: string(data)}}}
}

func (s *MCPServer) listBookings(client *otf_api.Client) CallToolResult {
	startsAfter := time.Now().Truncate(24 * time.Hour)
	endsBefore := time.Now().AddDate(0, 0, 60)

	bookings, err := client.GetBookings(s.ctx, startsAfter, endsBefore, true)
	if err != nil {
		return CallToolResult{IsError: true, Content: []ToolContent{{Type: "text", Text: fmt.Sprintf("Error fetching bookings: %v", err)}}}
	}

	data, _ := json.MarshalIndent(bookings, "", "  ")
	return CallToolResult{Content: []ToolContent{{Type: "text", Text: string(data)}}}
}

func (s *MCPServer) cancelBooking(client *otf_api.Client, args json.RawMessage) CallToolResult {
	var params struct {
		BookingID string `json:"booking_id"`
	}
	if err := json.Unmarshal(args, &params); err != nil || params.BookingID == "" {
		return CallToolResult{IsError: true, Content: []ToolContent{{Type: "text", Text: "booking_id is required"}}}
	}

	if err := client.CancelBooking(s.ctx, params.BookingID); err != nil {
		return CallToolResult{IsError: true, Content: []ToolContent{{Type: "text", Text: fmt.Sprintf("Error canceling booking: %v", err)}}}
	}

	return CallToolResult{Content: []ToolContent{{Type: "text", Text: fmt.Sprintf("Successfully canceled booking %s", params.BookingID)}}}
}

func (s *MCPServer) bookClass(client *otf_api.Client, args json.RawMessage) CallToolResult {
	var params struct {
		ClassID  string `json:"class_id"`
		Waitlist bool   `json:"waitlist"`
	}
	if err := json.Unmarshal(args, &params); err != nil || params.ClassID == "" {
		return CallToolResult{IsError: true, Content: []ToolContent{{Type: "text", Text: "class_id is required"}}}
	}

	bookingReq := otf_api.CreateBookingRequest{
		ClassID:   params.ClassID,
		Confirmed: false,
		Waitlist:  params.Waitlist,
	}

	if err := client.BookClass(s.ctx, bookingReq); err != nil {
		return CallToolResult{IsError: true, Content: []ToolContent{{Type: "text", Text: fmt.Sprintf("Error booking class: %v", err)}}}
	}

	msg := "Successfully booked class %s"
	if params.Waitlist {
		msg = "Successfully added to waitlist for class %s"
	}
	return CallToolResult{Content: []ToolContent{{Type: "text", Text: fmt.Sprintf(msg, params.ClassID)}}}
}

func (s *MCPServer) searchStudios(client *otf_api.Client, args json.RawMessage) CallToolResult {
	var params struct {
		Lat      float64 `json:"lat"`
		Long     float64 `json:"long"`
		Distance float64 `json:"distance"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return CallToolResult{IsError: true, Content: []ToolContent{{Type: "text", Text: "Invalid parameters"}}}
	}
	if params.Distance <= 0 {
		params.Distance = 10
	}

	studios, err := client.ListStudios(s.ctx, params.Lat, params.Long, params.Distance)
	if err != nil {
		return CallToolResult{IsError: true, Content: []ToolContent{{Type: "text", Text: fmt.Sprintf("Error searching studios: %v", err)}}}
	}

	data, _ := json.MarshalIndent(studios, "", "  ")
	return CallToolResult{Content: []ToolContent{{Type: "text", Text: string(data)}}}
}

func (s *MCPServer) writeResult(id any, result any) {
	resp := JSONRPCResponse{JSONRPC: "2.0", ID: id, Result: result}
	data, _ := json.Marshal(resp)
	fmt.Println(string(data))
}

func (s *MCPServer) writeError(id any, code int, message string) {
	resp := JSONRPCResponse{JSONRPC: "2.0", ID: id, Error: &RPCError{Code: code, Message: message}}
	data, _ := json.Marshal(resp)
	fmt.Println(string(data))
}

func loadConfig() (otf_api.CLIConfig, error) {
	return otf_api.LoadConfig()
}

func saveConfig(config otf_api.CLIConfig) error {
	return otf_api.SaveConfig(config)
}
