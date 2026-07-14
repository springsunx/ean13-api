package api

import (
	"testing"
)

func TestValidateEAN13(t *testing.T) {
	tests := []struct {
		name    string
		code    string
		wantErr bool
	}{
		{"valid 8413000065504", "8413000065504", false},
		{"valid 4006381333931", "4006381333931", false},
		{"valid 5901234123457", "5901234123457", false},
		{"too short", "123456789012", true},
		{"too long", "12345678901234", true},
		{"contains letter", "841300006550A", true},
		{"bad check digit", "8413000065505", true},
		{"all zeros valid", "0000000000000", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateEAN13(tt.code)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateEAN13(%q) error = %v, wantErr %v", tt.code, err, tt.wantErr)
			}
		})
	}
}

func TestExtractJSONFromText(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string // empty means no JSON found
	}{
		{"empty", "", ""},
		{"plain text", "no json here", ""},
		{"direct object", `{"name":"test"}`, `{"name":"test"}`},
		{"direct array", `[{"name":"test"}]`, `[{"name":"test"}]`},
		{"json after text", `Here is the result: {"name":"Coca Cola","brand":"Coca-Cola"}`, `{"name":"Coca Cola","brand":"Coca-Cola"}`},
		{"json in markdown", "```json\n{\"name\":\"test\"}\n```", `{"name":"test"}`},
		{"json in markdown no lang", "```\n{\"name\":\"test\"}\n```", `{"name":"test"}`},
		{"json with trailing text", `{"data":{"name":"test"},"msg":"ok"}  - some description text`, `{"data":{"name":"test"},"msg":"ok"}`},
		{"json with trailing multiline", "{\"data\":{\"name\":\"老油条\"},\"code\":200}\n  - 参数名称: code\n  - 参数名称: data", `{"data":{"name":"老油条"},"code":200}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractJSONFromText(tt.input)
			if got != tt.want {
				t.Errorf("extractJSONFromText(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestUnwrapArrayElement(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantOut string
		wantOK  bool
	}{
		{"empty array", "[]", "[]", false},
		{"single element", `[{"name":"test"}]`, `{"name":"test"}`, true},
		{"multi element", `[{"name":"a"},{"name":"b"}]`, `{"name":"a"}`, true},
		{"not array", `{"name":"test"}`, `{"name":"test"}`, false},
		{"plain text", "hello", "hello", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := unwrapArrayElement(tt.input)
			if ok != tt.wantOK {
				t.Errorf("unwrapArrayElement(%q) ok = %v, want %v", tt.input, ok, tt.wantOK)
			}
			if got != tt.wantOut {
				t.Errorf("unwrapArrayElement(%q) = %q, want %q", tt.input, got, tt.wantOut)
			}
		})
	}
}

func TestUnwrapKnownWrapper(t *testing.T) {
	tests := []struct {
		name string
		obj  map[string]any
		want string // expected key to exist in result
	}{
		{
			"direct fields",
			map[string]any{"name": "Coca Cola"},
			"name",
		},
		{
			"result wrapper",
			map[string]any{"result": map[string]any{"name": "Coca Cola"}},
			"name",
		},
		{
			"data wrapper",
			map[string]any{"data": map[string]any{"brand": "Coca-Cola"}},
			"brand",
		},
		{
			"no wrapper key",
			map[string]any{"foo": "bar", "name": "test"},
			"name",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := unwrapKnownWrapper(tt.obj)
			if _, ok := got[tt.want]; !ok {
				t.Errorf("unwrapKnownWrapper result missing key %q, got %v", tt.want, got)
			}
		})
	}
}

func TestValueToString(t *testing.T) {
	tests := []struct {
		name string
		val  any
		want string
	}{
		{"nil", nil, ""},
		{"string", "hello", "hello"},
		{"int-like float", float64(42), "42"},
		{"float", float64(3.14), "3.14"},
		{"bool true", true, "true"},
		{"bool false", false, "false"},
		{"map", map[string]any{"a": 1}, `{"a":1}`},
		{"array", []any{1, 2}, `[1,2]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := valueToString(tt.val)
			if got != tt.want {
				t.Errorf("valueToString(%v) = %q, want %q", tt.val, got, tt.want)
			}
		})
	}
}

