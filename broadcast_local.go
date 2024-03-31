package socketio

func newBroadcastLocal(nsp string) *broadcastLocal {
	uid := newV4UUID()
	return &broadcastLocal{
		nsp:       nsp,
		uid:       uid,
		roomsSync: newRoomMap(),
	}
}

type broadcastLocal struct {
	nsp string
	uid string

	roomsSync *roomMap
}

func (bc *broadcastLocal) forEach(room string, f EachFunc) {
	occupants, ok := bc.getOccupants(room)
	if !ok {
		return
	}

	occupants.forEach(func(_ string, conn Conn) bool {
		f(conn)
		return true
	})
}

// getOccupants return all occupants of a room
func (bc *broadcastLocal) getOccupants(room string) (*connMap, bool) {
	return bc.roomsSync.getConnections(room)
}

func (bc *broadcastLocal) clear(room string) {
	bc.roomsSync.delete(room)
}

func (bc *broadcastLocal) join(room string, conn Conn) {
	bc.roomsSync.join(room, conn)
}

func (bc *broadcastLocal) leaveAll(conn Conn) {
	bc.roomsSync.leaveAll(conn)
}

func (bc *broadcastLocal) leave(room string, conn Conn) {
	bc.roomsSync.leave(room, conn)
}

func (bc *broadcastLocal) send(room string, event string, args ...interface{}) {
	conns, ok := bc.getOccupants(room)
	if !ok {
		return
	}
	conns.forEach(func(_ string, conn Conn) bool {
		// TODO: review this concurrent
		go conn.Emit(event, args...)
		return true
	})
}

func (bc *broadcastLocal) sendAll(event string, args ...interface{}) {
	bc.roomsSync.forEach(func(_ string, conn *connMap) bool {
		conn.forEach(func(_ string, conn Conn) bool {
			// TODO: review this concurrent
			go conn.Emit(event, args...)
			return true
		})
		return true
	})
}

func (bc *broadcastLocal) allRooms() []string {
	rooms := make([]string, 0)
	bc.roomsSync.forEach(func(room string, _ *connMap) bool {
		rooms = append(rooms, room)
		return true
	})
	return rooms
}

func (bc *broadcastLocal) lenRoom(roomID string) int {
	var res int
	bc.roomsSync.forEach(func(room string, _ *connMap) bool {
		if room == roomID {
			res++
		}
		return true
	})
	return res
}

func (bc *broadcastLocal) getRoomsByConn(connection Conn) []string {
	var rooms []string
	bc.roomsSync.forEach(func(room string, cm *connMap) bool {
		cm.forEach(func(connID string, _ Conn) bool {
			if connection.ID() == connID {
				rooms = append(rooms, room)
			}
			return true
		})
		return true
	})
	return rooms
}
