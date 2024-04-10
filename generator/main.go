package main

// import (
// 	"bytes"
// 	"encoding/json"
// 	"io"
// 	"log"
// 	"math/rand" // #nosec
// 	"net/http"
// 	"time"

// 	"cloud.google.com/go/civil"

// 	"github.com/ping-42/server/structs"
// )

// const (
// 	IngestEventUrl = "https://us-central1-ping42-391114.cloudfunctions.net/dsn-carrier"
// )

// // main this is example code that pushes a fake event to the ingestor
// func main() {
// 	log.Println("Ping42 Generator - main() main called")

// 	eventLoop()
// }

// func eventLoop() {
// 	var newEvent structs.Ping42Event

// 	/* #nosec */
// 	r := rand.New(rand.NewSource(time.Now().UnixNano()))

// 	newEvent.DSN = "4502d487-f66a-41fb-83f0-1d2f24438d95"
// 	newEvent.EventType = 0
// 	newEvent.EventID = r.Int63()
// 	newEvent.GeneratedAt = civil.DateTimeOf(time.Now())

// 	newIcmpEvent := buildIcmpEvent(&newEvent, r)

// 	b, err := json.Marshal(newIcmpEvent)
// 	if err != nil {
// 		log.Printf("Unable to marshall Fake HTTP Request event payload: %v", err)
// 		return
// 	}

// 	jsonPayload := string(b[:])
// 	newEvent.EventPayload = &jsonPayload

// 	b, err = json.Marshal(newEvent)
// 	if err != nil {
// 		log.Printf("Unable to marshall Fake HTTP Request event payload: %v", err)
// 		return
// 	}

// 	log.Printf("Generated event: %v\n", jsonPayload)

// 	req, err := http.NewRequest("POST", IngestEventUrl, bytes.NewBuffer(b))
// 	if err != nil {
// 		log.Printf("Unable to create Fake HTTP Request: %v", err)
// 		return
// 	}

// 	req.Header.Set("Content-Type", "application/json")

// 	client := &http.Client{}
// 	resp, err := client.Do(req)

// 	if err != nil {
// 		log.Printf("Unable to send Fake HTTP Request: %v", err)
// 		return
// 	}

// 	defer resp.Body.Close()
// 	log.Println("Response Headers", resp.Header)
// 	body, _ := io.ReadAll(resp.Body)
// 	log.Println("Response", resp.Status, "| Body:", string(body))
// }

// func buildIcmpEvent(dsnEvent *structs.Ping42Event, r *rand.Rand) (event *structs.Ping42EventICMPPingInfo) {

// 	event = &structs.Ping42EventICMPPingInfo{
// 		DSN:     dsnEvent.DSN,
// 		EventID: dsnEvent.EventID,
// 		Rtt:     r.Int(),
// 		Ttl:     r.Int(),
// 		Status:  1,
// 		Success: true,
// 	}

// 	return event
// }
