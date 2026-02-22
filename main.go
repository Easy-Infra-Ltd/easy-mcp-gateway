package main

import (
	"context"
	"fmt"
	"os"

	logger "github.com/Easy-Infra-Ltd/easy-logger"

	"github.com/Easy-Infra-Ltd/easy-mcp-gateway/src/config"
	"github.com/Easy-Infra-Ltd/easy-mcp-gateway/src/gateway"
)

func main() {
	log := logger.CreateLoggerFromEnv(nil, "blue").With("process", "easymcpgateway")

	cfgPath := "config.json"
	if len(os.Args) > 1 {
		cfgPath = os.Args[1]
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	gw := gateway.New(cfg, log)
	if err := gw.Run(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "gateway: %v\n", err)
		os.Exit(1)
	}
}
