package socketio

func newBroadcastRemote(nsp string, opts *RedisAdapterConfig) (*broadcastRemote, error) {
	rbcLocal := newBroadcastLocal(nsp)
	rbcRemote, err := newRedisBroadcastRemoteV9(nsp, opts, rbcLocal)
	if err != nil {
		return nil, err
	}

	return &broadcastRemote{
		remote: rbcRemote,
		local:  rbcLocal,
	}, nil
}

// broadcastRemote gives Join, Leave & BroadcastTO server API support to socket.io along with room management
// map of rooms where each room contains a map of connection id to connections in that room
type broadcastRemote struct {
	remote *redisBroadcastRemoteV9
	local  *broadcastLocal
}

// Join joins the given connection to the broadcastRemote room.
func (bc *broadcastRemote) Join(room string, conn Conn) {
	bc.local.join(room, conn)
}

// Leave leaves the given connection from given room (if exist)
func (bc *broadcastRemote) Leave(room string, conn Conn) {
	bc.local.leave(room, conn)
}

// LeaveAll leaves the given connection from all rooms.
func (bc *broadcastRemote) LeaveAll(conn Conn) {
	bc.local.leaveAll(conn)
}

// ForEach sends data returned by DataFunc, if room does not exit sends anything.
func (bc *broadcastRemote) ForEach(room string, f EachFunc) {
	bc.local.forEach(room, f)
}

// Rooms gives the list of all the rooms available for broadcastRemote in case of
// no connection is given, in case of a connection is given, it gives
// list of all the rooms the connection is joined to.
func (bc *broadcastRemote) Rooms(connection Conn) []string {
	if connection == nil {
		return bc.AllRooms()
	}

	return bc.local.getRoomsByConn(connection)
}

// AllRooms gives list of all rooms available for broadcastRemote.
func (bc *broadcastRemote) AllRooms() []string {
	return bc.remote.allRooms()
}

// Clear clears the room.
func (bc *broadcastRemote) Clear(room string) {
	bc.local.clear(room)
	bc.remote.clear(room)
}

// Send sends given event & args to all the connections in the specified room.
func (bc *broadcastRemote) Send(room, event string, args ...interface{}) {
	bc.local.send(room, event, args...)
	bc.remote.send(room, event, args...)
}

// SendAll sends given event & args to all the connections to all the rooms.
func (bc *broadcastRemote) SendAll(event string, args ...interface{}) {
	bc.local.sendAll(event, args...)
	bc.remote.sendAll(event, args...)
}

// Len gives number of connections in the room.
func (bc *broadcastRemote) Len(room string) int {
	return bc.remote.lenRoom(room)
}
