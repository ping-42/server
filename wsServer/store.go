package server

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ping-42/42lib/db/models"
	"github.com/ping-42/42lib/dns"
	"github.com/ping-42/42lib/http"
	"github.com/ping-42/42lib/icmp"
	"github.com/ping-42/42lib/sensor"
	"github.com/ping-42/42lib/traceroute"
)

func (w wsServer) storeIcmpResults(sensorID uuid.UUID, taskID uuid.UUID, icmpRes icmp.Result) (err error) {

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
		err = w.dbClient.Create(&icmpResult).Error
		if err != nil {
			return fmt.Errorf("failed to insert dns result: %v", err)
		}
	}
	return
}

func (w wsServer) storeHttpResults(sensorID uuid.UUID, taskID uuid.UUID, httpRes http.Result, headersJson []byte) (err error) {
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
	err = w.dbClient.Create(&httpResult).Error
	if err != nil {
		return fmt.Errorf("failed to insert dns result: %v", err)
	}

	return
}

func (w wsServer) storeDnsResults(sensorID uuid.UUID, taskID uuid.UUID, dnsRes dns.Result) (err error) {
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
	err = w.dbClient.Create(&dnsResult).Error
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

		err := w.dbClient.Create(&httpResultAnswer).Error
		if err != nil {
			return fmt.Errorf("failed to insert TsDnsResultAnswer answer: %v", err)
		}
	}
	return
}

func (w wsServer) storeTracerouteResults(sensorID uuid.UUID, taskID uuid.UUID, tracerouteRes traceroute.Result) (err error) {
	// high level traceroute result
	tracerouteResult := models.TsTracerouteResult{
		TsSensorTaskBase: models.TsSensorTaskBase{
			Time:     time.Now().UTC(),
			SensorID: sensorID,
			TaskID:   taskID,
		},
		DestinationAdress: tracerouteRes.DestinationAdress,
	}

	// save the TsTracerouteResult
	err = w.dbClient.Create(&tracerouteResult).Error
	if err != nil {
		return fmt.Errorf("failed to insert TsTracerouteResult: %v", err)
	}

	// sinsert hop one by one
	for _, hop := range tracerouteRes.Hops {
		tracerouteHop := models.TsTracerouteResultHop{
			TsSensorTaskBase: models.TsSensorTaskBase{
				Time:     time.Now().UTC(),
				SensorID: sensorID,
				TaskID:   taskID,
			},
			Success:       hop.Success,
			Address:       hop.Address,
			Host:          hop.Host,
			BytesReceived: hop.BytesReceived,
			ElapsedTime:   hop.ElapsedTime,
			TTL:           hop.TTL,
			Error:         fmt.Sprint(hop.Error),
		}

		// save each hop
		err = w.dbClient.Create(&tracerouteHop).Error
		if err != nil {
			return fmt.Errorf("failed to insert TsTracerouteResultHop: %v", err)
		}
	}

	return nil
}

func (w wsServer) storeHostRuntimeStat(sensorID uuid.UUID, ht sensor.HostTelemetry, time time.Time) (err error) {
	runtimeStats := models.TsHostRuntimeStat{
		SensorID:       sensorID,
		Time:           time,
		GoRoutineCount: ht.GoRoutines,
		CpuCores:       ht.Cpu.Cores,
		CpuUsage:       ht.Cpu.CpuUsage,
		CpuModelName:   ht.Cpu.ModelName,
		MemTotal:       ht.Memory.Total,
		MemUsed:        ht.Memory.Used,
		MemFree:        ht.Memory.Free,
		MemUsedPercent: ht.Memory.UsedPercent,
	}

	err = w.dbClient.Create(&runtimeStats).Error
	if err != nil {
		return fmt.Errorf("failed to insert runtime stats: %v", err)
	}
	return
}

func (w wsServer) storeHostNetworkStats(sensorID uuid.UUID, networkTelemetry []sensor.Network, time time.Time) (err error) {
	// high level network stat result
	hostNetworkStat := models.TsHostNetworkStat{
		Time:     time,
		SensorID: sensorID,
	}

	// savehost network stat
	err = w.dbClient.Create(&hostNetworkStat).Error
	if err != nil {
		return fmt.Errorf("failed to insert TsHostNetworkStat: %v", err)
	}

	// prepare to store stats for each interface.
	var networkInterfaceStats []models.TsNetworkInterfaceStat
	for _, netStat := range networkTelemetry {
		// append network interface's stats to the list.
		networkInterfaceStats = append(networkInterfaceStats, models.TsNetworkInterfaceStat{
			NetworkStatID: hostNetworkStat.SensorID,
			InterfaceName: netStat.Name,
			BytesSent:     netStat.BytesSent,
			BytesRecv:     netStat.BytesRecv,
			PacketsSent:   netStat.PacketsSent,
			PacketsRecv:   netStat.PacketsRecv,
		})
	}

	// save collected network interface stats
	err = w.dbClient.Create(&networkInterfaceStats).Error
	if err != nil {
		return fmt.Errorf("failed to insert TsNetworkInterfaceStat: %v", err)
	}

	return nil
}
