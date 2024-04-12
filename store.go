package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ping-42/42lib/db/models"
	"github.com/ping-42/42lib/dns"
	"github.com/ping-42/42lib/http"
	"github.com/ping-42/42lib/icmp"
	"github.com/ping-42/42lib/sensorTask"
)

func storeIcmpResults(sensorID uuid.UUID, taskID uuid.UUID, icmpRes icmp.Result) (err error) {

	for _, res := range icmpRes.ResultPerIp {
		icmpResult := models.TsIcmpResult{
			TsSensorTaskBase: models.TsSensorTaskBase{
				Time:     time.Now().UTC(),
				SensorID: sensorID,
				TaskID:   taskID,
			},
			IPAddr:          res.IPAddr,
			PacketsSent:     res.PacketsSent,
			PacketsReceived: res.PacketsReceived,
			BytesWritten:    res.BytesWritten,
			BytesRead:       res.BytesRead,
			TotalRTT:        res.TotalRTT,
			MinRTT:          res.MinRTT,
			MaxRTT:          res.MaxRTT,
			AverageRTT:      res.AverageRTT,
			Loss:            res.Loss,
			FailureMessages: strings.Join(res.FailureMessages, ";"),
		}
		err = gormClient.Create(&icmpResult).Error
		if err != nil {
			return fmt.Errorf("failed to insert dns result: %v", err)
		}
	}
	return
}

func storeHttpResults(sensorID uuid.UUID, taskID uuid.UUID, httpRes http.Result, headersJson []byte) (err error) {
	httpResult := models.TsHttpResult{
		TsSensorTaskBase: models.TsSensorTaskBase{
			Time:     time.Now().UTC(),
			SensorID: sensorID,
			TaskID:   taskID,
		},
		ResponseCode:     httpRes.ResponseCode,
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

	return
}

func storeDnsResults(sensorID uuid.UUID, taskID uuid.UUID, dnsRes dns.Result) (err error) {
	taskBase := models.TsSensorTaskBase{
		Time:     time.Now().UTC(),
		SensorID: sensorID,
		TaskID:   taskID,
	}

	dnsResult := models.TsDnsResult{
		TsSensorTaskBase: taskBase,
		QueryRtt:         dnsRes.QueryRtt.Milliseconds(),
		SocketRtt:        dnsRes.SockRtt.Milliseconds(),
		RespSize:         dnsRes.RespSize,
		Proto:            dnsRes.Proto,
	}
	err = gormClient.Create(&dnsResult).Error
	if err != nil {
		return fmt.Errorf("failed to insert TsDnsResult result: %v", err)
	}

	// store AnswerA per DNS
	for _, answer := range dnsRes.AnswerA {
		httpResultAnswer := models.TsDnsResultAnswer{
			TsSensorTaskBase: taskBase,
			HdrName:          answer.Hdr.Name,
			HdrRrtype:        answer.Hdr.Rrtype,
			HdrClass:         answer.Hdr.Class,
			HdrTtl:           answer.Hdr.Ttl,
			HdrRdlength:      answer.Hdr.Rrtype,
			A:                answer.A,
		}

		err := gormClient.Create(&httpResultAnswer).Error
		if err != nil {
			return fmt.Errorf("failed to insert TsDnsResultAnswer answer: %v", err)
		}
	}
	return
}

func storeHostRuntimeStat(sensorID uuid.UUID, taskId uuid.UUID, ht sensorTask.HostTelemetry) (err error) {
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
