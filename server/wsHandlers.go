package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/ping-42/42lib/constants"
	"github.com/ping-42/42lib/db/models"
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

	// -------------- task results -----------
	dnsResult := models.TsDnsResult{
		TsSensorTaskBase: models.TsSensorTaskBase{
			Time:     time.Now().UTC(),
			SensorID: sensorID,
			TaskID:   sensorResult.TaskId,
		},
		QueryRtt:  dnsRes.QueryRtt.Milliseconds(),
		SocketRtt: dnsRes.SockRtt.Milliseconds(),
		RespSize:  dnsRes.RespSize,
		Proto:     constants.ProtoTCP, // TODO: fix in the DNS, now hardcoded
		// IPAddresses: dnsRes.GetIpSlice(), // TODO need to see how to store the IPs
	}
	err = gormClient.Create(&dnsResult).Error
	if err != nil {
		return fmt.Errorf("failed to insert dns result: %v", err)
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
		return fmt.Errorf("Unmarshal dns.Result{} err:%v", err)
	}

	// in case we have also DNS test
	if icmpRes.DnsResult.Proto != "" {

		// -------------- store DNS task results -----------
		dnsResult := models.TsDnsResult{
			TsSensorTaskBase: models.TsSensorTaskBase{
				Time:     time.Now().UTC(),
				SensorID: sensorID,
				TaskID:   sensorResult.TaskId,
			},
			QueryRtt:  icmpRes.DnsResult.QueryRtt.Milliseconds(),
			SocketRtt: icmpRes.DnsResult.SockRtt.Milliseconds(),
			RespSize:  icmpRes.DnsResult.RespSize,
			Proto:     constants.ProtoTCP, // TODO: fix in the DNS, now hardcoded
			// IPAddresses: dnsRes.GetIpSlice(), // TODO need to see how to store the IPs
		}
		err = gormClient.Create(&dnsResult).Error
		if err != nil {
			return fmt.Errorf("failed to insert dns result triggerd by icmp task: %v", err)
		}
	}

	// TODO uncomment once we have the 42lib build
	// for _, res := range icmpRes.ResultPerIp {
	// 	// -------------- store ICMP task results -----------
	// 	icmpResult := models.TsIcmpResult{
	// 		TsSensorTaskBase: models.TsSensorTaskBase{
	// 			Time:     time.Now().UTC(),
	// 			SensorID: sensorID,
	// 			TaskID:   sensorResult.TaskId,
	// 		},
	// 		IPAddr:          res.IPAddr,
	// 		PacketsSent:     res.PacketsSent,
	// 		PacketsReceived: res.PacketsReceived,
	// 		BytesWritten:    res.BytesWritten,
	// 		BytesRead:       res.BytesRead,
	// 		TotalRTT:        res.TotalRTT,
	// 		MinRTT:          res.MinRTT,
	// 		MaxRTT:          res.MaxRTT,
	// 		AverageRTT:      res.AverageRTT,
	// 		Loss:            res.Loss,
	// 		FailureMessages: strings.Join(res.FailureMessages, ";"),
	// 	}
	// 	err = gormClient.Create(&icmpResult).Error
	// 	if err != nil {
	// 		return fmt.Errorf("failed to insert dns result: %v", err)
	// 	}
	// }

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

	// -------------- task results -----------
	httpResult := models.TsHttpResult{
		TsSensorTaskBase: models.TsSensorTaskBase{
			Time:     time.Now().UTC(),
			SensorID: sensorID,
			TaskID:   sensorResult.TaskId,
		},
		ResponseCode:     uint8(httpRes.ResponseCode),
		DNSLookup:        httpRes.DNSLookup,
		TCPConnection:    httpRes.TCPConnection,
		TLSHandshake:     httpRes.TLSHandshake,
		ServerProcessing: httpRes.ServerProcessing,
		NameLookup:       httpRes.NameLookup,
		Connect:          httpRes.Connect,
		Pretransfer:      httpRes.Pretransfer,
		StartTransfer:    httpRes.StartTransfer,
		//
		ResponseBody:    httpRes.ResponseBody,
		ResponseHeaders: headersJson,
	}
	err = gormClient.Create(&httpResult).Error
	if err != nil {
		return fmt.Errorf("failed to insert dns result: %v", err)
	}

	err = storeHostRuntimeStat(sensorID, sensorResult.TaskId, sensorResult.HostTelemetry)
	if err != nil {
		return
	}

	serverLogger.Info("HTTP result saved successfully for task id:", sensorResult.TaskId)
	return
}

func storeHostRuntimeStat(sensorID uuid.UUID, taskId uuid.UUID, ht sensorTask.HostTelemetry) (err error) {
	// -------------- host telemetry -----------
	runtimeStats := models.TsHostRuntimeStat{
		TsSensorTaskBase: models.TsSensorTaskBase{
			Time:     time.Now().UTC(),
			SensorID: sensorID,
			TaskID:   taskId,
		},
		GoRoutineCount: ht.GoRoutines,
		CpuCores:       ht.Cpu.Cores,
		CpuUsage:       ht.Cpu.CpuUsage,
		CpuModelName:   ht.Cpu.ModelName,
		MemTotal:       ht.Memory.Total,
		MemUsed:        ht.Memory.Used,
		MemFree:        ht.Memory.Free,
		MemUsedPercent: ht.Memory.UsedPercent,
		// TODO: Think how to handle network telemetry. Maybe it should be in a separate hypertable? Skipped for now.
	}

	err = gormClient.Create(&runtimeStats).Error
	if err != nil {
		return fmt.Errorf("failed to insert runtime stats: %v", err)
	}
	return
}
