package socketio

import (
	"context"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/m4rc0nd35/go-socket.io/parser"
)

// Namespace describes a communication channel that allows you to split the logic of your application
// over a single shared connection.
type Namespace interface {
	// Context of this connection. You can save one context for one
	// connection, and share it between all handlers. The handlers
	// are called in one goroutine, so no need to lock context if it
	// only accessed in one connection.
	Context() context.Context
	SetContext(ctx context.Context)

	Namespace() string
	Emit(eventName string, v ...interface{})

	Join(room string)
	Leave(room string)
	LeaveAll()
	Rooms() []string
	Refuse(err error) error
}

type namespaceConn struct {
	*conn
	broadcast Broadcaster
	pkgID     atomic.Uint64

	namespace string
	context   context.Context

	ack sync.Map
}

func newNamespaceConn(conn *conn, namespace string, broadcast Broadcaster) *namespaceConn {
	return &namespaceConn{
		conn:      conn,
		namespace: namespace,
		broadcast: broadcast,
	}
}

func (nc *namespaceConn) SetContext(ctx context.Context) {
	nc.context = ctx
}

func (nc *namespaceConn) Context() context.Context {
	return nc.context
}

func (nc *namespaceConn) Namespace() string {
	return nc.namespace
}

func (nc *namespaceConn) Join(room string) {
	nc.broadcast.Join(room, nc)
}

func (nc *namespaceConn) Leave(room string) {
	nc.broadcast.Leave(room, nc)
}

func (nc *namespaceConn) LeaveAll() {
	nc.broadcast.LeaveAll(nc)
}

func (nc *namespaceConn) Rooms() []string {
	return nc.broadcast.Rooms(nc)
}

func (nc *namespaceConn) Refuse(err error) error {
	if err == nil {
		return nil
	}
	nc.writeWithArgs(parser.Header{
		Type:      parser.Error,
		Namespace: nc.namespace,
	}, reflect.ValueOf(map[string]interface{}{
		"message": err.Error(),
		"data":    nil,
	}))
	time.AfterFunc(2*time.Second, func() {
		_ = nc.Close()
	})
	return nil
}

func (nc *namespaceConn) nextPkgID() uint64 {
	return nc.pkgID.Add(1)
}

func (nc *namespaceConn) Emit(eventName string, v ...interface{}) {
	header := parser.Header{
		Type: parser.Event,
	}

	if nc.namespace != aliasRootNamespace {
		header.Namespace = nc.namespace
	}

	// if provide an ack function, will register for callback
	if l := len(v); l > 0 {
		last := v[l-1]
		lastV := reflect.TypeOf(last)

		if lastV.Kind() == reflect.Func {
			f := newAckFunc(last)

			header.ID = nc.nextPkgID()
			header.NeedAck = true

			nc.ack.Store(header.ID, f)
			v = v[:l-1]
		}
	}

	args := make([]reflect.Value, len(v)+1)
	args[0] = reflect.ValueOf(eventName)

	for i := 1; i < len(args); i++ {
		args[i] = reflect.ValueOf(v[i-1])
	}

	nc.conn.write(header, args...)
}
