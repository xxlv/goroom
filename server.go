package goroom

import (
	"embed"
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

//go:embed static
var staticFiles embed.FS

// Server 封装 SSE 服务
type Server struct {
	manager *RoomManager
	router  *mux.Router
}

// NewServer 创建一个新的 SSE 服务
func NewServer() *Server {
	manager := NewRoomManager()
	router := mux.NewRouter()
	server := &Server{manager: manager, router: router}
	return server
}

func (s *Server) Mount(router *mux.Router, prefix string) {
	if prefix == "" {
		prefix = "/"
	}
	if prefix != "/" && prefix[len(prefix)-1] != '/' {
		prefix += "/"
	}
	s.setupRoutes(prefix)
	router.PathPrefix(prefix).Handler(s.router)
}

// setupRoutes 配置 SSE 和静态文件路由
func (s *Server) setupRoutes(prefix string) {
	log.Println(prefix + "room/{roomID}")
	s.router.HandleFunc(prefix+"room/{roomID}", s.handleSSE).Methods("GET")
	s.router.HandleFunc(prefix+"send/{roomID}", s.handleSendMessage).Methods("POST")
	s.router.PathPrefix(prefix).Handler(http.StripPrefix(prefix, http.FileServer(http.FS(staticFiles))))
}

// Start 启动独立 SSE 服务
func (s *Server) Start(addr string) error {
	return http.ListenAndServe(addr, s.router)
}

// WriteToRoom 向指定房间写入消息
func (s *Server) WriteToRoom(roomID, message string) bool {
	return s.manager.WriteToRoom(roomID, message)
}

// handleSSE 处理 SSE 连接
func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	roomID := vars["roomID"]

	// 设置 SSE 响应头
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// 创建或获取房间
	s.manager.CreateRoom(roomID)
	s.manager.addClient(roomID, &w)

	// 保持连接直到客户端断开
	<-r.Context().Done()
	s.manager.removeClient(roomID, &w)
}

// handleSendMessage 处理发送消息的请求
func (s *Server) handleSendMessage(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	roomID := vars["roomID"]

	var data struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if s.manager.WriteToRoom(roomID, data.Message) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Message sent to room " + roomID))
	} else {
		http.Error(w, "Room not found or channel full", http.StatusNotFound)
	}
}
