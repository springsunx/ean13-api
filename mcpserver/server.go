package mcpserver

import (
	"context"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/springsunx/ean13-api"
	"github.com/springsunx/ean13-api/oned"
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
		return mcp.NewToolResultError(err.Error()), nil
	}

	file, err := os.Open(path)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("cannot open file: %v", err)), nil
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("cannot decode image: %v", err)), nil
	}

	return decodeImage(img)
}

func handleDecodeBase64(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	b64, err := request.RequireString("image")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Strip data URI prefix if present
	if idx := strings.Index(b64, ","); idx != -1 {
		b64 = b64[idx+1:]
	}

	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid base64: %v", err)), nil
	}

	img, _, err := image.Decode(strings.NewReader(string(data)))
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("cannot decode image: %v", err)), nil
	}

	return decodeImage(img)
}

func decodeImage(img image.Image) (*mcp.CallToolResult, error) {
	bmp, err := gozxing.NewBinaryBitmapFromImage(img)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("bitmap error: %v", err)), nil
	}

	reader := oned.NewEAN13Reader()
	hints := map[gozxing.DecodeHintType]interface{}{
		gozxing.DecodeHintType_TRY_HARDER: true,
	}
	result, err := reader.Decode(bmp, hints)
	if err != nil {
		return mcp.NewToolResultError("no EAN-13 barcode found in image"), nil
	}

	return mcp.NewToolResultText(result.GetText()), nil
}
