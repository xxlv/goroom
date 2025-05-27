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

type Server struct {
	manager *RoomManager
	router  *mux.Router
}

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

func (s *Server) setupRoutes(prefix string) {
	//prefix is  /events/
	log.Println(prefix + "room/{roomID}")
	s.router.HandleFunc(prefix+"room/{roomID}", s.handleSSE).Methods("GET")
	s.router.HandleFunc(prefix+"send/{roomID}", s.handleSendMessage).Methods("POST")
	s.router.PathPrefix(prefix).Handler(http.StripPrefix(prefix, http.FileServer(http.FS(staticFiles))))
}

func (s *Server) Start(addr string) error {
	return http.ListenAndServe(addr, s.router)
}

func (s *Server) WriteToRoom(roomID, message string) bool {
	return s.manager.WriteToRoom(roomID, message)
}

func (s *Server) WriteInfof(roomID, format string, args ...interface{}) bool {
	return s.manager.WriteInfof(roomID, format, args...)
}
func (s *Server) WriteSuccessf(roomID, format string, args ...interface{}) bool {
	return s.manager.WriteSuccessf(roomID, format, args...)
}

func (s *Server) WriteWarningf(roomID, format string, args ...interface{}) bool {
	return s.manager.WriteWarningf(roomID, format, args...)
}
func (s *Server) WriteErrorf(roomID, format string, args ...interface{}) bool {
	return s.manager.WriteErrorf(roomID, format, args...)
}

func (s *Server) WriteDebugf(roomID, format string, args ...interface{}) bool {
	return s.manager.WriteDebugf(roomID, format, args...)
}

func (s *Server) CloseRoom(roomID string) {
	s.manager.CloseRoom(roomID)
}

func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	roomID := vars["roomID"]

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	s.manager.CreateRoom(roomID)
	s.manager.addClient(roomID, &w)

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
		_, _ = w.Write([]byte("Message sent to room " + roomID))
	} else {
		http.Error(w, "Room not found or channel full", http.StatusNotFound)
	}
}
