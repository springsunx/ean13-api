package api

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

// MCPConfig represents the user-provided MCP server configuration.
type MCPConfig struct {
	Type string `json:"type"` // "streamableHttp", "streamable-http", or "sse"
	URL  string `json:"url"`
}

// MCPToolInfo describes an available tool on the remote MCP server.
type MCPToolInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema any    `json:"inputSchema,omitempty"`
}

// MCPTestResponse is the JSON response from POST /api/mcp/test.
type MCPTestResponse struct {
	Success    bool          `json:"success"`
	ServerName string        `json:"serverName,omitempty"`
	Tools      []MCPToolInfo `json:"tools,omitempty"`
	Error      string        `json:"error,omitempty"`
}

// BarcodeLookupRequest is the JSON body for POST /api/barcode/lookup.
type BarcodeLookupRequest struct {
	EAN13    string    `json:"ean13"`
	MCP      MCPConfig `json:"mcp"`
	ToolName string    `json:"toolName,omitempty"`
	ParamName string  `json:"paramName,omitempty"`
}

// BarcodeLookupResponse is the JSON response from POST /api/barcode/lookup.
type BarcodeLookupResponse struct {
	Success    bool   `json:"success"`
	EAN13      string `json:"ean13,omitempty"`
	RawContent string `json:"rawContent,omitempty"`
	Name       string `json:"name,omitempty"`
	Brand      string `json:"brand,omitempty"`
	Manufacturer string `json:"manufacturer,omitempty"`
	Category   string `json:"category,omitempty"`
	Spec       string `json:"spec,omitempty"`
	Description string `json:"description,omitempty"`
	Price      string `json:"price,omitempty"`
	ManufacturerAddress string `json:"manufacturerAddress,omitempty"`
	ImageURL   string `json:"imageUrl,omitempty"`
	Barcode    string `json:"barcode,omitempty"`
	Keywords   string `json:"keywords,omitempty"`
	ImageList  []string `json:"imageList,omitempty"`
	Ambiguous  bool   `json:"ambiguous,omitempty"`
	AvailableTools []MCPToolInfo `json:"availableTools,omitempty"`
	Error      string `json:"error,omitempty"`
}

// newMCPClient creates a new MCP client based on the transport type.
func newMCPClient(cfg MCPConfig) (*mcpclient.Client, error) {
	switch strings.ToLower(cfg.Type) {
	case "streamablehttp", "streamable-http":
		return mcpclient.NewStreamableHttpClient(cfg.URL)
	case "sse":
		return mcpclient.NewSSEMCPClient(cfg.URL)
	default:
		return nil, fmt.Errorf("unsupported MCP transport type: %q (must be streamableHttp, streamable-http, or sse)", cfg.Type)
	}
}

// initMCPClient creates, starts, and initializes an MCP client.
func initMCPClient(ctx context.Context, cfg MCPConfig) (*mcpclient.Client, error) {
	c, err := newMCPClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create MCP client: %w", err)
	}

	if err := c.Start(ctx); err != nil {
		c.Close()
		return nil, fmt.Errorf("failed to start MCP client: %w", err)
	}

	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcp.Implementation{
		Name:    "ean13-api",
		Version: "1.0.0",
	}

	if _, err := c.Initialize(ctx, initReq); err != nil {
		c.Close()
		return nil, fmt.Errorf("failed to initialize MCP client: %w", err)
	}

	return c, nil
}

// testMCPConnection connects to the remote MCP server, lists tools, and returns server info.
func testMCPConnection(ctx context.Context, cfg MCPConfig) (*MCPTestResponse, error) {
	c, err := initMCPClient(ctx, cfg)
	if err != nil {
		return nil, err
	}
	defer c.Close()

	// List tools
	toolsReq := mcp.ListToolsRequest{}
	toolsResult, err := c.ListTools(ctx, toolsReq)
	if err != nil {
		return nil, fmt.Errorf("failed to list tools: %w", err)
	}

	tools := make([]MCPToolInfo, 0, len(toolsResult.Tools))
	for _, t := range toolsResult.Tools {
		tools = append(tools, MCPToolInfo{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
		})
	}

	return &MCPTestResponse{
		Success: true,
		Tools:   tools,
	}, nil
}

// barcodeParamKeywords defines priority order for auto-matching barcode parameter names.
var barcodeParamKeywords = []string{"barcode", "ean", "ean13", "code"}

// barcodeToolKeywords defines priority order for auto-matching barcode tool names.
var barcodeToolKeywords = []string{"barcode", "ean", "ean13", "lookup", "query"}

