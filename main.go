package main

import (
	"fmt"
	"log"
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

	logger.Logger.Info(fmt.Sprintf("Initializing Ping42 Telemetry Server %v commit %v", version, commit))

	configuration = config.GetConfig()
	var err error

	// TODO: down the line of using gorm/redis clients, we need to wrap this and add a retry mechanism
	//  Furthermore, won't be as easy to mock and test
	gormClient, err = db.InitPostgreeDatabase(configuration.PostgreeDBDsn)
	if err != nil {
		logger.LogError(err.Error(), "error while connectToWsServer()", serverLogger)
		panic(err.Error())
	}

	redisClient, err = db.InitRedis(configuration.RedisHost, configuration.RedisPassword)
	if err != nil {
		logger.LogError(err.Error(), "error while InitRedis()", serverLogger)
		panic(err.Error())
	}
}

func main() {

	// set up logging
	log.SetFlags(0)

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
