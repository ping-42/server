package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-redis/redis"
	ws "github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/ping-42/42lib/constants"
	"github.com/ping-42/42lib/db/models"
	"github.com/ping-42/42lib/wss"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type wsServer struct {
	dbClient    *gorm.DB
	redisClient *redis.Client
}

type RedisData struct {
	SensorId      uuid.UUID
	SensorVersion string
}

func (w wsServer) run(port string) {
	// Set up a handler function for incoming requests
	http.HandleFunc("/", w.handleIncomingClient)

	// Start listening for incoming requests
	ln, err := net.Listen("tcp", port)
	if err != nil {
		serverLogger.Error("listen error", port, err)
		return
	}

	serverLogger.Info("Listening", ln.Addr())

	// Set up a server to handle incoming clients
	var (
		s     = new(http.Server)
		serve = make(chan error, 1)
		sig   = make(chan os.Signal, 1)
	)
	signal.Notify(sig, syscall.SIGTERM)
	go func() { serve <- s.Serve(ln) }()

	// This bit is straight up from the gobwas/ws examples on handling shutdowns
	select {
	case err := <-serve:
		serverLogger.Fatal(err)
	case sig := <-sig:
		const timeout = 5 * time.Second

		serverLogger.Info(fmt.Sprintf("signal %q received; shutting down with %s timeout", sig, timeout))

		ctx, ctxCancel := context.WithTimeout(context.Background(), timeout)
		defer ctxCancel()
		if err := s.Shutdown(ctx); err != nil {
			serverLogger.Fatal(err)
		}
	}
}

// handleIncomingClient is the handler function for incoming clients
func (w wsServer) handleIncomingClient(wr http.ResponseWriter, r *http.Request) {

	connectionId := uuid.New()

	jwtToken := r.Header.Get("Authorization")
	if jwtToken == "" {
		serverLogger.WithFields(log.Fields{
			"clientAddr": r.Header.Get("X-Real-IP"),
		}).Error("No JWT Token received from client")
		http.Error(wr, "Invalid sensor token received", http.StatusBadRequest)
		return
	}

	sensorId, err := w.parseAndValidateJwtToken(jwtToken)
	if err != nil {
		serverLogger.WithFields(log.Fields{
			"clientAddr": r.Header.Get("X-Real-IP"),
		}).Error(fmt.Sprintf("Unable to parse JWT token: %v", err))
		http.Error(wr, "Invalid sensor token received", http.StatusUnauthorized)
		return
	}

	sensorVersion := r.Header.Get("SensorVersion")
	if sensorVersion == "" {
		serverLogger.WithFields(log.Fields{
			"clientAddr": r.Header.Get("X-Real-IP"),
			"sensorId":   sensorId,
		}).Info("missing sensorId in connection request")
	}

	conn, _, _, err := ws.UpgradeHTTP(r, wr)
	if err != nil {
		serverLogger.WithFields(log.Fields{
			"clientAddr": r.Header.Get("X-Real-IP"),
		}).Error("UpgradeHTTP error", err)
		http.Error(wr, "Unable to upgrade HTTP connection", http.StatusInternalServerError)
		return
	}

	defer func() {

		// Delete active sensor from redis
		err := w.redisClient.Del(constants.RedisActiveSensorsKeyPrefix + sensorId.String()).Err()
		if err != nil {
			serverLogger.Error("Error deleting Redis active sensor key: ", err)
		}

		connLock.Lock()
		delete(sensorConnections, sensorId)
		serverLogger.WithFields(log.Fields{
			"connectionId": connectionId.String(),
			"sensorId":     sensorId,
		}).Info("Deleted connection")

		connLock.Unlock()
		err = conn.Close()
		if err != nil {
			serverLogger.Error("conn.Close() err: ", err.Error())
			return
		}
	}()

	connLock.Lock()
	sensorConnections[sensorId] = sensorConnection{
		ConnectionId:  connectionId,
		Connection:    conn,
		SensorId:      sensorId,
		SensorVersion: sensorVersion,
	}
	connLock.Unlock()

	// Add active sensor from redis
	// err = w.redisClient.Set(constants.RedisActiveSensorsKeyPrefix+sensorId.String(), sensorId, constants.TelemetryMonitorPeriod+constants.TelemetryMonitorPeriodThreshold).Err()
	err = w.redisClient.Set(
		constants.RedisActiveSensorsKeyPrefix+sensorId.String(),
		RedisData{SensorId: sensorId, SensorVersion: sensorVersion},
		constants.TelemetryMonitorPeriod+constants.TelemetryMonitorPeriodThreshold,
	).Err()
	if err != nil {
		serverLogger.Error("Failed to store active connection data in Redis: ", err.Error(), sensorId)
	}

	serverLogger.WithFields(log.Fields{
		"connectionId": connectionId.String(),
		"sensorId":     sensorId,
	}).Info("Added new sensor connection")

	w.listenForMessages(sensorConnections[sensorId]) // TODO maybe in goroutine?
}

