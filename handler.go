package socketio

import (
	"errors"
	"reflect"
	"sync"

	"github.com/m4rc0nd35/go-socket.io/parser"
)

// Handler contains all logics for working with connections
type Handler struct {
	broadcast Broadcaster

	events     map[string]*funcHandler
	eventsLock sync.RWMutex

	onConnect    OnConnectHandler
	onDisconnect OnDisconnectHandler
	onError      OnErrorHandler
}

func NewHandler(nsp string, adapterOpts *RedisAdapterConfig) *Handler {
	var broadcast Broadcaster
	if adapterOpts == nil {
		broadcast = newBroadcast()
	} else {
		broadcast, _ = newBroadcastRemote(nsp, adapterOpts)
	}

	return &Handler{
		broadcast: broadcast,
		events:    make(map[string]*funcHandler),
	}
}

func (nh *Handler) OnConnect(f OnConnectHandler) {
	nh.onConnect = f
}

func (nh *Handler) OnDisconnect(f OnDisconnectHandler) {
	nh.onDisconnect = f
}

func (nh *Handler) OnError(f OnErrorHandler) {
	nh.onError = f
}

func (nh *Handler) OnEvent(event string, f interface{}) {
	nh.eventsLock.Lock()
	defer nh.eventsLock.Unlock()

	nh.events[event] = newEventFunc(f)
}

func (nh *Handler) Join(room string, conn Conn) bool {
	if nh == nil {
		return false
	}
	nh.broadcast.Join(room, conn)
	return true
}

func (nh *Handler) Leave(room string, conn Conn) bool {
	if nh == nil {
		return false
	}
	nh.broadcast.Leave(room, conn)
	return true
}

func (nh *Handler) LeaveAll(conn Conn) bool {
	if nh == nil {
		return false
	}
	nh.broadcast.LeaveAll(conn)
	return true
}

func (nh *Handler) Clear(room string) bool {
	if nh == nil {
		return false
	}
	nh.broadcast.Clear(room)
	return true
}

func (nh *Handler) Send(room string, event string, args ...interface{}) bool {
	if nh == nil {
		return false
	}
	nh.broadcast.Send(room, event, args...)
	return true
}

func (nh *Handler) SendAll(event string, args ...interface{}) bool {
	if nh == nil {
		return false
	}
	nh.broadcast.SendAll(event, args...)
	return true
}

func (nh *Handler) Len(room string) int {
	if nh == nil {
		return -1
	}
	return nh.broadcast.Len(room)
}

func (nh *Handler) Rooms(conn Conn) []string {
	if nh == nil {
		return nil
	}
	return nh.broadcast.Rooms(conn)
}

func (nh *Handler) ForEach(room string, f EachFunc) bool {
	if nh == nil {
		return false
	}
	nh.broadcast.ForEach(room, f)
	return true
}

func (nh *Handler) getEventTypes(event string) []reflect.Type {
	nh.eventsLock.RLock()
	namespaceHandler := nh.events[event]
	nh.eventsLock.RUnlock()

	if namespaceHandler != nil {
		return namespaceHandler.argTypes
	}

	return nil
}

func (nh *Handler) dispatch(conn Conn, header parser.Header, args ...reflect.Value) ([]reflect.Value, error) {
	switch header.Type {
	case parser.Connect:
		if nh.onConnect != nil {
			return nil, nh.onConnect(conn, getDispatchData(args...))
		}
		return nil, nil

	case parser.Disconnect:
		if nh.onDisconnect != nil {
			reason, details := getDispatchDisconnectData(args...)
			nh.onDisconnect(conn, reason, details)
		}
		return nil, nil

	case parser.Error:
		if nh.onError != nil {
			msg := getDispatchMessage(args...)
			if msg == "" {
				msg = "parser error gotAck"
			}
			nh.onError(conn, errors.New(msg))
		}
	}

	return nil, parser.ErrInvalidPacketType
}

func (nh *Handler) dispatchEvent(conn Conn, event string, args ...reflect.Value) ([]reflect.Value, error) {
	nh.eventsLock.RLock()
	namespaceHandler := nh.events[event]
	nh.eventsLock.RUnlock()

	if namespaceHandler == nil {
		return nil, nil
	}

	return namespaceHandler.Call(append([]reflect.Value{reflect.ValueOf(conn)}, args...))
}

func getDispatchDisconnectData(args ...reflect.Value) (reason string, details map[string]interface{}) {
	if len(args) > 0 {
		reason = args[0].Interface().(string)
	}
	if len(args) > 1 {
		details = args[1].Interface().(map[string]interface{})
	}

	return
}

func getDispatchMessage(args ...reflect.Value) string {
	var msg string
	if len(args) > 0 {
		msg = args[0].Interface().(string)
	}

	return msg
}

func getDispatchData(args ...reflect.Value) map[string]interface{} {
	var val map[string]interface{}
	if len(args) > 0 {
		val = args[0].Interface().(map[string]interface{})
	}

	return val
}

type OnConnectHandler func(Conn, map[string]interface{}) error
type OnDisconnectHandler func(Conn, string, map[string]interface{})
type OnErrorHandler func(Conn, error)
