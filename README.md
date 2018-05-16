# EAN-13 Barcode Decoder API

A lightweight REST API and MCP server for decoding EAN-13 barcodes from images, built with Go.

## Features

- **REST API** — Upload image, get decoded barcode
- **MCP Server** — AI tool integration via stdio or SSE
- **Docker** — One-command deployment

## REST API

### POST /api/decode

```bash
curl -X POST http://localhost:8080/api/decode -F "image=@barcode.png"
```

Response:

```json
{"success": true, "text": "5901234123457", "format": "EAN_13"}
```

### GET /health

```bash
curl http://localhost:8080/health
```

## MCP (Model Context Protocol)

### Tools

| Tool | Description |
|------|-------------|
| `decode_ean13` | Decode EAN-13 from image file path |
| `decode_ean13_base64` | Decode EAN-13 from base64 image |

### Stdio mode (for AI assistants)

```bash
go run ./cmd/mcp
```

Claude Desktop config (`claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "ean13-api": {
      "type": "stdio",
      "command": "ean13-api-mcp.exe",
      "args": []
    }
  }
}
```

### SSE mode (remote)

The HTTP server exposes MCP via StreamableHTTP:

```
Endpoint:  POST /mcp
```

Web 客户端（如 MCP Inspector）连接地址：`http://localhost:8080/mcp`

## Quick Start

### Run locally

```bash
# REST API + MCP SSE
go run ./cmd/server

# MCP stdio only
go run ./cmd/mcp
```

### Docker

```bash
docker compose up -d
```

### Build

```bash
go build -o ean13-api.exe ./cmd/server
go build -o ean13-api-mcp.exe ./cmd/mcp
```

## Testing

```bash
go test ./...
```

## Project Structure

```
.
├── api/              # REST API handlers
├── cmd/server/       # HTTP server (REST + MCP SSE)
├── cmd/mcp/          # MCP stdio server
├── mcpserver/        # MCP tool definitions
├── oned/             # EAN-13 barcode reader
├── common/           # Shared utilities
├── Dockerfile
├── docker-compose.yml
└── README.md
```

## License

See [LICENSE](LICENSE).
