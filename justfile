build:
    go build -o easymcpgateway

test:
    go test ./...

test-verbose:
    go test -v ./...

run config="config.json":
    go run . {{config}}

clean:
    rm -f easymcpgateway
