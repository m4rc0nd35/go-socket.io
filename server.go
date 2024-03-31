package socketio

import (
	"net/http"

	"github.com/m4rc0nd35/go-socket.io/engineio"
)

// Server is a go-socket.io server.
type Server struct {
	engine *engineio.Server

	nspHandlers  *Handlers
	redisAdapter *RedisAdapterConfig
}

// NewServer returns a server.
func NewServer(opts *engineio.Options) *Server {
	return &Server{
		nspHandlers: NewHandlers(),
		engine:      engineio.NewServer(opts),
	}
}

// Adapter sets redis broadcast adapter.
func (s *Server) Adapter(opts *RedisAdapterConfig) (bool, error) {
	s.redisAdapter = GetOptions(opts)

	return true, nil
}

// Close closes server.
func (s *Server) Close() error {
	return s.engine.Close()
}

// ServeHTTP dispatches the request to the handler whose pattern most closely matches the request URL.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.engine.ServeHTTP(w, r)
}

// OnConnect set a handler function f to handle open event for
func (s *Server) OnConnect(namespace string, f OnConnectHandler) {
	h := s.getOrCreateNamespaceHandler(namespace)
	h.OnConnect(f)
}

// OnDisconnect set a handler function f to handle disconnect event for
func (s *Server) OnDisconnect(namespace string, f OnDisconnectHandler) {
	h := s.getOrCreateNamespaceHandler(namespace)
	h.OnDisconnect(f)
}

// OnError set a handler function f to handle error for
func (s *Server) OnError(namespace string, f OnErrorHandler) {
	h := s.getOrCreateNamespaceHandler(namespace)
	h.OnError(f)
}

// OnEvent set a handler function f to handle event for
func (s *Server) OnEvent(namespace string, event string, f interface{}) {
	h := s.getOrCreateNamespaceHandler(namespace)
	h.OnEvent(event, f)
}

// Serve serves go-socket.io server.
func (s *Server) Serve() error {
	for {
		conn, err := s.engine.Accept()
		//todo maybe need check EOF from Accept()
		if err != nil {
			return err
		}

		go func(conn engineio.Conn) {
			defer func() {
				s.engine.Remove(conn.ID())
			}()
			c := NewConn(conn, s.nspHandlers)
			c.Serve()
		}(conn)
	}
}

// JoinRoom joins given connection to the room.
func (s *Server) JoinRoom(namespace string, room string, conn Conn) bool {
	nspHandler := s.getNamespaceHandler(namespace)
	return nspHandler.Join(room, conn)
}

// LeaveRoom leaves given connection from the room.
func (s *Server) LeaveRoom(namespace string, room string, conn Conn) bool {
	nspHandler := s.getNamespaceHandler(namespace)
	return nspHandler.Leave(room, conn)
}

// LeaveAllRooms leaves the given connection from all rooms.
func (s *Server) LeaveAllRooms(namespace string, conn Conn) bool {
	nspHandler := s.getNamespaceHandler(namespace)
	return nspHandler.LeaveAll(conn)
}

// ClearRoom clears the room.
func (s *Server) ClearRoom(namespace string, room string) bool {
	nspHandler := s.getNamespaceHandler(namespace)
	return nspHandler.Clear(room)
}

// BroadcastToRoom broadcasts given event & args to all the connections in the room.
func (s *Server) BroadcastToRoom(namespace string, room, event string, args ...interface{}) bool {
	nspHandler := s.getNamespaceHandler(namespace)
	return nspHandler.Send(room, event, args...)
}

// BroadcastToNamespace broadcasts given event & args to all the connections in the same
func (s *Server) BroadcastToNamespace(namespace string, event string, args ...interface{}) bool {
	nspHandler := s.getNamespaceHandler(namespace)
	return nspHandler.SendAll(event, args...)
}

// RoomLen gives number of connections in the room.
func (s *Server) RoomLen(namespace string, room string) int {
	nspHandler := s.getNamespaceHandler(namespace)
	return nspHandler.Len(room)
}

// Rooms gives list of all the rooms.
func (s *Server) Rooms(namespace string) []string {
	nspHandler := s.getNamespaceHandler(namespace)
	return nspHandler.Rooms(nil)
}

// ForEach sends data by DataFunc, if room does not exit sends anything.
func (s *Server) ForEach(namespace string, room string, f EachFunc) bool {
	nspHandler := s.getNamespaceHandler(namespace)
	return nspHandler.ForEach(room, f)
}

// Count number of connections.
func (s *Server) Count() int {
	return s.engine.Count()
}

func (s *Server) getOrCreateNamespaceHandler(nsp string) *Handler {
	h := s.getNamespaceHandler(nsp)
	if h == nil {
		h = s.createNamespaceHandler(nsp)
	}

	return h
}

func (s *Server) createNamespaceHandler(nsp string) *Handler {
	if nsp == aliasRootNamespace {
		nsp = rootNamespace
	}

	handler := NewHandler(nsp, s.redisAdapter)
	s.nspHandlers.Set(nsp, handler)

	return handler
}

func (s *Server) getNamespaceHandler(nsp string) *Handler {
	if nsp == aliasRootNamespace {
		nsp = rootNamespace
	}

	ret, ok := s.nspHandlers.Get(nsp)
	if !ok {
		return nil
	}

	return ret
}
