package main

import (
	"os"

	"github.com/jessevdk/go-flags"
	"github.com/ping-42/42lib/config"
	"github.com/ping-42/42lib/db"
	"github.com/ping-42/42lib/logger"

	"github.com/ping-42/server/cmd"
	log "github.com/sirupsen/logrus"
)

func main() {

	configuration := config.GetConfig()
	serverLogger := logger.Base("server")
	var err error

	gormClient, err := db.InitPostgreeDatabase(configuration.PostgreeDBDsn)
	if err != nil {
		serverLogger.WithFields(log.Fields{
			"error": err.Error(),
		}).Error("Unable to connect to Postgre Database")
		os.Exit(3)
	}

	redisClient, err := db.InitRedis(configuration.RedisHost, configuration.RedisPassword)
	if err != nil {
		serverLogger.WithFields(log.Fields{
			"error": err.Error(),
		}).Error("Unable to connect to Redis Database")
		os.Exit(4)
	}

	// Parse the command arguments first
	if _, err := cmd.Parser.Parse(); err != nil {
		switch flagsErr := err.(type) {
		case flags.ErrorType:
			if flagsErr == flags.ErrHelp {
				os.Exit(0)
			}
			serverLogger.Error(flagsErr)
			os.Exit(1)
		default:
			serverLogger.Error(flagsErr)
			os.Exit(1)
		}
	}

	// Handle the command flags
	cmd.Flags.Handle(cmd.HandleOpts{
		DbClient:    gormClient,
		RedisClient: redisClient,
		Logger:      serverLogger,
	})
}
