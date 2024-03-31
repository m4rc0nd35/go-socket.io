package socketio

// EachFunc typed for each callback function
type EachFunc func(Conn)

// Broadcaster is the adaptor to handle broadcasts & rooms for socket.io server API
type Broadcaster interface {
	Join(room string, connection Conn)            // Join causes the connection to join a room
	Leave(room string, connection Conn)           // Leave causes the connection to leave a room
	LeaveAll(connection Conn)                     // LeaveAll causes given connection to leave all rooms
	Clear(room string)                            // Clear causes removal of all connections from the room
	Send(room, event string, args ...interface{}) // Send will send an event with args to the room
	SendAll(event string, args ...interface{})    // SendAll will send an event with args to all the rooms
	ForEach(room string, f EachFunc)              // ForEach sends data by DataFunc, if room does not exits sends nothing
	Len(room string) int                          // Len gives number of connections in the room
	Rooms(connection Conn) []string               // Gives list of all the rooms if no connection given, else list of all the rooms the connection joined
	AllRooms() []string                           // Gives list of all the rooms the connection joined
}

// broadcast gives Join, Leave & BroadcastTO server API support to socket.io along with room management
// map of rooms where each room contains a map of connection id to connections in that room
type broadcast struct {
	*broadcastLocal
}

var _ Broadcaster = &broadcast{}

// newBroadcast creates a new broadcast adapter
func newBroadcast() *broadcast {
	return &broadcast{
		broadcastLocal: newBroadcastLocal(""),
	}
}

// Join joins the given connection to the broadcast room
func (bc *broadcast) Join(room string, conn Conn) {
	bc.join(room, conn)
}

// Leave leaves the given connection from given room (if exist)
func (bc *broadcast) Leave(room string, conn Conn) {
	bc.leave(room, conn)
}

// LeaveAll leaves the given connection from all rooms
func (bc *broadcast) LeaveAll(conn Conn) {
	bc.leaveAll(conn)
}

// Clear clears the room
func (bc *broadcast) Clear(room string) {
	bc.clear(room)
}

// Send sends given event & args to all the connections in the specified room
func (bc *broadcast) Send(room, event string, args ...interface{}) {
	bc.send(room, event, args...)
}

// SendAll sends given event & args to all the connections to all the rooms
func (bc *broadcast) SendAll(event string, args ...interface{}) {
	bc.sendAll(event, args...)
}

// ForEach sends data returned by DataFunc, if room does not exits sends nothing
func (bc *broadcast) ForEach(room string, f EachFunc) {
	bc.forEach(room, f)
}

// Len gives number of connections in the room
func (bc *broadcast) Len(room string) int {
	return bc.lenRoom(room)
}

// Rooms gives the list of all the rooms available for broadcast in case of
// no connection is given, in case of a connection is given, it gives
// list of all the rooms the connection is joined to
func (bc *broadcast) Rooms(conn Conn) []string {
	return bc.getRoomsByConn(conn)
}

// AllRooms gives list of all rooms available for broadcast
func (bc *broadcast) AllRooms() []string {
	return bc.allRooms()
}
