package main

import (
	"net"
	"os"
	"sync"

	"github.com/go-redis/redis"
	"github.com/google/uuid"
	"github.com/jessevdk/go-flags"
	"github.com/ping-42/42lib/config"
	"github.com/ping-42/42lib/config/consts"
	"github.com/ping-42/42lib/db"
	"github.com/ping-42/42lib/logger"

	"github.com/ping-42/server/cmd"
	log "github.com/sirupsen/logrus"

	"gorm.io/gorm"
)

// sensorConnection define ws client connection
type sensorConnection struct {
	// Uuid unique id per each connection
	ConnectionId uuid.UUID
	// Connection ws connection
	Connection net.Conn
	// models.Sensor.ID
	SensorId uuid.UUID
}

var (
	sensorConnections = make(map[uuid.UUID]sensorConnection)
	connLock          = sync.Mutex{}
	serverLogger      = logger.Base("server")
)

var gormClient *gorm.DB
var configuration config.Configuration
var redisClient *redis.Client
var ws42 wsServer

// Release versioning magic
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func init() {

	serverLogger.WithFields(log.Fields{
		"version":   version,
		"commit":    commit,
		"buildDate": date,
	}).Info("Starting PING42 Telemetry Server...")

	configuration = config.GetConfig()
	var err error

	// TODO: down the line of using gorm/redis clients, we need to wrap this and add a retry mechanism
	// TODO: Furthermore, won't be as easy to mock and test
	gormClient, err = db.InitPostgreeDatabase(configuration.PostgreeDBDsn)
	if err != nil {
		serverLogger.WithFields(log.Fields{
			"error": err.Error(),
		}).Error("Unable to connect to Postgre Database")
		os.Exit(3)
	}

	redisClient, err = db.InitRedis(configuration.RedisHost, configuration.RedisPassword)
	if err != nil {
		serverLogger.WithFields(log.Fields{
			"error": err.Error(),
		}).Error("Unable to connect to Redis Database")
		os.Exit(4)
	}
}

func main() {

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
		DbClient: gormClient,
		Logger:   serverLogger,
	})

	// subscribe to the redis channel
	pubsub := redisClient.Subscribe(consts.SchedulerNewTaskChannel)
	defer pubsub.Close()
	go schedulerListener(pubsub)

	// run ws server
	ws42.run()
}
