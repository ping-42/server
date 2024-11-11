package server

import (
	"encoding/json"
	"fmt"

	"github.com/containerd/log"
	"github.com/ping-42/42lib/db/models"
	"github.com/ping-42/42lib/logger"
	"github.com/ping-42/42lib/sensor"
)

func (w wsServer) schedulerListener() {
	for {
		msg, err := w.redisPubSub.ReceiveMessage()
		if err != nil {
			logger.LogError(err.Error(), "pubsub.ReceiveMessage, error receiving message", w.serverLogger)
			continue
		}

		var recevedTask sensor.Task
		err = json.Unmarshal([]byte(msg.Payload), &recevedTask)
		if err != nil {
			logger.LogError(err.Error(), fmt.Sprintf("pubsub.ReceiveMessage, error unmarshal message:%v", msg.Payload), w.serverLogger)
			continue
		}

		var serverLogger = w.serverLogger.WithFields(log.Fields{
			"sensorId": recevedTask.SensorId,
			"taskId":   recevedTask.Id,
		})

		serverLogger.Info("Receved a task submission from sensor")

		// the sensor may not be connected to this server, in this case just pass the task
		wsConn, exists := w.getSensorWsConnection(recevedTask.SensorId)
		if !exists {
			serverLogger.Info("Not interested, passing task... The sensor is not connected to this server.")
			continue
		}

		// update the task status to RECEIVED_BY_SERVER
		updateTx := w.dbClient.Model(&models.Task{}).Where("id = ?", recevedTask.Id).Update("task_status_id", 3)
		if updateTx.Error != nil {
			serverLogger.Error("Error updating task to RECEIVED_BY_SERVER", updateTx.Error)
			return
		}

		// send the received message to the sensor
		err = w.sendTaskToSensors(wsConn, []byte(msg.Payload))
		if err != nil {
			serverLogger.Error("Error sending task to sensor", err.Error())
			return
		}

		// update the task status to SENT_TO_SENSOR_BY_SERVER
		updateTx = w.dbClient.Model(&models.Task{}).Where("id = ?", recevedTask.Id).Update("task_status_id", 4)
		if updateTx.Error != nil {
			serverLogger.Error("Error updating task to SENT_TO_SENSOR_BY_SERVER", updateTx.Error)
			return
		}
	}
}
