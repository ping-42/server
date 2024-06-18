package server

import (
	"net"
	"sync"

	"github.com/containerd/log"
	"github.com/go-redis/redis"
	"github.com/google/uuid"
	"github.com/ping-42/42lib/config/consts"
	"github.com/ping-42/42lib/logger"
	"gorm.io/gorm"
)

// sensorConnection define ws client connection
type sensorConnection struct {
	// Uuid unique id per each connection
	ConnectionId uuid.UUID
	// Connection ws connection
	// we do not need this field storing it to Redis
	Connection net.Conn `json:"-"`
	// models.Sensor.ID
	SensorId uuid.UUID
}

var (
	sensorConnections = make(map[uuid.UUID]sensorConnection)
	connLock          = sync.Mutex{}
	serverLogger      = logger.Base("server")
)

func Init(dbClient *gorm.DB, redisClient *redis.Client, logger *log.Entry, port string) {

	// subscribe to the redis channel
	pubsub := redisClient.Subscribe(consts.SchedulerNewTaskChannel)
	defer pubsub.Close()

	var ws42 = wsServer{
		dbClient:    dbClient,
		redisClient: redisClient,
	}

	// start listening for tasks
	go ws42.schedulerListener(dbClient, pubsub)

	// run ws server
	ws42.run(port)
}
