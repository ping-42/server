package server

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/ping-42/42lib/constants"
	"github.com/ping-42/42lib/sensor"
	"github.com/ping-42/42lib/wss"
)

func (w wsServer) handleTelemtryMessage(conn wss.SensorConnection, msg []byte) (err error) {
	var time = time.Now().UTC()
	var hostTelemetryMsg sensor.HostTelemetry
	err = json.Unmarshal(msg, &hostTelemetryMsg)
	if err != nil {
		err = fmt.Errorf("Unmarshal HostTelemetry err:%v, msg:%v", err, string(msg))
		return
	}

	err = w.storeHostRuntimeStat(conn.SensorId, hostTelemetryMsg, time)
	if err != nil {
		return
	}

	err = w.storeHostNetworkStats(conn.SensorId, hostTelemetryMsg.Network, time)
	if err != nil {
		return
	}

	// Store active connection data in Redis with ttl
	activeSensor, err := json.Marshal(conn)
	if err != nil {
		w.serverLogger.Error("marshal RedisDataActiveSensor err:", err.Error())
		return
	}
	err = w.redisClient.Set(
		constants.RedisActiveSensorsKeyPrefix+conn.SensorId.String(),
		activeSensor,
		constants.TelemetryMonitorPeriod+constants.TelemetryMonitorPeriodThreshold).Err()
	if err != nil {
		err = fmt.Errorf("failed to store active connection data in Redis:%v", err)
		return
	}

	return
}
