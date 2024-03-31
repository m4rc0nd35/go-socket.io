package session

import (
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/m4rc0nd35/go-socket.io/engineio/frame"
	"github.com/m4rc0nd35/go-socket.io/engineio/packet"
	"github.com/m4rc0nd35/go-socket.io/engineio/payload"
	"github.com/m4rc0nd35/go-socket.io/engineio/transport"
	"github.com/m4rc0nd35/go-socket.io/logger"
)

// Pauser is connection which can be paused and resumes.
type Pauser interface {
	Pause()
	Resume()
}

type Session struct {
	conn      transport.Conn
	params    transport.ConnParameters
	transport string

	context interface{}

	readDeadline *time.Timer
	readDdlLock  sync.Mutex

	upgradeLocker sync.RWMutex
	quitChan      chan struct{}
	quitOnce      sync.Once
}

func (s *Session) Done() <-chan struct{} {
	return s.quitChan
}

func New(conn transport.Conn, sid, transport string, params transport.ConnParameters) (*Session, error) {
	params.SID = sid

	ses := &Session{
		transport: transport,
		conn:      conn,
		params:    params,
	}

	if err := ses.resetDeadlines(); err != nil {
		// If we cannot reset the deadlines, close the session
		_ = ses.Close()
		return nil, err
	}

	ses.quitChan = make(chan struct{})
	go ses.doHealthCheck()

	return ses, nil
}

func (s *Session) setPongReadline(ddl time.Time) {
	s.readDdlLock.Lock()
	defer s.readDdlLock.Unlock()
	if s.readDeadline == nil {
		// will close in the future, if not release in time
		s.readDeadline = time.AfterFunc(time.Until(ddl), func() {
			_ = s.Close()
		})
	}
}

func (s *Session) releasePongReadline() {
	s.readDdlLock.Lock()
	defer s.readDdlLock.Unlock()
	if s.readDeadline != nil {
		// stop the closing
		s.readDeadline.Stop()
		s.readDeadline = nil
	}
}

func (s *Session) SetContext(v interface{}) {
	s.context = v
}

func (s *Session) Context() interface{} {
	return s.context
}

func (s *Session) ID() string {
	return s.params.SID
}

func (s *Session) Transport() string {
	s.upgradeLocker.RLock()
	defer s.upgradeLocker.RUnlock()

	return s.transport
}

func (s *Session) Close() error {
	s.upgradeLocker.RLock()
	defer s.upgradeLocker.RUnlock()

	s.quitOnce.Do(func() {
		close(s.quitChan)
	})
	return s.conn.Close()
}

// NextReader attempts to obtain a ReadCloser from the session's connection.
// When finished writing, the caller MUST Close the ReadCloser to unlock the
// connection's FramerReader.
func (s *Session) NextReader() (FrameType, io.ReadCloser, error) {
	for {
		ft, pt, r, err := s.nextReader()
		if err != nil {
			_ = s.Close()
			return 0, nil, err
		}

		s.releasePongReadline()

		// if the packet is message type, delegate for above
		if pt == packet.MESSAGE {
			// Caller must Close the ReadCloser to unlock the connection's
			// FrameReader when finished reading.
			return FrameType(ft), r, nil
		}

		err = func() error {
			defer func() {
				// Close the current reader
				_ = r.Close()
			}()

			switch pt {
			case packet.PING:
				// Respond to a ping with a pong.
				err := func() error {
					w, err := s.nextWriter(ft, packet.PONG)
					if err != nil {
						return err
					}
					defer func() {
						_ = w.Close() // unlocks the wrapped connection's FrameWriter
					}()

					// echo the pong
					_, err = io.Copy(w, r)
					return err
				}()

				if err != nil {
					// If we cannot pong back close the connection
					_ = s.Close()
					return err
				}

			case packet.CLOSE:
				_ = s.Close()
				return io.EOF

			case packet.PONG:

			case packet.MESSAGE:

			default:
				// Unknown packet type. Close reader and try again.
			}
			return nil
		}()
		if err != nil {
			return 0, nil, err
		}
	}
}

func (s *Session) URL() url.URL {
	s.upgradeLocker.RLock()
	defer s.upgradeLocker.RUnlock()

	return s.conn.URL()
}

func (s *Session) LocalAddr() net.Addr {
	s.upgradeLocker.RLock()
	defer s.upgradeLocker.RUnlock()

	return s.conn.LocalAddr()
}

func (s *Session) RemoteAddr() net.Addr {
	s.upgradeLocker.RLock()
	defer s.upgradeLocker.RUnlock()

	return s.conn.RemoteAddr()
}

func (s *Session) RemoteHeader() http.Header {
	s.upgradeLocker.RLock()
	defer s.upgradeLocker.RUnlock()

	return s.conn.RemoteHeader()
}

// NextWriter attempts to obtain a WriteCloser from the session's connection.
// When finished writing, the caller MUST Close the WriteCloser to unlock the
// connection's FrameWriter.
func (s *Session) NextWriter(typ FrameType) (io.WriteCloser, error) {
	return s.nextWriter(frame.Type(typ), packet.MESSAGE)
}

func (s *Session) Upgrade(transport string, conn transport.Conn) {
	go s.upgrading(transport, conn)
}

