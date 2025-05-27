package goroom

import (
	"embed"
	"encoding/json"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/mux"
)

//go:embed static
var staticFiles embed.FS

// Router defines the interface for HTTP routers
type Router interface {
	HandleFunc(pattern string, handler http.HandlerFunc) Router
	PathPrefix(path string) Router
	Handler(handler http.Handler) Router
}

// MuxRouterAdapter adapts mux.Router to our Router interface
type MuxRouterAdapter struct {
	router *mux.Router
}

func (a *MuxRouterAdapter) HandleFunc(pattern string, handler http.HandlerFunc) Router {
	a.router.HandleFunc(pattern, handler)
	return a
}

func (a *MuxRouterAdapter) PathPrefix(path string) Router {
	return &MuxRouterAdapter{router: a.router.PathPrefix(path).Subrouter()}
}

func (a *MuxRouterAdapter) Handler(handler http.Handler) Router {
	a.router.PathPrefix("/").Handler(handler)
	return a
}

// GinRouterAdapter adapts gin.IRouter to our Router interface
type GinRouterAdapter struct {
	router gin.IRouter
}

func (a *GinRouterAdapter) HandleFunc(pattern string, handler http.HandlerFunc) Router {
	a.router.Handle(http.MethodGet, pattern, gin.WrapF(handler))
	return a
}

func (a *GinRouterAdapter) PathPrefix(path string) Router {
	return &GinRouterAdapter{router: a.router.Group(path)}
}

func (a *GinRouterAdapter) Handler(handler http.Handler) Router {
	a.router.StaticFS("/static", http.FS(staticFiles))
	return a
}

type Server struct {
	manager *RoomManager
	router  Router
}

func NewServer() *Server {
	manager := NewRoomManager()
	router := &MuxRouterAdapter{router: mux.NewRouter()}
	server := &Server{manager: manager, router: router}
	return server
}

// NewServerWithRouter creates a new server with a custom router
func NewServerWithRouter(router Router) *Server {
	manager := NewRoomManager()
	server := &Server{manager: manager, router: router}
	return server
}

// Mount mounts the server to a router with a prefix
func (s *Server) Mount(router interface{}, prefix string) {
	if prefix == "" {
		prefix = "/"
	}
	if prefix != "/" && prefix[len(prefix)-1] != '/' {
		prefix += "/"
	}

	var r Router
	switch v := router.(type) {
	case *mux.Router:
		r = &MuxRouterAdapter{router: v}
	case gin.IRouter:
		r = &GinRouterAdapter{router: v}
	default:
		log.Fatal("Unsupported router type")
	}

	s.router = r
	s.setupRoutes(prefix)
}

func (s *Server) setupRoutes(prefix string) {
	log.Println(prefix + "room/{roomID}")
	s.router.HandleFunc(prefix+"room/{roomID}", s.handleSSE)
	s.router.HandleFunc(prefix+"send/{roomID}", s.handleSendMessage)
	s.router.PathPrefix(prefix).Handler(http.StripPrefix(prefix, http.FileServer(http.FS(staticFiles))))
}

// Start starts the server on the specified address
func (s *Server) Start(addr string) error {
	// Create a new mux router for the server
	serverRouter := mux.NewRouter()

	// Mount our router to the server router
	serverRouter.PathPrefix("/").Handler(s.router.(*MuxRouterAdapter).router)

	return http.ListenAndServe(addr, serverRouter)
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
