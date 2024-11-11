package server

import (
	"github.com/containerd/log"
	"github.com/go-redis/redis"
	"github.com/google/uuid"
	"github.com/ping-42/42lib/config/consts"
	logger42 "github.com/ping-42/42lib/logger"
	"github.com/ping-42/42lib/wss"
	"gorm.io/gorm"
)

func Init(dbClient *gorm.DB, redisClient *redis.Client, logger *log.Entry, port string) {

	// subscribe to the redis channel
	pubsub := redisClient.Subscribe(consts.SchedulerNewTaskChannel)
	defer pubsub.Close()

	var ws42 = wsServer{
		dbClient:          dbClient,
		redisClient:       redisClient,
		redisPubSub:       pubsub,
		sensorConnections: make(map[uuid.UUID]wss.SensorConnection),
		serverLogger:      logger42.Base("server"),
	}

	// start listening for tasks
	go ws42.schedulerListener()

	// run ws server
	ws42.run(port)
}
