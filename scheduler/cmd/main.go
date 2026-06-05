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

	if cfg.LogPath != "" {
		logger.InitWithFile(cfg.LogLevel, cfg.LogFormat, cfg.LogPath)
	} else {
		logger.Init(cfg.LogLevel, cfg.LogFormat)
	}

	app := NewApp(cfg)
	app.Run()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	for {
		sig := <-sigChan
		switch sig {
		case syscall.SIGHUP:
			logger.Info("received SIGHUP signal, reloading config")
			if err := app.ReloadConfig(); err != nil {
				logger.Error("config reload failed", "error", err)
			} else {
				logger.Info("config reload succeeded")
			}
		case syscall.SIGINT, syscall.SIGTERM:
			logger.Info("received shutdown signal")
			app.Shutdown()
			return
		}
	}
}