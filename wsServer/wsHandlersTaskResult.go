package server

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/ping-42/42lib/db/models"
	"github.com/ping-42/42lib/dns"
	"github.com/ping-42/42lib/http"
	"github.com/ping-42/42lib/icmp"
	"github.com/ping-42/42lib/logger"
	"github.com/ping-42/42lib/sensor"
	log "github.com/sirupsen/logrus"
)

func (w wsServer) handleTaskResultMessage(sensorId uuid.UUID, msg []byte) (err error) {
	// parse the base result
	var sensorResult = sensor.TResult{}
	err = json.Unmarshal(msg, &sensorResult)
	if err != nil {
		err = fmt.Errorf("Unable to unmarshall message: %v", err)
		return
	}

	// init the logger
	var serverLogger = serverLogger.WithFields(log.Fields{
		"task_name": sensorResult.TaskName,
		"task_id":   sensorResult.TaskId,
		"sensor_id": sensorId,
	})

	// Update the task status to RESULTS_RECEIVED_BY_SERVER
	updateTx := w.dbClient.Model(&models.Task{}).Where("id = ?", sensorResult.TaskId).Update("task_status_id", 7)
	if updateTx.Error != nil {
		err = fmt.Errorf("error updating to RESULTS_RECEIVED_BY_SERVER")
		logger.LogError(updateTx.Error.Error(), "error updating to RESULTS_RECEIVED_BY_SERVER", serverLogger)
		return
	}

	// if we have error from the sernsor
	if sensorResult.Error != "" {
		logger.LogError(sensorResult.Error, "sensor error", serverLogger)
		// update the task status to ERROR
		updateTx := w.dbClient.Model(&models.Task{}).Where("id = ?", sensorResult.TaskId).Update("task_status_id", 9)
		if updateTx.Error != nil {
			err = fmt.Errorf("error updating to ERROR")
			logger.LogError(updateTx.Error.Error(), "error updating to ERROR", serverLogger)
			return
		}
		return
	}

	// handle & insert the result to the db
	err = w.handleSensorResult(sensorResult, sensorId)
	if err != nil {
		return
	}

	// update the task status to DONE & increment the Client Subscription
	err = w.taskDone(sensorResult.TaskId)
	if err != nil {
		return
	}
	return
}

func (w wsServer) handleSensorResult(sensorResult sensor.TResult, sensorId uuid.UUID) (err error) {

	// based on the type parse the actual res
	switch sensorResult.TaskName {
	case dns.TaskName:

		err = w.handleDnsResult(sensorResult, sensorId)
		if err != nil {
			err = fmt.Errorf("handleDnsResult error:%v", err)
			return
		}

	case icmp.TaskName:

		err = w.handleIcmpResult(sensorResult, sensorId)
		if err != nil {
			err = fmt.Errorf("handleIcmpResult error:%v", err)
			return
		}

	case http.TaskName:

		err = w.handleHttpResult(sensorResult, sensorId)
		if err != nil {
			err = fmt.Errorf("handleHttpResult error:%v", err)
			return
		}

	default:
		err = fmt.Errorf("msg unexpected TaskName:%v, ResponseReceived:%+v", sensorResult.TaskName, sensorResult)
		return
	}
	return
}

func (w wsServer) handleDnsResult(sensorResult sensor.TResult, sensorID uuid.UUID) (err error) {

	var dnsRes = dns.Result{}
	err = json.Unmarshal(sensorResult.Result, &dnsRes)
	if err != nil {
		return fmt.Errorf("Unmarshal dns.Result{} err:%v", err)
	}

	err = w.storeDnsResults(sensorID, sensorResult.TaskId, dnsRes)
	if err != nil {
		return
	}

	serverLogger.Info("DNS result saved successfully for task id:", sensorResult.TaskId)
	return
}

func (w wsServer) handleIcmpResult(sensorResult sensor.TResult, sensorID uuid.UUID) (err error) {

	var icmpRes icmp.Result
	err = json.Unmarshal(sensorResult.Result, &icmpRes)
	if err != nil {
		return fmt.Errorf("Unmarshal icmp.Result{} err:%v", err)
	}

	// store DNS task result in case we have domain in the opts
	if icmpRes.DnsResult.Proto != 0 { // todo implement check for empty
		err = w.storeDnsResults(sensorID, sensorResult.TaskId, icmpRes.DnsResult)
		if err != nil {
			return
		}
	}

	err = w.storeIcmpResults(sensorID, sensorResult.TaskId, icmpRes)
	if err != nil {
		return
	}

	serverLogger.Info("ICMP result saved successfully for task id:", sensorResult.TaskId)

	return
}

func (w wsServer) handleHttpResult(sensorResult sensor.TResult, sensorID uuid.UUID) (err error) {

	var httpRes = http.Result{}
	err = json.Unmarshal(sensorResult.Result, &httpRes)
	if err != nil {
		return fmt.Errorf("Unmarshal http.Result{} err:%v", err)
	}

	headersJson, err := json.Marshal(httpRes.ResponseHeaders)
	if err != nil {
		err = fmt.Errorf("Marshal httpRes.ResponseHeaders err:%v", err)
		return
	}

	err = w.storeHttpResults(sensorID, sensorResult.TaskId, httpRes, headersJson)
	if err != nil {
		return
	}

	serverLogger.Info("HTTP result saved successfully for task id:", sensorResult.TaskId)
	return
}

func (w wsServer) taskDone(taskId uuid.UUID) (err error) {
	// 1. Laod the task
	var task models.Task
	if err = w.dbClient.First(&task, "id = ?", taskId).Error; err != nil {
		err = fmt.Errorf("Failed to load Task record, TaskStatusID:%v, to DONE err:%v", taskId, err)
		return
	}

	// 2. Load the associated ClientSubscription record
	var clientSubscription models.ClientSubscription
	if err = w.dbClient.First(&clientSubscription, "id = ?", task.ClientSubscriptionID).Error; err != nil {
		err = fmt.Errorf("Failed to load ClientSubscription record,  TaskStatusID:%v, to DONE err:%v", taskId, err)
		return
	}

	// 3. Increment the TestsCountExecuted field by one
	clientSubscription.TestsCountExecuted++
	clientSubscription.LastExecutionCompleted = time.Now()
	if err = w.dbClient.Save(&clientSubscription).Error; err != nil {
		err = fmt.Errorf("Failed to update TestsCountExecuted, TaskStatusID:%v, to DONE err:%v", taskId, err)
		return
	}

	// 4. Update the task status to DONE
	task.TaskStatusID = 8
	if err = w.dbClient.Save(&task).Error; err != nil {
		err = fmt.Errorf("Failed to update Task status, TaskStatusID:%v, to DONE err:%v", taskId, err)
		return
	}
	return
}
