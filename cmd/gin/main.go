package main

import (
	"fmt"
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xxlv/goroom"
)

func main() {
	// 创建 Gin 引擎
	router := gin.Default()

	// 创建 SSE 服务器
	sseServer := goroom.NewServerWithRouter(goroom.NewGinRouter(router))
	sseServer.Mount(router, "/events")

	// 启动异步消息发送
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
				sseServer.WriteDebugf("room1", "Memory usage 78%%")
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

	// 启动服务器
	log.Println("Server starting on :8080")
	if err := router.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}