// matchTool auto-selects a tool from the list based on keyword matching.
// Returns the matched tool index and whether the match is ambiguous.
func matchTool(tools []mcp.Tool) (int, bool) {
	type candidate struct {
		index    int
		priority int // lower is better
	}

	var candidates []candidate
	for i, t := range tools {
		nameLower := strings.ToLower(t.Name)
		for pri, kw := range barcodeToolKeywords {
			if strings.Contains(nameLower, kw) {
				candidates = append(candidates, candidate{index: i, priority: pri})
				break
			}
		}
	}

	if len(candidates) == 0 {
		return -1, false
	}
	if len(candidates) > 1 {
		return -1, true // ambiguous
	}
	return candidates[0].index, false
}

// matchParam auto-selects a parameter name from the tool's input schema.
func matchParam(tool mcp.Tool) string {
	// Extract property names from InputSchema
	schemaBytes, err := json.Marshal(tool.InputSchema)
	if err != nil {
		return ""
	}

	var schema struct {
		Properties map[string]any `json:"properties"`
	}
	if err := json.Unmarshal(schemaBytes, &schema); err != nil {
		return ""
	}

	if schema.Properties == nil {
		return ""
	}

	propNames := make([]string, 0, len(schema.Properties))
	for name := range schema.Properties {
		propNames = append(propNames, name)
	}

	// Try priority keywords
	for _, kw := range barcodeParamKeywords {
		for _, name := range propNames {
			if strings.Contains(strings.ToLower(name), kw) {
				return name
			}
		}
	}

	// Fallback: return the first property
	if len(propNames) > 0 {
		return propNames[0]
	}
	return ""
}

// lookupBarcode connects to the MCP server and calls a tool to look up barcode info.
func lookupBarcode(ctx context.Context, req BarcodeLookupRequest) (*BarcodeLookupResponse, error) {
	// Validate EAN-13
	if err := validateEAN13(req.EAN13); err != nil {
		return nil, err
	}

	c, err := initMCPClient(ctx, req.MCP)
	if err != nil {
		return nil, err
	}
	defer c.Close()

	// List tools
	toolsReq := mcp.ListToolsRequest{}
	toolsResult, err := c.ListTools(ctx, toolsReq)
	if err != nil {
		return nil, fmt.Errorf("failed to list tools: %w", err)
	}

	if len(toolsResult.Tools) == 0 {
		return nil, fmt.Errorf("no tools available on the MCP server")
	}

	// Find the target tool
	var toolName string
	var paramName string

	if req.ToolName != "" {
		toolName = req.ToolName
	} else {
		idx, ambiguous := matchTool(toolsResult.Tools)
		if ambiguous {
			// Return available tools for the user to choose
			toolInfos := make([]MCPToolInfo, 0, len(toolsResult.Tools))
			for _, t := range toolsResult.Tools {
				toolInfos = append(toolInfos, MCPToolInfo{
					Name:        t.Name,
					Description: t.Description,
					InputSchema: t.InputSchema,
				})
			}
			return &BarcodeLookupResponse{
				Success:        false,
				Ambiguous:      true,
				AvailableTools: toolInfos,
				Error:          "multiple matching tools found, please select one",
			}, nil
		}
		if idx < 0 {
			return nil, fmt.Errorf("no suitable barcode lookup tool found on the server")
		}
		toolName = toolsResult.Tools[idx].Name
	}

	// Find the target tool object
	var targetTool *mcp.Tool
	for _, t := range toolsResult.Tools {
		if t.Name == toolName {
			tt := t
			targetTool = &tt
			break
		}
	}
	if targetTool == nil {
		return nil, fmt.Errorf("tool %q not found on the MCP server", toolName)
	}

	// Find parameter name
	if req.ParamName != "" {
		paramName = req.ParamName
	} else {
		paramName = matchParam(*targetTool)
		if paramName == "" {
			return nil, fmt.Errorf("could not determine parameter name for tool %q", toolName)
		}
	}

	// Call the tool
	callReq := mcp.CallToolRequest{}
	callReq.Params.Name = toolName
	callReq.Params.Arguments = map[string]any{
		paramName: req.EAN13,
	}

	callResult, err := c.CallTool(ctx, callReq)
	if err != nil {
		return nil, fmt.Errorf("tool call failed: %w", err)
	}

	if callResult == nil {
		return nil, fmt.Errorf("tool returned empty result")
	}

	// Extract raw content from result
	rawContent := extractTextContent(callResult)

	resp := &BarcodeLookupResponse{
		Success:    true,
		EAN13:      req.EAN13,
		RawContent: rawContent,
	}

	// Try to parse structured fields
	parseStructuredFields(rawContent, resp)

	return resp, nil
}

