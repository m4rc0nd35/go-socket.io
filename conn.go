package socketio

import (
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"reflect"
	"sync"

	"github.com/m4rc0nd35/go-socket.io/engineio"
	"github.com/m4rc0nd35/go-socket.io/parser"
)

// Conn is a connection in go-socket.io
type Conn interface {
	io.Closer
	Namespace

	// ID returns session id
	ID() string
	URL() url.URL
	LocalAddr() net.Addr
	RemoteAddr() net.Addr
	RemoteHeader() http.Header
	Serve()
}

type conn struct {
	engineio.Conn
	encoder *parser.Encoder
	decoder *parser.Decoder

	writeChan chan parser.Payload
	errorChan chan error
	quitChan  chan struct{}

	closeOnce sync.Once

	handlers       *Handlers       // bound handlers
	namespaceConns *namespaceConns // specific handlers of each namespace instances
}

func NewConn(
	engineConn engineio.Conn,
	handlers *Handlers,
) *conn {
	return &conn{
		Conn:    engineConn,
		encoder: parser.NewEncoder(engineConn),
		decoder: parser.NewDecoder(engineConn),

		errorChan: make(chan error, 1),
		writeChan: make(chan parser.Payload, 1),
		quitChan:  make(chan struct{}),

		handlers:       handlers,
		namespaceConns: newNamespaceConns(),
	}
}

func (c *conn) Close() error {
	var err error

	c.closeOnce.Do(func() {
		// for each namespace, leave all rooms, and call the disconnect handler.
		c.namespaceConns.Range(func(ns string, nc *namespaceConn) {
			nc.LeaveAll()

			if nh, _ := c.handlers.Get(ns); nh != nil && nh.onDisconnect != nil {
				nh.onDisconnect(nc, clientDisconnectMsg, nil)
			}
		})
		err = c.Conn.Close()

		close(c.quitChan)
	})

	return err
}

func (c *conn) Serve() {
	go c.serveError()
	go c.serveWrite()
	go c.serveRead()
	<-c.Conn.Done()
	_ = c.Close()
}

func (c *conn) serveError() {
	for {
		select {
		case <-c.quitChan:
			return
		case err := <-c.errorChan:
			// emit the error detail back to client
			//c.writeWithArgs(parser.Header{
			//	Type: parser.Error,
			//}, reflect.ValueOf(map[string]interface{}{
			//	"message": err.Error(),
			//	"data":    nil,
			//}))

			// dispatch the error message to handler
			var errMsg *errorMessage
			if !errors.As(err, &errMsg) {
				continue
			}

			if handler := c.getNamespaceHandler(errMsg.namespace); handler != nil {
				if handler.onError != nil {
					nsConn, ok := c.namespaceConns.Get(errMsg.namespace)
					if !ok {
						continue
					}
					handler.onError(nsConn, errMsg.err)
				}
			}
		}
	}
}

func (c *conn) serveWrite() {
	for {
		select {
		case <-c.quitChan:
			return
		case pkg := <-c.writeChan:
			if len(pkg.Args) > 0 {
				if err := c.encoder.Encode(pkg.Header, pkg.Args...); err != nil {
					c.onError(pkg.Header.Namespace, err)
				}
			} else {
				if err := c.encoder.Encode(pkg.Header, pkg.Data); err != nil {
					c.onError(pkg.Header.Namespace, err)
				}
			}
		}
	}
}

func (c *conn) serveRead() {
	for {
		select {
		case <-c.quitChan:
			return
		default:
			var (
				event  string
				header parser.Header
			)

			if err := c.decoder.DecodeHeader(&header, &event); err != nil {
				c.onError(rootNamespace, err)
				return
			}

			if header.Namespace == aliasRootNamespace {
				header.Namespace = rootNamespace
			}

			var err error
			switch header.Type {
			case parser.Ack:
				err = c.ackPacketHandler(header)
			case parser.Connect:
				err = c.connectPacketHandler(header)
			case parser.Disconnect:
				err = c.disconnectPacketHandler(header)
			case parser.Event:
				err = c.eventPacketHandler(event, header)
			}

			if err != nil {
				c.onError(rootNamespace, err)
				return
			}
		}
	}
}

func (c *conn) write(header parser.Header, args ...reflect.Value) {
	data := make([]interface{}, len(args))

	for i := range data {
		data[i] = args[i].Interface()
	}

	pkg := parser.Payload{
		Header: header,
		Data:   data,
	}

	select {
	case <-c.quitChan:
		return
	case c.writeChan <- pkg:
	}
}

func (c *conn) writeWithArgs(header parser.Header, args ...reflect.Value) {
	data := make([]interface{}, len(args))

	for i := range data {
		data[i] = args[i].Interface()
	}

	pkg := parser.Payload{
		Header: header,
		Args:   data,
	}

	select {
	case <-c.quitChan:
		return
	case c.writeChan <- pkg:
	}
}

func (c *conn) onError(namespace string, err error) {
	select {
	case <-c.quitChan:
		return
	case c.errorChan <- newErrorMessage(namespace, err):
	}
}

func (c *conn) getNamespaceHandler(nsp string) *Handler {
	handler, _ := c.handlers.Get(nsp)
	return handler
}
