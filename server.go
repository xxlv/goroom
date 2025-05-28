package goroom

import (
	"context"
	"embed"
	"encoding/json"
	"log"
	"net/http"
	"strings"

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

// NewGinRouter creates a new GinRouterAdapter
func NewGinRouter(router gin.IRouter) *GinRouterAdapter {
	return &GinRouterAdapter{router: router}
}

func (a *GinRouterAdapter) HandleFunc(pattern string, handler http.HandlerFunc) Router {
	a.router.Any(pattern, func(c *gin.Context) {
		// Store the gin.Context in the request context
		ctx := context.WithValue(c.Request.Context(), "gin.Context", c)
		req := c.Request.WithContext(ctx)
		handler.ServeHTTP(c.Writer, req)
	})
	return a
}

func (a *GinRouterAdapter) StaticFS(relativePath string, fs http.FileSystem) Router {
	a.router.StaticFS(relativePath, fs)
	return a
}

func (a *GinRouterAdapter) PathPrefix(path string) Router {
	return &GinRouterAdapter{router: a.router.Group(path)}
}

func (a *GinRouterAdapter) Handler(handler http.Handler) Router {
	a.router.Any("/*filepath", gin.WrapH(handler))
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
	if prefix != "/" && prefix[len(prefix)-1] == '/' {
		prefix = prefix[:len(prefix)-1]
	}

	switch r := s.router.(type) {
	case *MuxRouterAdapter:
		r.HandleFunc(prefix+"/room/{roomID}", s.handleSSE)
		r.HandleFunc(prefix+"/send/{roomID}", s.handleSendMessage)
		r.PathPrefix(prefix).Handler(http.StripPrefix(prefix, http.FileServer(http.FS(staticFiles))))
	case *GinRouterAdapter:
		r.HandleFunc(prefix+"/room/:roomID", s.handleSSE)
		r.HandleFunc(prefix+"/send/:roomID", s.handleSendMessage)
		s.setupGinStaticRoutes(r, prefix)
	}
}

func (s *Server) CloseRoom(roomID string) {
	s.manager.CloseRoom(roomID)
}

// Start starts the server on the specified address
func (s *Server) Start(addr string) error {
	switch r := s.router.(type) {
	case *MuxRouterAdapter:
		serverRouter := mux.NewRouter()
		serverRouter.PathPrefix("/").Handler(r.router)
		return http.ListenAndServe(addr, serverRouter)
	case *GinRouterAdapter:
		if engine, ok := r.router.(*gin.Engine); ok {
			return engine.Run(addr)
		} else {
			engine := gin.New()
			engine.Use(gin.Logger(), gin.Recovery())
			s.Mount(engine, "")
			return engine.Run(addr)
		}
	default:
		return http.ListenAndServe(addr, nil)
	}
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

func (s *Server) setupGinStaticRoutes(r *GinRouterAdapter, prefix string) {
	staticPath := prefix + "/static"
	if prefix == "" {
		staticPath = "/static"
	}

	r.router.GET(staticPath, func(c *gin.Context) {
		if indexData, err := staticFiles.ReadFile("static/index.html"); err == nil {
			c.Header("Content-Type", "text/html; charset=utf-8")
			c.Data(http.StatusOK, "text/html; charset=utf-8", indexData)
			return
		}
		c.Status(http.StatusNotFound)
	})

	r.router.GET(staticPath+"/*filepath", func(c *gin.Context) {
		filepath := c.Param("filepath")
		if len(filepath) > 0 && filepath[0] == '/' {
			filepath = filepath[1:]
		}
		fullPath := "static/" + filepath
		if len(filepath) == 0 || filepath[len(filepath)-1] == '/' {
			indexPath := fullPath + "index.html"
			if indexData, err := staticFiles.ReadFile(indexPath); err == nil {
				c.Header("Content-Type", "text/html; charset=utf-8")
				c.Data(http.StatusOK, "text/html; charset=utf-8", indexData)
				return
			}
		}

		if fileData, err := staticFiles.ReadFile(fullPath); err == nil {
			contentType := getContentType(filepath)
			c.Header("Content-Type", contentType)
			c.Data(http.StatusOK, contentType, fileData)
			return
		}

		c.Status(http.StatusNotFound)
	})
}

func getContentType(filename string) string {
	if len(filename) == 0 {
		return "application/octet-stream"
	}

	switch {
	case strings.HasSuffix(filename, ".html") || strings.HasSuffix(filename, ".htm"):
		return "text/html; charset=utf-8"
	case strings.HasSuffix(filename, ".css"):
		return "text/css; charset=utf-8"
	case strings.HasSuffix(filename, ".js"):
		return "application/javascript; charset=utf-8"
	case strings.HasSuffix(filename, ".json"):
		return "application/json; charset=utf-8"
	case strings.HasSuffix(filename, ".png"):
		return "image/png"
	case strings.HasSuffix(filename, ".jpg") || strings.HasSuffix(filename, ".jpeg"):
		return "image/jpeg"
	case strings.HasSuffix(filename, ".gif"):
		return "image/gif"
	case strings.HasSuffix(filename, ".svg"):
		return "image/svg+xml"
	case strings.HasSuffix(filename, ".ico"):
		return "image/x-icon"
	default:
		return "application/octet-stream"
	}
}

func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	var roomID string
	if c, ok := r.Context().Value("gin.Context").(*gin.Context); ok {
		roomID = c.Param("roomID")
	} else {
		vars := mux.Vars(r)
		roomID = vars["roomID"]
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	s.manager.CreateRoom(roomID)
	s.manager.addClient(roomID, &w)

	<-r.Context().Done()
	s.manager.removeClient(roomID, &w)
}

func (s *Server) handleSendMessage(w http.ResponseWriter, r *http.Request) {
	var roomID string
	if c, ok := r.Context().Value("gin.Context").(*gin.Context); ok {
		roomID = c.Param("roomID")
	} else {
		vars := mux.Vars(r)
		roomID = vars["roomID"]
	}

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