// extractTextContent extracts text from MCP CallToolResult.
func extractTextContent(result *mcp.CallToolResult) string {
	var parts []string
	for _, c := range result.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			parts = append(parts, tc.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// commonFieldNames maps JSON field name variations to our standard field names.
var commonFieldNames = map[string]string{
	"name":        "Name",
	"productname": "Name",
	"product_name": "Name",
	"goodsname":   "Name",
	"goods_name":  "Name",
	"title":       "Name",
	"brand":       "Brand",
	"trademark":   "Brand",
	"manufacturer": "Manufacturer",
	"manuname":    "Manufacturer",
	"manu_name":   "Manufacturer",
	"maker":       "Manufacturer",
	"category":    "Category",
	"gpctype":     "Category",
	"gpc_type":    "Category",
	"goodstype":   "Category",
	"goods_type":  "Category",
	"type":        "Category",
	"class":       "Category",
	"spec":        "Spec",
	"specification": "Spec",
	"size":        "Spec",
	"weight":      "Spec",
	"nw":          "Spec",
	"gw":          "Spec",
	"width":       "Spec",
	"hight":       "Spec",
	"height":      "Spec",
	"depth":       "Spec",
	"volume":      "Spec",
	"description": "Description",
	"desc":        "Description",
	"detail":      "Description",
	"summary":     "Description",
	"note":        "Description",
	"price":       "Price",
	"cost":        "Price",
	"amount":      "Price",
	"manuaddress": "ManufacturerAddress",
	"manu_address": "ManufacturerAddress",
	"barcode":     "Barcode",
	"code":        "Barcode",
	"keyword":     "Keywords",
	"image":       "ImageURL",
	"imageurl":    "ImageURL",
	"image_url":   "ImageURL",
	"img":         "ImageURL",
	"picture":     "ImageURL",
	"thumbnail":   "ImageURL",
	"photo":       "ImageURL",
	"sptmimg":     "ImageURL",
}

// isValidImageURL checks if the URL is a valid HTTP(S) URL.
var validImageURL = regexp.MustCompile(`^https?://`)

// extractJSONFromText attempts to find a JSON object or array embedded in arbitrary text.
// Handles: raw JSON, JSON inside markdown code blocks, JSON embedded after text prefix,
// and JSON followed by trailing non-JSON text (e.g. API parameter descriptions).
func extractJSONFromText(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}

	// Strip markdown code block if present anywhere: ```json ... ``` or ``` ... ```
	if startIdx := strings.Index(s, "```"); startIdx != -1 {
		afterStart := s[startIdx+3:]
		// Skip optional language identifier on the first line
		if nlIdx := strings.Index(afterStart, "\n"); nlIdx != -1 {
			afterStart = afterStart[nlIdx+1:]
		}
		// Find closing ```
		if endIdx := strings.Index(afterStart, "```"); endIdx != -1 {
			s = strings.TrimSpace(afterStart[:endIdx])
		}
	}

	// Try to find first '{' or '[' and extract the complete JSON by bracket depth
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch != '{' && ch != '[' {
			continue
		}

		// Scan from this position, tracking bracket depth
		openCh := ch
		var closeCh byte
		if ch == '{' {
			closeCh = '}'
		} else {
			closeCh = ']'
		}

		depth := 0
		inString := false
		escaped := false
		j := i
		for ; j < len(s); j++ {
			c := s[j]

			if escaped {
				escaped = false
				continue
			}

			if c == '\\' && inString {
				escaped = true
				continue
			}

			if c == '"' {
				inString = !inString
				continue
			}

			if inString {
				continue
			}

			if c == openCh {
				depth++
			} else if c == closeCh {
				depth--
				if depth == 0 {
					break
				}
			}
		}

		if depth != 0 {
			continue // unbalanced brackets, try next occurrence
		}

		candidate := s[i : j+1]
		if json.Valid([]byte(candidate)) {
			return candidate
		}
	}

	return ""
}

// unwrapKnownWrapper recursively unwraps known JSON wrapper keys to find the real data.
// e.g. {"result": {"name": ...}} → {"name": ...}
// e.g. {"data": {"products": [...]}} → first product object
func unwrapKnownWrapper(obj map[string]any) map[string]any {
	wrapperKeys := []string{"result", "data", "response", "product", "item", "info", "details", "body"}
	for _, key := range wrapperKeys {
		if val, ok := obj[key]; ok {
			switch v := val.(type) {
			case map[string]any:
				return v
			}
		}
	}
	return obj
}

// unwrapArrayElement extracts the first element from a JSON array, if applicable.
// If raw is a JSON array, returns the first element as a map; otherwise returns the input.
func unwrapArrayElement(raw string) (string, bool) {
	s := strings.TrimSpace(raw)
	if len(s) < 2 || s[0] != '[' {
		return raw, false
	}
	var arr []json.RawMessage
	if err := json.Unmarshal([]byte(s), &arr); err != nil {
		return raw, false
	}
	if len(arr) == 0 {
		return raw, false
	}
	return string(arr[0]), true
}

