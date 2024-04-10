package main

import (
	"encoding/json"
	"fmt"

	"github.com/containerd/log"
	"github.com/go-redis/redis"
	"github.com/ping-42/42lib/db/models"
	"github.com/ping-42/42lib/logger"
	"github.com/ping-42/42lib/sensorTask"
)

func schedulerListener(pubsub *redis.PubSub) {
	for {
		msg, err := pubsub.ReceiveMessage()
		if err != nil {
			logger.LogError(err.Error(), "error receiving message", serverLogger)
			return
		}

		var recevedTask sensorTask.Task
		err = json.Unmarshal([]byte(msg.Payload), &recevedTask)
		if err != nil {
			logger.LogError(err.Error(), fmt.Sprintf("error unmarshal message:%v", msg.Payload), serverLogger)
			continue
		}

		serverLogger = serverLogger.WithFields(log.Fields{
			"SensorID": recevedTask.SensorId,
			"TaskID":   recevedTask.Id,
		})

		serverLogger.Info(fmt.Sprintf("RecevedTask for SensorID:%v, TaskId:%v\n", recevedTask.SensorId, recevedTask.Id))

		// the sensor may not be connected to this server, in this case just pass the task
		wsConn, exists := ws42.getSensorWsConnection(recevedTask.SensorId)
		if !exists {
			serverLogger.Info(fmt.Sprintf("Not intrested passing... The sensor is not connected to this server. SensorID:%v, TaskId:%v\n", recevedTask.SensorId, recevedTask.Id))
			continue
		}

		// Update the task status to RECEIVED_BY_SERVER
		updateTx := gormClient.Model(&models.Task{}).Where("id = ?", recevedTask.Id).Update("task_status_id", 3)
		if updateTx.Error != nil {
			serverLogger.Info(fmt.Sprintf("updating TaskStatusID:%v, to RECEIVED_BY_SERVER err:%v", recevedTask.Id, updateTx.Error))
			return
		}

		// send the received message to the sensor
		err = ws42.sendTaskToSensors(wsConn, []byte(msg.Payload))
		if err != nil {
			logger.LogError(err.Error(), "sendTaskToSensors err:", serverLogger)
			return
		}

		// Update the task status to SENT_TO_SENSOR_BY_SERVER
		updateTx = gormClient.Model(&models.Task{}).Where("id = ?", recevedTask.Id).Update("task_status_id", 4)
		if updateTx.Error != nil {
			serverLogger.Info(fmt.Sprintf("updating TaskStatusID:%v, to SENT_TO_SENSOR_BY_SERVER err:%v", recevedTask.Id, updateTx.Error))
			return
		}
	}
}
