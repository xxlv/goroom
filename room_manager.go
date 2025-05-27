package goroom

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type LogLevel string

const (
	LogLevelInfo    LogLevel = "info"
	LogLevelSuccess LogLevel = "success"
	LogLevelWarning LogLevel = "warning"
	LogLevelError   LogLevel = "error"
	LogLevelDebug   LogLevel = "debug"
)

type LogMessage struct {
	Level     LogLevel  `json:"level"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

func (lm *LogMessage) ToJSON() string {
	data, _ := json.Marshal(lm)
	return string(data)
}

type RoomManager struct {
	rooms   map[string]chan string
	clients map[string]map[*http.ResponseWriter]struct{}
	mutex   sync.RWMutex
}

func NewRoomManager() *RoomManager {
	return &RoomManager{
		rooms:   make(map[string]chan string),
		clients: make(map[string]map[*http.ResponseWriter]struct{}),
	}
}

func (rm *RoomManager) CreateRoom(roomID string) chan string {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	if ch, exists := rm.rooms[roomID]; exists {
		return ch
	}

	ch := make(chan string, 100)
	rm.rooms[roomID] = ch
	rm.clients[roomID] = make(map[*http.ResponseWriter]struct{})
	go rm.broadcast(roomID, ch)
	return ch
}

func (rm *RoomManager) WriteToRoomWithLevel(roomID, message string, level LogLevel) bool {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()

	if ch, exists := rm.rooms[roomID]; exists {
		logMsg := &LogMessage{
			Level:     level,
			Message:   message,
			Timestamp: time.Now(),
		}

		select {
		case ch <- logMsg.ToJSON():
			return true
		default:
			return false
		}
	}
	return false
}

func (rm *RoomManager) WriteToRoom(roomID, message string) bool {
	return rm.WriteToRoomWithLevel(roomID, message, LogLevelInfo)
}
func (rm *RoomManager) WriteFormattedToRoom(roomID string, level LogLevel, format string, args ...interface{}) bool {
	message := fmt.Sprintf(format, args...)
	return rm.WriteToRoomWithLevel(roomID, message, level)
}

func (rm *RoomManager) WriteInfof(roomID, format string, args ...interface{}) bool {
	return rm.WriteFormattedToRoom(roomID, LogLevelInfo, format, args...)
}

func (rm *RoomManager) WriteSuccessf(roomID, format string, args ...interface{}) bool {
	return rm.WriteFormattedToRoom(roomID, LogLevelSuccess, format, args...)
}

func (rm *RoomManager) WriteWarningf(roomID, format string, args ...interface{}) bool {
	return rm.WriteFormattedToRoom(roomID, LogLevelWarning, format, args...)
}

func (rm *RoomManager) WriteErrorf(roomID, format string, args ...interface{}) bool {
	return rm.WriteFormattedToRoom(roomID, LogLevelError, format, args...)
}

func (rm *RoomManager) WriteDebugf(roomID, format string, args ...interface{}) bool {
	return rm.WriteFormattedToRoom(roomID, LogLevelDebug, format, args...)
}

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

func (rm *RoomManager) addClient(roomID string, w *http.ResponseWriter) {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()
	if _, exists := rm.clients[roomID]; !exists {
		rm.clients[roomID] = make(map[*http.ResponseWriter]struct{})
	}
	rm.clients[roomID][w] = struct{}{}
}

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

func (rm *RoomManager) CloseRoom(roomID string) {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	if ch, exists := rm.rooms[roomID]; exists {
		close(ch)
		delete(rm.rooms, roomID)
	}
	delete(rm.clients, roomID)
}