func TestParseStructuredFields_DirectJSON(t *testing.T) {
	raw := `{"name":"Coca Cola","brand":"Coca-Cola","category":"饮料","price":"3.50","description":"经典可乐 330ml"}`
	resp := &BarcodeLookupResponse{Success: true, RawContent: raw}
	parseStructuredFields(raw, resp)

	if resp.Name != "Coca Cola" {
		t.Errorf("Name = %q, want %q", resp.Name, "Coca Cola")
	}
	if resp.Brand != "Coca-Cola" {
		t.Errorf("Brand = %q, want %q", resp.Brand, "Coca-Cola")
	}
	if resp.Category != "饮料" {
		t.Errorf("Category = %q, want %q", resp.Category, "饮料")
	}
	if resp.Price != "3.50" {
		t.Errorf("Price = %q, want %q", resp.Price, "3.50")
	}
	if resp.Description != "经典可乐 330ml" {
		t.Errorf("Description = %q, want %q", resp.Description, "经典可乐 330ml")
	}
	if resp.RawContent != "" {
		t.Errorf("RawContent should be cleared, got %q", resp.RawContent)
	}
}

func TestParseStructuredFields_WithTextPrefix(t *testing.T) {
	raw := `Here is the product information: {"name":"Sprite","brand":"Coca-Cola","price":"2.80"}`
	resp := &BarcodeLookupResponse{Success: true, RawContent: raw}
	parseStructuredFields(raw, resp)

	if resp.Name != "Sprite" {
		t.Errorf("Name = %q, want %q", resp.Name, "Sprite")
	}
	if resp.RawContent != "" {
		t.Errorf("RawContent should be cleared, got %q", resp.RawContent)
	}
}

func TestParseStructuredFields_Array(t *testing.T) {
	raw := `[{"name":"Product A","brand":"Brand A"},{"name":"Product B","brand":"Brand B"}]`
	resp := &BarcodeLookupResponse{Success: true, RawContent: raw}
	parseStructuredFields(raw, resp)

	if resp.Name != "Product A" {
		t.Errorf("Name = %q, want %q", resp.Name, "Product A")
	}
	if resp.RawContent != "" {
		t.Errorf("RawContent should be cleared, got %q", resp.RawContent)
	}
}

func TestParseStructuredFields_NestedWrapper(t *testing.T) {
	raw := `{"result":{"name":"Fanta","brand":"Coca-Cola","price":"3.00","size":"500ml"}}`
	resp := &BarcodeLookupResponse{Success: true, RawContent: raw}
	parseStructuredFields(raw, resp)

	if resp.Name != "Fanta" {
		t.Errorf("Name = %q, want %q", resp.Name, "Fanta")
	}
	if resp.Spec != "500ml" {
		t.Errorf("Spec = %q, want %q", resp.Spec, "500ml")
	}
}

func TestParseStructuredFields_MarkdownCodeBlock(t *testing.T) {
	raw := "Here is the result:\n```json\n{\"name\":\"Pepsi\",\"brand\":\"PepsiCo\"}\n```"
	resp := &BarcodeLookupResponse{Success: true, RawContent: raw}
	parseStructuredFields(raw, resp)

	if resp.Name != "Pepsi" {
		t.Errorf("Name = %q, want %q", resp.Name, "Pepsi")
	}
}

func TestParseStructuredFields_NestedFieldValues(t *testing.T) {
	raw := `{"name":"Product","spec":{"weight":"500g","dimensions":"10x20x30cm"},"tags":["food","drink"]}`
	resp := &BarcodeLookupResponse{Success: true, RawContent: raw}
	parseStructuredFields(raw, resp)

	if resp.Name != "Product" {
		t.Errorf("Name = %q, want %q", resp.Name, "Product")
	}
	// Nested objects should be JSON-stringified, not %v-formatted
	if resp.Spec == "" {
		t.Error("Spec should not be empty for nested object")
	}
}

