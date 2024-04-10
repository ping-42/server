package main

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

	ws "github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/ping-42/42lib/db/models"
	"github.com/ping-42/42lib/logger"
	"github.com/ping-42/42lib/sensorTask"
	"github.com/ping-42/server/cmd"
	log "github.com/sirupsen/logrus"
)

type wsServer struct{}

func (w wsServer) run() {
	// Set up a handler function for incoming requests
	http.HandleFunc("/", w.handleIncomingClient)

	// Start listening for incoming requests
	ln, err := net.Listen("tcp", cmd.Flags.Port)
	if err != nil {
		serverLogger.Error("listen error", cmd.Flags.Port, err)
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
	jwtToken := r.URL.Query().Get("sensor_token")

	if jwtToken == "" {
		serverLogger.Error("jwtToken reqiured!")
		return
	}

	sensorId, err := parseAndValidateJwtToken(jwtToken)
	if err != nil {
		serverLogger.Error(fmt.Sprintf("parseJwtToken err:%v", err))
		return
	}

	conn, _, _, err := ws.UpgradeHTTP(r, wr)
	if err != nil {
		serverLogger.Error("upgrade error", err)
		return
	}

	defer func() {
		connLock.Lock()
		delete(sensorConnections, sensorId)
		serverLogger.Info("deleted connection ", connectionId.String(), " sernsorID:", sensorId)
		connLock.Unlock()
		err = conn.Close()
		if err != nil {
			serverLogger.Error("conn.Close() err:", err.Error())
			return
		}
	}()

	connLock.Lock()
	sensorConnections[sensorId] = sensorConnection{
		ConnectionId: connectionId,
		Connection:   conn,
		SensorId:     sensorId,
	}
	connLock.Unlock()

	serverLogger.Info("added new connection", connectionId.String(), " sensorID:", sensorId)

	w.listenForResults(sensorConnections[sensorId])
}

// TODO implement validation & mv to
// func validateSensorToken(sensorID uuid.UUID, token string) bool {
// 	return true
// }

func (w wsServer) listenForResults(conn sensorConnection) {
	for {
		msg, _, err := wsutil.ReadClientData(conn.Connection)
		if err != nil {
			if err == io.EOF {
				serverLogger.Info(fmt.Sprintf("client disconnected, ConnectionId:%v, SensorId:%v", conn.ConnectionId.String(), conn.SensorId))
				break // client disconnected, break out of the loop
			}
			serverLogger.Error(fmt.Sprintf("read message error:%v, ConnectionId:%v", err, conn.ConnectionId.String()))
			continue
		}

		serverLogger.Info(fmt.Sprintf("received message ConnectionId:%v, %v", conn.ConnectionId.String(), string(msg)))

		// parse the base result
		var sensorResult = sensorTask.TResult{}
		err = json.Unmarshal(msg, &sensorResult)
		if err != nil {
			serverLogger.Error(fmt.Sprintf("msg Unmarshal error:%v, ConnectionId:%v", err, conn.ConnectionId.String()))
			continue
		}

		// init the logger
		serverLogger := serverLogger.WithFields(log.Fields{
			"task_name":     sensorResult.TaskName,
			"task_id":       sensorResult.TaskId,
			"connection_id": conn.ConnectionId.String(),
		})

		// Update the task status to RESULTS_RECEIVED_BY_SERVER
		updateTx := gormClient.Model(&models.Task{}).Where("id = ?", sensorResult.TaskId).Update("task_status_id", 7)
		if updateTx.Error != nil {
			logger.LogError(updateTx.Error.Error(), "error updating to RESULTS_RECEIVED_BY_SERVER", serverLogger)
			continue
		}

		// if we have error from the sernsor
		if sensorResult.Error != "" {
			logger.LogError(sensorResult.Error, "sensor error", serverLogger)
			// update the task status to ERROR
			updateTx := gormClient.Model(&models.Task{}).Where("id = ?", sensorResult.TaskId).Update("task_status_id", 9)
			if updateTx.Error != nil {
				logger.LogError(updateTx.Error.Error(), "error updating to ERROR", serverLogger)
				continue
			}
			continue
		}

		// handle & insert the result to the db
		err = handleSensorResult(sensorResult, conn.SensorId)
		if err != nil {
			logger.LogError(err.Error(), "error handleSensorResult", serverLogger)
			continue
		}

		// update the task status to DONE & increment the Client Subscription
		err = taskDone(sensorResult.TaskId)
		if err != nil {
			logger.LogError(err.Error(), "error taskDone", serverLogger)
			continue
		}
	}
}

func (w wsServer) getSensorWsConnection(sensorId uuid.UUID) (con sensorConnection, exists bool) {
	con, exists = sensorConnections[sensorId]
	return con, exists
}

func (w wsServer) sendTaskToSensors(wsConn sensorConnection, tt []byte) error {
	serverLogger.Info(fmt.Sprintf("sending task:%s to client:%v", string(tt), wsConn.ConnectionId.String()))
	err := wsutil.WriteServerMessage(wsConn.Connection, ws.OpText, tt)
	if err != nil {
		return fmt.Errorf("error WriteServerMessage newTask to client:%v, %v", wsConn.ConnectionId.String(), err)
	}
	return nil
}

func taskDone(taskId uuid.UUID) (err error) {
	// 1. Laod the task
	var task models.Task
	if err = gormClient.First(&task, "id = ?", taskId).Error; err != nil {
		err = fmt.Errorf("Failed to load Task record, TaskStatusID:%v, to DONE err:%v", taskId, err)
		return
	}

	// 2. Load the associated ClientSubscription record
	var clientSubscription models.ClientSubscription
	if err = gormClient.First(&clientSubscription, "id = ?", task.ClientSubscriptionID).Error; err != nil {
		err = fmt.Errorf("Failed to load ClientSubscription record,  TaskStatusID:%v, to DONE err:%v", taskId, err)
		return
	}

	// 3. Increment the TestsCountExecuted field by one
	clientSubscription.TestsCountExecuted++
	clientSubscription.LastExecutionCompleted = time.Now()
	if err = gormClient.Save(&clientSubscription).Error; err != nil {
		err = fmt.Errorf("Failed to update TestsCountExecuted, TaskStatusID:%v, to DONE err:%v", taskId, err)
		return
	}

	// 4. Update the task status to DONE
	task.TaskStatusID = 8
	if err = gormClient.Save(&task).Error; err != nil {
		err = fmt.Errorf("Failed to update Task status, TaskStatusID:%v, to DONE err:%v", taskId, err)
		return
	}
	return
}

func parseAndValidateJwtToken(jwtToken string) (sensorId uuid.UUID, err error) {

	// Parse the token without validation in order to get the sensorId
	token, _, err := new(jwt.Parser).ParseUnverified(jwtToken, jwt.MapClaims{})
	if err != nil {
		err = fmt.Errorf("ParseUnverified:%v", err)
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		err = fmt.Errorf("failed to parse claims")
		return
	}

	// Access the NOT validated claims
	sId, ok := claims["sensorId"].(string)
	if !ok {
		err = fmt.Errorf("sensorId is not string")
		return
	}
	sensorIdNotValidated, err := uuid.ParseBytes([]byte(sId))
	if err != nil {
		err = fmt.Errorf("sensorId claim not found or not uuid.UUID:%v", err)
		return
	}

	// select the NOT VALIDATED sensor and validate with the secret
	var sensor models.Sensor
	if err = gormClient.First(&sensor, "id = ?", sensorIdNotValidated).Error; err != nil {
		err = fmt.Errorf("Failed to load Sensor record, sensorIdNotValidated:%v, err:%v", sensorIdNotValidated, err)
		return
	}
	if sensor.Secret == "" {
		err = fmt.Errorf("empty sensor.Secret, sensorIdNotValidated:%v", sensorIdNotValidated)
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
		err = fmt.Errorf("That's not even a jwt token, per sensorIdNotValidated:%v", sensorIdNotValidated)
		return
	case errors.Is(err, jwt.ErrTokenSignatureInvalid):
		err = fmt.Errorf("Invalid jwt signature, per sensorIdNotValidated:%v", sensorIdNotValidated)
		return
	case errors.Is(err, jwt.ErrTokenExpired) || errors.Is(err, jwt.ErrTokenNotValidYet):
		err = fmt.Errorf("Jwt Token is either expired or not active yet, per sensorIdNotValidated:%v", sensorIdNotValidated)
		return
	default:
		err = fmt.Errorf("Jwt Couldn't handle this token:%v, per sensorIdNotValidated:%v", err, sensorIdNotValidated)
		return
	}
}
