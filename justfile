version := `git describe --tags --always --dirty 2>/dev/null || echo "dev"`
ldflags := "-X github.com/Easy-Infra-Ltd/easy-mcp-gateway/src/transport.Version=" + version

build:
    go build -ldflags "{{ldflags}}" -o easymcpgateway

test:
    go test ./...

test-verbose:
    go test -v ./...

lint:
    golangci-lint run

run config="config.json":
    go run . {{config}}

clean:
    rm -f easymcpgateway