func TestParseStructuredFields_ImageURL(t *testing.T) {
	// Valid HTTP URL
	raw := `{"name":"Test","image":"https://example.com/img.jpg"}`
	resp := &BarcodeLookupResponse{Success: true, RawContent: raw}
	parseStructuredFields(raw, resp)
	if resp.ImageURL != "https://example.com/img.jpg" {
		t.Errorf("ImageURL = %q, want %q", resp.ImageURL, "https://example.com/img.jpg")
	}

	// Invalid (non-HTTP) URL should be rejected
	raw2 := `{"name":"Test","image":"file:///etc/passwd"}`
	resp2 := &BarcodeLookupResponse{Success: true, RawContent: raw2}
	parseStructuredFields(raw2, resp2)
	if resp2.ImageURL != "" {
		t.Errorf("ImageURL should be empty for non-HTTP URL, got %q", resp2.ImageURL)
	}
}

func TestParseStructuredFields_AlternateFieldNames(t *testing.T) {
	raw := `{"product_name":"Test Product","manufacturer":"Acme Corp","specification":"500ml","thumbnail":"https://example.com/thumb.png"}`
	resp := &BarcodeLookupResponse{Success: true, RawContent: raw}
	parseStructuredFields(raw, resp)

	if resp.Name != "Test Product" {
		t.Errorf("Name = %q, want %q", resp.Name, "Test Product")
	}
	if resp.Manufacturer != "Acme Corp" {
		t.Errorf("Manufacturer = %q, want %q", resp.Manufacturer, "Acme Corp")
	}
	if resp.Spec != "500ml" {
		t.Errorf("Spec = %q, want %q", resp.Spec, "500ml")
	}
	if resp.ImageURL != "https://example.com/thumb.png" {
		t.Errorf("ImageURL = %q, want %q", resp.ImageURL, "https://example.com/thumb.png")
	}
}

func TestParseStructuredFields_NonJSON(t *testing.T) {
	raw := "This is just a plain text response with no JSON"
	resp := &BarcodeLookupResponse{Success: true, RawContent: raw}
	parseStructuredFields(raw, resp)

	// Should keep raw content unchanged
	if resp.RawContent != raw {
		t.Errorf("RawContent should be preserved for non-JSON, got %q", resp.RawContent)
	}
	if resp.Name != "" {
		t.Errorf("Name should be empty for non-JSON, got %q", resp.Name)
	}
}

func TestParseStructuredFields_EmptyString(t *testing.T) {
	resp := &BarcodeLookupResponse{Success: true, RawContent: ""}
	parseStructuredFields("", resp)
	if resp.Name != "" {
		t.Errorf("Name should be empty for empty input, got %q", resp.Name)
	}
}

func TestParseStructuredFields_IntPrice(t *testing.T) {
	raw := `{"name":"Test","price":42}`
	resp := &BarcodeLookupResponse{Success: true, RawContent: raw}
	parseStructuredFields(raw, resp)

	if resp.Price != "42" {
		t.Errorf("Price = %q, want %q", resp.Price, "42")
	}
}

func TestMatchTool(t *testing.T) {
	// This tests the matchTool function with mcp.Tool objects
	// We can't easily create mcp.Tool objects without importing the package,
	// but we can test that the function doesn't panic with empty input
	idx, ambiguous := matchTool(nil)
	if idx != -1 || ambiguous {
		t.Errorf("matchTool(nil) = (%d, %v), want (-1, false)", idx, ambiguous)
	}
}

