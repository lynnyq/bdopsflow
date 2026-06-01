package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/lynnyq/bdopsflow/scheduler/internal/config"
	"github.com/lynnyq/bdopsflow/scheduler/internal/logger"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "keygen":
			runKeygen()
			return
		case "encrypt-password":
			runEncryptPassword()
			return
		case "decrypt-password":
			runDecryptPassword()
			return
		case "-h", "--help", "help":
			printHelp()
			return
		}
	}

	flag.Usage = printHelp
	configFile := flag.String("config", "", "path to config file (default: config.yaml in current directory)")
	advertiseAddr := flag.String("advertise-addr", "", "externally reachable HTTP address for cluster deployment (format: host:port, overrides app.advertise_addr)")
	flag.Parse()

	logger.Init("info", "json")

	cfg := config.Load(*configFile)

	if *advertiseAddr != "" {
		cfg.AdvertiseAddr = *advertiseAddr
	}

	logger.Init(cfg.LogLevel, cfg.LogFormat)

	app := NewApp(cfg)
	app.Run()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	app.Shutdown()
}