func (s *Session) InitSession() error {
	w, err := s.nextWriter(frame.String, packet.OPEN)
	if err != nil {
		_ = s.Close()
		return err
	}

	if _, err := s.params.WriteTo(w); err != nil {
		_ = w.Close()
		_ = s.Close()
		return err
	}

	if err := w.Close(); err != nil {
		_ = s.Close()
		return err
	}

	return nil
}

func (s *Session) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.upgradeLocker.RLock()
	conn := s.conn
	s.upgradeLocker.RUnlock()

	if h, ok := conn.(http.Handler); ok {
		h.ServeHTTP(w, r)
	}
}

func (s *Session) doHealthCheck() {
	ll := logger.GetLogger("engineio.session.doHealthCheck")
	for {
		select {
		case <-s.quitChan:
			return
		case <-time.After(s.params.PingInterval):
		}

		s.setPongReadline(time.Now().Add(s.params.PingInterval + s.params.PingTimeout))

		s.upgradeLocker.RLock()
		conn := s.conn
		s.upgradeLocker.RUnlock()

		w, err := conn.NextWriter(frame.String, packet.PING)
		if err != nil {
			ll.Error(err, "failed to get ping writer")
			return
		}

		if err = conn.SetWriteDeadline(time.Now().Add(s.params.PingInterval + s.params.PingTimeout)); err != nil {
			ll.Error(err, "failed to set writer's deadline")
			_ = w.Close()
			_ = conn.Close()
			return
		}

		if err := w.Close(); err != nil {
			ll.Error(err, "failed to close ping writer")
			_ = conn.Close()
			return
		}
	}
}

func (s *Session) nextReader() (frame.Type, packet.Type, io.ReadCloser, error) {
	for {
		s.upgradeLocker.RLock()
		conn := s.conn
		s.upgradeLocker.RUnlock()

		// Expect the next read will come in the ping timeout
		_ = conn.SetReadDeadline(time.Now().Add(s.params.PingTimeout))

		ft, pt, r, err := conn.NextReader()
		if err != nil {
			if op, ok := err.(payload.Error); ok && op.Temporary() {
				continue
			}
			return 0, 0, nil, err
		}
		return ft, pt, r, nil
	}
}

func (s *Session) nextWriter(ft frame.Type, pt packet.Type) (io.WriteCloser, error) {
	for {
		s.upgradeLocker.RLock()
		conn := s.conn
		s.upgradeLocker.RUnlock()

		w, err := conn.NextWriter(ft, pt)
		if err != nil {
			if op, ok := err.(payload.Error); ok && op.Temporary() {
				continue
			}
			return nil, err
		}

		// Set the deadline for writing operation
		_ = conn.SetWriteDeadline(time.Now().Add(s.params.PingTimeout))

		// Caller must Close the WriteCloser to unlock the connection's
		// FrameWriter when finished writing.
		return w, nil
	}
}

func (s *Session) resetDeadlines() error {
	s.upgradeLocker.RLock()
	defer s.upgradeLocker.RUnlock()

	deadline := time.Now().Add(s.params.PingTimeout)

	err := s.conn.SetReadDeadline(deadline)
	if err != nil {
		return err
	}

	return s.conn.SetWriteDeadline(deadline)
}

func (s *Session) upgrading(t string, conn transport.Conn) {
	// Read a ping from the client.
	err := conn.SetReadDeadline(time.Now().Add(s.params.PingTimeout))
	if err != nil {
		_ = conn.Close()
		return
	}

	ft, pt, r, err := conn.NextReader()
	if err != nil {
		_ = conn.Close()
		return
	}
	if pt != packet.PING {
		_ = r.Close()
		_ = conn.Close()
		return
	}

	// Wait to close the reader until after data is read and echoed in the reply.
	// Sent a pong in reply.
	w, err := conn.NextWriter(ft, packet.PONG)
	if err != nil {
		_ = r.Close()
		_ = conn.Close()
		return
	}

	err = conn.SetWriteDeadline(time.Now().Add(s.params.PingTimeout))
	if err != nil {
		_ = w.Close()
		_ = r.Close()
		_ = conn.Close()
		return
	}

	// echo
	if _, err = io.Copy(w, r); err != nil {
		_ = w.Close()
		_ = r.Close()
		_ = conn.Close()
		return
	}
	if err = r.Close(); err != nil {
		_ = w.Close()
		_ = conn.Close()
		return
	}
	if err = w.Close(); err != nil {
		_ = conn.Close()
		return
	}

	// Pause the old connection.
	s.upgradeLocker.RLock()
	old := s.conn
	s.upgradeLocker.RUnlock()

	p, ok := old.(Pauser)
	if !ok {
		// old transport doesn't support upgrading, close it
		_ = conn.Close()
		return
	}

	// if support upgrading, pause it
	p.Pause()

	// Prepare to resume the connection if upgrade fails.
	defer func() {
		if p != nil {
			p.Resume()
		}
	}()

	// Check for upgrade packet from the client.
	err = conn.SetReadDeadline(time.Now().Add(s.params.PingTimeout))
	if err != nil {
		_ = conn.Close()
		return
	}

	_, pt, r, err = conn.NextReader()
	if err != nil {
		_ = conn.Close()
		return
	}

	if pt != packet.UPGRADE {
		_ = r.Close()
		_ = conn.Close()
		return
	}

	if err = r.Close(); err != nil {
		_ = conn.Close()
		return
	}

	// Successful upgrade.
	s.upgradeLocker.Lock()
	s.conn = conn
	s.transport = t
	s.upgradeLocker.Unlock()

	p = nil

	_ = old.Close()
}
