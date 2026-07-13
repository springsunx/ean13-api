package mcpserver

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/springsunx/ean13-api/decode"
)

// NewMCPServer creates a configured MCP server with EAN-13 decode tools.
func NewMCPServer() *server.MCPServer {
	s := server.NewMCPServer(
		"ean13-api",
		"1.0.0",
		server.WithToolCapabilities(false),
	)

	s.AddTool(decodeFileTool(), handleDecodeFile)
	s.AddTool(decodeBase64Tool(), handleDecodeBase64)

	return s
}

func decodeFileTool() mcp.Tool {
	return mcp.NewTool("decode_ean13",
		mcp.WithDescription("Decode an EAN-13 barcode from an image file on disk"),
		mcp.WithString("path",
			mcp.Required(),
			mcp.Description("Absolute path to the image file (PNG or JPEG)"),
		),
	)
}

func decodeBase64Tool() mcp.Tool {
	return mcp.NewTool("decode_ean13_base64",
		mcp.WithDescription("Decode an EAN-13 barcode from a base64-encoded image"),
		mcp.WithString("image",
			mcp.Required(),
			mcp.Description("Base64-encoded image data (PNG or JPEG)"),
		),
	)
}

func handleDecodeFile(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	path, err := request.RequireString("path")
	if err != nil {
		return mcp.NewToolResultError("missing required parameter: path"), nil
	}

	file, err := os.Open(path)
	if err != nil {
		return mcp.NewToolResultError("cannot open file: check that the path exists and is accessible"), nil
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return mcp.NewToolResultError("cannot decode image: unsupported or corrupted image format"), nil
	}

	return decodeImage(img)
}

func handleDecodeBase64(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	b64, err := request.RequireString("image")
	if err != nil {
		return mcp.NewToolResultError("missing required parameter: image"), nil
	}

	// Strip data URI prefix if present
	if idx := strings.Index(b64, ","); idx != -1 {
		b64 = b64[idx+1:]
	}

	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return mcp.NewToolResultError("invalid base64 data"), nil
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return mcp.NewToolResultError("cannot decode image: unsupported or corrupted image format"), nil
	}

	return decodeImage(img)
}

func decodeImage(img image.Image) (*mcp.CallToolResult, error) {
	result, err := decode.EAN13(img)
	if err != nil {
		return mcp.NewToolResultError("no EAN-13 barcode found in image"), nil
	}

	return mcp.NewToolResultText(result.Text), nil
}
