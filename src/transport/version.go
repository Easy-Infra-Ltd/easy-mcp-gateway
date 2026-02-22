package transport

// Version is the current build version, injected at build time via ldflags:
//
//	-X github.com/Easy-Infra-Ltd/easy-mcp-gateway/src/transport.Version=<tag>
//
// Defaults to "dev" when built without ldflags (local development).
var Version = "dev"
