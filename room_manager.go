package goroom

import (
	"fmt"
	"net/http"
	"sync"
)

// RoomManager 管理房间和客户端连接
type RoomManager struct {
	rooms   map[string]chan string
	clients map[string]map[*http.ResponseWriter]struct{}
	mutex   sync.RWMutex
}

// NewRoomManager 创建一个新的 RoomManager
func NewRoomManager() *RoomManager {
	return &RoomManager{
		rooms:   make(map[string]chan string),
		clients: make(map[string]map[*http.ResponseWriter]struct{}),
	}
}

// CreateRoom 创建或获取一个房间的消息通道
func (rm *RoomManager) CreateRoom(roomID string) chan string {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	if ch, exists := rm.rooms[roomID]; exists {
		return ch
	}

	ch := make(chan string, 100) // 带缓冲的通道
	rm.rooms[roomID] = ch
	rm.clients[roomID] = make(map[*http.ResponseWriter]struct{})
	go rm.broadcast(roomID, ch)
	return ch
}

// WriteToRoom 向指定房间写入消息
func (rm *RoomManager) WriteToRoom(roomID, message string) bool {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()
	if ch, exists := rm.rooms[roomID]; exists {
		select {
		case ch <- message:
			return true
		default:
			return false // 通道已满
		}
	}
	return false // 房间不存在
}

// broadcast 广播消息到房间内的所有客户端
func (rm *RoomManager) broadcast(roomID string, ch chan string) {
	for msg := range ch {
		rm.mutex.RLock()
		for client := range rm.clients[roomID] {
			fmt.Fprintf(*client, "data: %s\n\n", msg)
			if f, ok := (*client).(http.Flusher); ok {
				f.Flush()
			}
		}
		rm.mutex.RUnlock()
	}
}

// addClient 添加客户端到房间
func (rm *RoomManager) addClient(roomID string, w *http.ResponseWriter) {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()
	if _, exists := rm.clients[roomID]; !exists {
		rm.clients[roomID] = make(map[*http.ResponseWriter]struct{})
	}
	rm.clients[roomID][w] = struct{}{}
}

// removeClient 从房间移除客户端
func (rm *RoomManager) removeClient(roomID string, w *http.ResponseWriter) {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()
	if clients, exists := rm.clients[roomID]; exists {
		delete(clients, w)
		if len(clients) == 0 {
			delete(rm.clients, roomID)
			if ch, exists := rm.rooms[roomID]; exists {
				close(ch)
				delete(rm.rooms, roomID)
			}
		}
	}
}
