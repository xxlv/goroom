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
	// 创建主路由器
	router := mux.NewRouter()

	// 创建 SSE 服务并挂载到 /events 子路径
	sseServer := goroom.NewServer()
	sseServer.Mount(router, "/events")

	// 定时向 room1 发送消息
	go func() {
		ticker := time.NewTicker(1 * time.Second / 10)
		defer ticker.Stop()
		for i := 1; ; i++ {
			select {
			case <-ticker.C:
				message := time.Now().Format("2006-01-02 15:04:05") + " - Message " + fmt.Sprintf("%d", i)
				if sseServer.WriteToRoom("room1", message) {
					log.Printf("Sent to room1: %s", message)
				} else {
					log.Println("Failed to send message to room1")
				}
			}
		}
	}()

	// 启动服务
	log.Println("Server starting on :8080")
	if err := http.ListenAndServe(":8080", router); err != nil {
		log.Fatal(err)
	}
}
