.PHONY: build build-mcp test run run-mcp docker clean

build:
	go build -o ean13-api.exe ./cmd/server

build-mcp:
	go build -o ean13-api-mcp.exe ./cmd/mcp

test:
	go test ./...

run:
	go run ./cmd/server

run-mcp:
	go run ./cmd/mcp

docker:
	docker compose up --build -d

clean:
	rm -f ean13-api.exe ean13-api-mcp.exe
