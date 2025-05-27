package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/xxlv/goroom"
)

func main() {
	router := mux.NewRouter()
	sseServer := goroom.NewServer()
	sseServer.Mount(router, "/events")

	go func() {
		ticker := time.NewTicker(1 * time.Second / 10)
		defer ticker.Stop()
		for i := 1; ; i++ {
			select {
			case <-ticker.C:
				sseServer.WriteInfof("room1", "Start async task...")
				sseServer.WriteSuccessf("room1", "Database connection established")
				sseServer.WriteWarningf("room1", "Task execution time is long, please wait")
				sseServer.WriteErrorf("room1", "Network connection timeout, reconnecting...")
				sseServer.WriteDebugf("room1", "Memory usage 78%")
				sseServer.WriteInfof("room1", "Processing task %d/%d", 1, 5)
				sseServer.WriteSuccessf("room1", "Task completed, time: %.2f seconds", 12.5)
				sseServer.WriteErrorf("room1", "Task failed, error code: %d", 500)
				message := time.Now().Format("2006-01-02 15:04:05") + " - Message " + fmt.Sprintf("%d", i)
				if sseServer.WriteToRoom("room1", message) {
					log.Printf("Sent to room1: %s", message)
				} else {
					log.Println("Failed to send message to room1")
				}
			}
		}
	}()

	log.Println("Server starting on :8080")
	if err := http.ListenAndServe(":8080", router); err != nil {
		log.Fatal(err)
	}
}