// valueToString converts a parsed JSON value to a clean display string.
// Handles nested objects/arrays by pretty-printing with JSON, scalars by direct conversion.
func valueToString(val any) string {
	if val == nil {
		return ""
	}
	switch v := val.(type) {
	case string:
		return v
	case float64:
		// Display integers without decimal point
		if v == float64(int64(v)) {
			return fmt.Sprintf("%d", int64(v))
		}
		return fmt.Sprintf("%g", v)
	case bool:
		if v {
			return "true"
		}
		return "false"
	case map[string]any, []any:
		b, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(b)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// parseStructuredFields tries to parse JSON content into structured fields.
// Handles: raw JSON objects, arrays (takes first element), JSON embedded in text,
// nested wrapper keys (result/data/response/etc.), and nested field values.
func parseStructuredFields(raw string, resp *BarcodeLookupResponse) {
	if raw == "" {
		return
	}

	// Step 1: Extract JSON from possible surrounding text
	jsonStr := extractJSONFromText(raw)
	if jsonStr == "" {
		return
	}

	// Step 2: If it's an array, take the first element
	if unwrapped, ok := unwrapArrayElement(jsonStr); ok {
		jsonStr = unwrapped
	}

	// Step 3: Parse as object
	var obj map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &obj); err != nil {
		return
	}

	// Step 4: Unwrap known wrapper keys
	obj = unwrapKnownWrapper(obj)

	// Step 5: Extract fields (sorted key order for deterministic "first write wins")
	hasField := false
	keys := make([]string, 0, len(obj))
	for key := range obj {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		val := obj[key]
		fieldName := commonFieldNames[strings.ToLower(strings.ReplaceAll(key, "-", "_"))]
		if fieldName == "" {
			continue
		}

		strVal := valueToString(val)
		strVal = strings.TrimSpace(strVal)
		if strVal == "" || strVal == "null" || strVal == "<nil>" {
			continue
		}

		switch {
		case fieldName == "Name" && resp.Name == "":
			resp.Name = strVal
		case fieldName == "Brand" && resp.Brand == "":
			resp.Brand = strVal
		case fieldName == "Manufacturer" && resp.Manufacturer == "":
			resp.Manufacturer = strVal
		case fieldName == "Category" && resp.Category == "":
			resp.Category = strVal
		case fieldName == "Spec" && resp.Spec == "":
			resp.Spec = strVal
		case fieldName == "Description" && resp.Description == "":
			resp.Description = strVal
		case fieldName == "Price" && resp.Price == "":
			resp.Price = strVal
		case fieldName == "ManufacturerAddress" && resp.ManufacturerAddress == "":
			resp.ManufacturerAddress = strVal
		case fieldName == "Barcode" && resp.Barcode == "":
			resp.Barcode = strVal
		case fieldName == "Keywords" && resp.Keywords == "":
			resp.Keywords = strVal
		case fieldName == "ImageURL" && resp.ImageURL == "" && validImageURL.MatchString(strVal):
			resp.ImageURL = strVal
		}
		hasField = true
	}

	// Step 6: Handle imageList/imgList (array of image URLs) separately
	// Keys are case-sensitive after unwrapping, so iterate and match case-insensitively
	imageListTargets := map[string]bool{"imagelist": true, "imglist": true, "images": true}
	for key, val := range obj {
		if !imageListTargets[strings.ToLower(key)] {
			continue
		}
		if arr, ok := val.([]any); ok {
			for _, item := range arr {
				if s, ok := item.(string); ok && s != "" {
					resp.ImageList = append(resp.ImageList, s)
					if resp.ImageURL == "" && validImageURL.MatchString(s) {
						resp.ImageURL = s
					}
				}
			}
			if len(resp.ImageList) > 0 {
				hasField = true
			}
		}
		break
	}

	// If we parsed at least one field, clear raw content
	if hasField {
		resp.RawContent = ""
	}
}

// validateEAN13 checks that the barcode is a valid EAN-13.
func validateEAN13(code string) error {
	if len(code) != 13 {
		return fmt.Errorf("invalid EAN-13: expected 13 digits, got %d", len(code))
	}
	for i, ch := range code {
		if ch < '0' || ch > '9' {
			return fmt.Errorf("invalid EAN-13: character %q at position %d is not a digit", ch, i)
		}
	}

	// Mod-10 check digit validation
	sum := 0
	for i := 0; i < 12; i++ {
		d := int(code[i] - '0')
		if i%2 == 0 {
			sum += d
		} else {
			sum += d * 3
		}
	}
	checkDigit := (10 - (sum % 10)) % 10
	expected := int(code[12] - '0')
	if checkDigit != expected {
		return fmt.Errorf("invalid EAN-13: check digit mismatch (expected %d, got %d)", checkDigit, expected)
	}

	return nil
}

