package main

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/ping-42/42lib/dns"
	"github.com/ping-42/42lib/http"
	"github.com/ping-42/42lib/icmp"
	"github.com/ping-42/42lib/sensorTask"
)

type resultHandler struct{}

func handleSensorResult(sensorResult sensorTask.TResult, sensorId uuid.UUID) (err error) {

	var rh resultHandler

	// based on the type parse the actual res
	switch sensorResult.TaskName {
	case dns.TaskName:

		err = rh.handleDnsResult(sensorResult, sensorId)
		if err != nil {
			err = fmt.Errorf("handleDnsResult error:%v", err)
			return
		}

	case icmp.TaskName:

		err = rh.handleIcmpResult(sensorResult, sensorId)
		if err != nil {
			err = fmt.Errorf("handleIcmpResult error:%v", err)
			return
		}

	case http.TaskName:

		err = rh.handleHttpResult(sensorResult, sensorId)
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

func (rh resultHandler) handleDnsResult(sensorResult sensorTask.TResult, sensorID uuid.UUID) (err error) {

	var dnsRes = dns.Result{}
	err = json.Unmarshal(sensorResult.Result, &dnsRes)
	if err != nil {
		return fmt.Errorf("Unmarshal dns.Result{} err:%v", err)
	}

	err = storeDnsResults(sensorID, sensorResult.TaskId, dnsRes)
	if err != nil {
		return
	}

	err = storeHostRuntimeStat(sensorID, sensorResult.TaskId, sensorResult.HostTelemetry)
	if err != nil {
		return
	}

	serverLogger.Info("DNS result saved successfully for task id:", sensorResult.TaskId)
	return
}

func (rh resultHandler) handleIcmpResult(sensorResult sensorTask.TResult, sensorID uuid.UUID) (err error) {

	var icmpRes icmp.Result
	err = json.Unmarshal(sensorResult.Result, &icmpRes)
	if err != nil {
		return fmt.Errorf("Unmarshal icmp.Result{} err:%v", err)
	}

	// store DNS task result in case we have domain in the opts
	if icmpRes.DnsResult.Proto != 0 { // todo implement check for empty
		err = storeDnsResults(sensorID, sensorResult.TaskId, icmpRes.DnsResult)
		if err != nil {
			return
		}
	}

	err = storeIcmpResults(sensorID, sensorResult.TaskId, icmpRes)
	if err != nil {
		return
	}

	err = storeHostRuntimeStat(sensorID, sensorResult.TaskId, sensorResult.HostTelemetry)
	if err != nil {
		return
	}

	serverLogger.Info("ICMP result saved successfully for task id:", sensorResult.TaskId)

	return
}

func (rh resultHandler) handleHttpResult(sensorResult sensorTask.TResult, sensorID uuid.UUID) (err error) {

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

	err = storeHttpResults(sensorID, sensorResult.TaskId, httpRes, headersJson)
	if err != nil {
		return
	}

	err = storeHostRuntimeStat(sensorID, sensorResult.TaskId, sensorResult.HostTelemetry)
	if err != nil {
		return
	}

	serverLogger.Info("HTTP result saved successfully for task id:", sensorResult.TaskId)
	return
}