func (w wsServer) listenForMessages(conn sensorConnection) {
	for {
		msg, _, err := wsutil.ReadClientData(conn.Connection)
		if err != nil {
			if err == io.EOF {
				serverLogger.WithFields(log.Fields{
					"connectionId": conn.ConnectionId.String(),
					"sensorId":     conn.SensorId,
				}).Info("Sensor disconnected")

				break // client disconnected, break out of the loop
			}
			serverLogger.WithFields(log.Fields{
				"connectionId": conn.ConnectionId.String(),
				"sensorId":     conn.SensorId,
			}).Error(fmt.Sprintf("Read message error: %v", err))
			continue
		}

		serverLogger.WithFields(
			log.Fields{
				"sensorId":     conn.SensorId.String(),
				"connectionId": conn.ConnectionId.String(),
			}).Info(fmt.Sprintf("Received message msg: %v", string(msg)))

		// Determine message type
		var generalMessage wss.GeneralMessage
		err = json.Unmarshal(msg, &generalMessage)
		if err != nil {
			serverLogger.WithFields(log.Fields{
				"connectionId": conn.ConnectionId.String(),
				"sensorId":     conn.SensorId,
			}).Error(fmt.Sprintf("Unmarshal WssMessageType err: %v, msg: %v", err, string(msg)))
			continue
		}

		switch generalMessage.MessageGeneralType {
		case wss.MessageTypeTaskResult:

			err = w.handleTaskResultMessage(conn.SensorId, msg)
			if err != nil {
				serverLogger.WithFields(log.Fields{
					"connectionId": conn.ConnectionId.String(),
					"sensorId":     conn.SensorId,
				}).Error(fmt.Sprintf("handleSensorResultMessage err: %v, msg: %v", err, string(msg)))
				continue
			}

		case wss.MessageTypeTelemtry:

			err = w.handleTelemtryMessage(conn, msg)
			if err != nil {
				serverLogger.WithFields(log.Fields{
					"connectionId": conn.ConnectionId.String(),
					"sensorId":     conn.SensorId,
				}).Error(fmt.Sprintf("handleTelemtryMessage err: %v, msg: %v", err, string(msg)))
				continue
			}

		default:
			serverLogger.WithFields(log.Fields{
				"connectionId": conn.ConnectionId.String(),
				"sensorId":     conn.SensorId,
			}).Error(fmt.Sprintf("Unexpected wssMessageType: %v, msg: %v", generalMessage.MessageGeneralType, string(msg)))
			continue
		}
	}

}

func (w wsServer) getSensorWsConnection(sensorId uuid.UUID) (con sensorConnection, exists bool) {
	con, exists = sensorConnections[sensorId]
	return con, exists
}

func (w wsServer) sendTaskToSensors(wsConn sensorConnection, tt []byte) error {
	serverLogger.WithFields(log.Fields{
		"connectionId": wsConn.ConnectionId.String(),
		"sensorId":     wsConn.SensorId.String(),
	}).Info(fmt.Sprintf("Dispatching task: %s", string(tt)))
	err := wsutil.WriteServerMessage(wsConn.Connection, ws.OpText, tt)
	if err != nil {
		return fmt.Errorf("Error WriteServerMessage newTask to sensor: %v, %v", wsConn.ConnectionId.String(), err)
	}
	return nil
}

func (w wsServer) parseAndValidateJwtToken(jwtToken string) (sensorId uuid.UUID, err error) {

	// Parse the token without validation in order to get the sensorId
	token, _, err := new(jwt.Parser).ParseUnverified(jwtToken, jwt.MapClaims{})
	if err != nil {
		err = fmt.Errorf("ParseUnverified: %v", err)
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		err = fmt.Errorf("Failed to parse claims")
		return
	}

	// Access the NOT validated claims
	sId, ok := claims["sensorId"].(string)
	if !ok {
		err = fmt.Errorf("sensorId is not a string")
		return
	}
	sensorIdNotValidated, err := uuid.ParseBytes([]byte(sId))
	if err != nil {
		err = fmt.Errorf("sensorId claim not found or not uuid.UUID: %v", err)
		return
	}

	// select the NOT VALIDATED sensor and validate with the secret
	var sensor models.Sensor
	if err = w.dbClient.First(&sensor, "id = ?", sensorIdNotValidated).Error; err != nil {
		err = fmt.Errorf("Failed to load Sensor record, sensorIdNotValidated: %v, err: %v", sensorIdNotValidated, err)
		return
	}
	if sensor.Secret == "" {
		err = fmt.Errorf("Empty sensor.Secret, sensorIdNotValidated: %v", sensorIdNotValidated)
		return
	}

	secret := []byte(sensor.Secret)

	// Now validate the token
	token, err = jwt.Parse(jwtToken, func(token *jwt.Token) (interface{}, error) {
		// Check if the signing method is what you expect
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return secret, nil
	})

	switch {
	case token.Valid:
		sensorId = sensorIdNotValidated
		return
	case errors.Is(err, jwt.ErrTokenMalformed):
		err = fmt.Errorf("That's not even a JWT token, per sensorIdNotValidated: %v", sensorIdNotValidated)
		return
	case errors.Is(err, jwt.ErrTokenSignatureInvalid):
		err = fmt.Errorf("Invalid JWT signature, per sensorIdNotValidated: %v", sensorIdNotValidated)
		return
	case errors.Is(err, jwt.ErrTokenExpired) || errors.Is(err, jwt.ErrTokenNotValidYet):
		err = fmt.Errorf("JWT Token is either expired or not active yet, per sensorIdNotValidated: %v", sensorIdNotValidated)
		return
	default:
		err = fmt.Errorf("JWT Couldn't handle this token: %v, per sensorIdNotValidated: %v", err, sensorIdNotValidated)
		return
	}
}
