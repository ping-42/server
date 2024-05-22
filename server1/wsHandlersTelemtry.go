package server

import (
	"encoding/json"
	"fmt"

	"github.com/ping-42/42lib/constants"
	"github.com/ping-42/42lib/sensor"
)

func (w wsServer) handleTelemtryMessage(conn sensorConnection, msg []byte) (err error) {

	var hostTelemetryMsg sensor.HostTelemetry
	err = json.Unmarshal(msg, &hostTelemetryMsg)
	if err != nil {
		err = fmt.Errorf("Unmarshal HostTelemetry err:%v, msg:%v", err, string(msg))
		return
	}

	err = w.storeHostRuntimeStat(conn.SensorId, hostTelemetryMsg)
	if err != nil {
		return
	}

	// Store active connection data in Redis with ttl
	// activeConnJSON, err := json.Marshal(conn)
	// if err != nil {
	// 	err = fmt.Errorf("failed to marshal active connection data:%v", err)
	// 	return
	// }

	err = w.redisClient.Set(constants.RedisActiveSensorsKeyPrefix+conn.SensorId.String(), conn.SensorId, constants.TelemetryMonitorPeriod+constants.TelemetryMonitorPeriodThreshold).Err()
	if err != nil {
		err = fmt.Errorf("failed to store active connection data in Redis:%v", err)
		return
	}

	return
}
