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
	flag.Parse()

	logger.Init()

	cfg := config.Load(*configFile)

	app := NewApp(cfg)
	app.Run()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	app.Shutdown()
}