func TestParseStructuredFields_RealAPIResponse(t *testing.T) {
	// Simulates the exact response from the real barcode lookup MCP tool
	// with all fields populated including the ones previously empty
	raw := `{"data":{"manuName":"四川省喜富食品有限公司","code":"6977809080172","depth":"10","width":"20","hight":"30","trademark":"金排","gpc":"10000301","goodsName":"老油条","gpcType":"非即食型咸味面粉制品、餐食（冷冻）","nw":"500g","gw":"550g","price":"15.8","note":"经典口味","keyword":"老油条,面食,膨化","manuAddress":"四川省成都市","imgList":["https://example.com/img1.jpg","https://example.com/img2.jpg"]},"msg":"成功","success":true,"code":200,"taskNo":"517824641208931748597356"}`
	resp := &BarcodeLookupResponse{Success: true, RawContent: raw}
	parseStructuredFields(raw, resp)

	if resp.Name != "老油条" {
		t.Errorf("Name = %q, want %q", resp.Name, "老油条")
	}
	// Brand comes from trademark, Manufacturer from manuName
	if resp.Brand != "金排" {
		t.Errorf("Brand = %q, want %q", resp.Brand, "金排")
	}
	if resp.Manufacturer != "四川省喜富食品有限公司" {
		t.Errorf("Manufacturer = %q, want %q", resp.Manufacturer, "四川省喜富食品有限公司")
	}
	if resp.Category != "非即食型咸味面粉制品、餐食（冷冻）" {
		t.Errorf("Category = %q, want %q", resp.Category, "非即食型咸味面粉制品、餐食（冷冻）")
	}
	if resp.Barcode != "6977809080172" {
		t.Errorf("Barcode = %q, want %q", resp.Barcode, "6977809080172")
	}
	if resp.Price != "15.8" {
		t.Errorf("Price = %q, want %q", resp.Price, "15.8")
	}
	if resp.Description != "经典口味" {
		t.Errorf("Description = %q, want %q", resp.Description, "经典口味")
	}
	if resp.Keywords != "老油条,面食,膨化" {
		t.Errorf("Keywords = %q, want %q", resp.Keywords, "老油条,面食,膨化")
	}
	if resp.ManufacturerAddress != "四川省成都市" {
		t.Errorf("ManufacturerAddress = %q, want %q", resp.ManufacturerAddress, "四川省成都市")
	}
	if resp.ImageURL != "https://example.com/img1.jpg" {
		t.Errorf("ImageURL = %q, want %q", resp.ImageURL, "https://example.com/img1.jpg")
	}
	if len(resp.ImageList) != 2 {
		t.Errorf("ImageList len = %d, want 2", len(resp.ImageList))
	}
	// Spec should contain one of the dimension/weight fields (last one wins since they all map to Spec)
	if resp.Spec == "" {
		t.Error("Spec should not be empty (width/hight/depth/nw/gw all map to Spec)")
	}
	if resp.RawContent != "" {
		t.Errorf("RawContent should be cleared, got %q", resp.RawContent)
	}
}

func TestParseStructuredFields_ManufacturerAddress(t *testing.T) {
	raw := `{"data":{"goodsName":"测试商品","trademark":"测试品牌","manuName":"测试厂商","manuAddress":"江苏省苏州市工业园区"}}`
	resp := &BarcodeLookupResponse{Success: true, RawContent: raw}
	parseStructuredFields(raw, resp)

	if resp.Name != "测试商品" {
		t.Errorf("Name = %q, want %q", resp.Name, "测试商品")
	}
	if resp.Brand != "测试品牌" {
		t.Errorf("Brand = %q, want %q", resp.Brand, "测试品牌")
	}
	if resp.Manufacturer != "测试厂商" {
		t.Errorf("Manufacturer = %q, want %q", resp.Manufacturer, "测试厂商")
	}
	if resp.ManufacturerAddress != "江苏省苏州市工业园区" {
		t.Errorf("ManufacturerAddress = %q, want %q", resp.ManufacturerAddress, "江苏省苏州市工业园区")
	}
	if resp.RawContent != "" {
		t.Errorf("RawContent should be cleared, got %q", resp.RawContent)
	}
}
