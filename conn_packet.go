package socketio

import (
	"fmt"
	"reflect"

	"github.com/m4rc0nd35/go-socket.io/parser"
)

func (c *conn) ackPacketHandler(header parser.Header) error {
	nc, ok := c.namespaceConns.Get(header.Namespace)
	if !ok {
		_ = c.decoder.DiscardLast()
		return nil
	}

	rawFunc, ok := nc.ack.LoadAndDelete(header.ID)
	if !ok {
		return nil
	}

	f, ok := rawFunc.(*funcHandler)
	if !ok {
		nc.conn.onError(nc.namespace, fmt.Errorf("incorrect data stored for header %d", header.ID))
		return nil
	}

	args, err := nc.decoder.DecodeArgs(f.argTypes)
	if err != nil {
		nc.conn.onError(nc.namespace, err)
		return nil
	}
	if _, err := f.Call(args); err != nil {
		nc.conn.onError(nc.namespace, err)
		return nil
	}

	return nil
}

func (c *conn) eventPacketHandler(event string, header parser.Header) error {
	conn, ok := c.namespaceConns.Get(header.Namespace)
	if !ok {
		_ = c.decoder.DiscardLast()
		return nil
	}

	handler, ok := c.handlers.Get(header.Namespace)
	if !ok {
		_ = c.decoder.DiscardLast()
		return nil
	}

	args, err := c.decoder.DecodeArgs(handler.getEventTypes(event))
	if err != nil {
		c.onError(header.Namespace, err)
		return errDecodeArgs
	}

	ret, err := handler.dispatchEvent(conn, event, args...)
	if err != nil {
		c.onError(header.Namespace, err)
		return errHandleDispatch
	}

	if len(ret) > 0 {
		header.Type = parser.Ack
		c.write(header, ret...)
	}

	return nil
}

func (c *conn) connectPacketHandler(header parser.Header) error {
	args, err := c.decoder.DecodeArgs(defaultHeaderType)
	if err != nil {
		c.onError(header.Namespace, err)
		return errDecodeArgs
	}

	handler, ok := c.handlers.Get(header.Namespace)
	if !ok {
		c.onError(header.Namespace, errFailedConnectNamespace)
		return errFailedConnectNamespace
	}

	conn, ok := c.namespaceConns.Get(header.Namespace)
	if !ok {
		conn = newNamespaceConn(c, header.Namespace, handler.broadcast)
		c.namespaceConns.Set(header.Namespace, conn)
		conn.Join(c.ID())
	}

	_, err = handler.dispatch(conn, header, args...)
	if err != nil {
		c.onError(header.Namespace, err)
		return errHandleDispatch
	}

	c.writeWithArgs(header, reflect.ValueOf(map[string]interface{}{
		"sid": conn.ID(),
	}))

	return nil
}

func (c *conn) disconnectPacketHandler(header parser.Header) error {
	args, err := c.decoder.DecodeArgs(disconnectHeaderType)
	if err != nil {
		c.onError(header.Namespace, err)
		return errDecodeArgs
	}

	conn, ok := c.namespaceConns.Get(header.Namespace)
	if !ok {
		_ = c.decoder.DiscardLast()
		return nil
	}

	conn.LeaveAll()

	c.namespaceConns.Delete(header.Namespace)

	handler, ok := c.handlers.Get(header.Namespace)
	if !ok {
		return nil
	}

	_, err = handler.dispatch(conn, header, args...)
	if err != nil {
		c.onError(header.Namespace, err)
		return errHandleDispatch
	}

	return nil
}

var (
	defaultHeaderType    = []reflect.Type{reflect.TypeOf(make(map[string]interface{}))}
	disconnectHeaderType = []reflect.Type{reflect.TypeOf(""), reflect.TypeOf(make(map[string]interface{}))}
)

const (
	goSocketIOConnInterface = "Conn"
)

type funcHandler struct {
	argTypes []reflect.Type
	f        reflect.Value
}

func (h *funcHandler) Call(args []reflect.Value) (ret []reflect.Value, err error) {
	defer func() {
		if r := recover(); r != nil {
			var ok bool
			err, ok = r.(error)
			if !ok {
				err = fmt.Errorf("event call error: %s", r)
			}
		}
	}()

	ret = h.f.Call(args)

	return
}

func newEventFunc(f interface{}) *funcHandler {
	fv := reflect.ValueOf(f)

	if fv.Kind() != reflect.Func {
		panic("event handler must be a func.")
	}
	ft := fv.Type()

	if ft.NumIn() < 1 || ft.In(0).Name() != goSocketIOConnInterface {
		panic("handler function should be like func(socketio.Conn, ...)")
	}

	argTypes := make([]reflect.Type, ft.NumIn()-1)
	for i := range argTypes {
		argTypes[i] = ft.In(i + 1)
	}

	if len(argTypes) == 0 {
		argTypes = nil
	}

	return &funcHandler{
		argTypes: argTypes,
		f:        fv,
	}
}

func newAckFunc(f interface{}) *funcHandler {
	fv := reflect.ValueOf(f)

	if fv.Kind() != reflect.Func {
		panic("ack callback must be a func.")
	}

	ft := fv.Type()
	argTypes := make([]reflect.Type, ft.NumIn())

	for i := range argTypes {
		argTypes[i] = ft.In(i)
	}
	if len(argTypes) == 0 {
		argTypes = nil
	}

	return &funcHandler{
		argTypes: argTypes,
		f:        fv,
	}
}
