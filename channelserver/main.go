package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

var messages = make(map[string][]timestampedMessage)

type timestampedMessage struct {
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

func main() {
	http.HandleFunc("/message/new/", newMessageHandler)
	http.HandleFunc("/message/since/", sinceMessageHandler)

	log.Println("Running on 8090")
	log.Fatal(http.ListenAndServe(":8090", nil))
}

func sinceMessageHandler(rw http.ResponseWriter, req *http.Request) {
	if req.Method != "GET" {
		rw.Write([]byte("Only GET is accepted on this endpoint"))
		rw.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	req.ParseForm()

	to := req.Form.Get("to")
	if to == "" {
		rw.Write([]byte("Need a to"))
		rw.WriteHeader(http.StatusBadRequest)
		return
	}

	since := req.Form.Get("since")
	if since == "" {
		rw.Write([]byte("Need a since"))
		rw.WriteHeader(http.StatusBadRequest)
		return
	}

	sinceTime, err := time.Parse(time.RFC3339, since)
	if err != nil {
		rw.Write([]byte("Error parsing the since elem"))
		rw.WriteHeader(http.StatusBadRequest)
		return
	}

	tmessages, ok := messages[to]
	if !ok {
		rw.WriteHeader(200)
		return
	}

	resp := make([]timestampedMessage, 0)

	for _, tmessage := range tmessages {
		if tmessage.Timestamp.Before(sinceTime) || tmessage.Timestamp.Equal(sinceTime) {
			continue
		}

		resp = append(resp, tmessage)
	}

	err = json.NewEncoder(rw).Encode(resp)
	if err != nil {
		log.Println(err)
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}
	rw.WriteHeader(http.StatusOK)
}

func newMessageHandler(rw http.ResponseWriter, req *http.Request) {
	if req.Method != "PUT" {
		rw.Write([]byte("Only PUT is accepted on this endpoint"))
		rw.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	req.ParseForm()

	to := req.Form.Get("to")
	if to == "" {
		rw.Write([]byte("Need a to"))
		rw.WriteHeader(http.StatusBadRequest)
		return
	}

	message := req.Form.Get("message")
	if message == "" {
		rw.Write([]byte("Need a message"))
		rw.WriteHeader(http.StatusBadRequest)
		return
	}

	if _, ok := messages[to]; !ok {
		messages[to] = make([]timestampedMessage, 0)
	}

	tmessage := timestampedMessage{
		Message:   message,
		Timestamp: time.Now(),
	}

	messages[to] = append(messages[to], tmessage)

	err := json.NewEncoder(rw).Encode(tmessage)
	if err != nil {
		log.Println(err)
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}
	rw.WriteHeader(http.StatusOK)
}
