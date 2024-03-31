package socketio

import (
	"sync"
)

// roomMap as sync.Map

func newRoomMap() *roomMap {
	return &roomMap{data: make(map[string]*connMap)}
}

type roomMap struct {
	data  map[string]*connMap
	mutex sync.RWMutex
}

// join register the connection to room
func (rm *roomMap) join(room string, conn Conn) {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	cm, ok := rm.data[room]
	if !ok {
		rm.data[room] = newConnMap()
		cm = rm.data[room]
	}

	cm.join(conn)
}

func (rm *roomMap) listRoomID() []string {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()

	return getKeysOfMap(rm.data)
}

// leaveAll remove the connection from all rooms
func (rm *roomMap) leaveAll(conn Conn) {
	roomIDList := rm.listRoomID()

	for _, roomID := range roomIDList {
		rm.leave(roomID, conn)
	}
}

// leave remove the connection from the specific room
func (rm *roomMap) leave(room string, conn Conn) {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	// find conn map
	cm, ok := rm.data[room]
	if !ok {
		return
	}

	cm.leave(conn)
	if cm.len() == 0 {
		delete(rm.data, room)
	}
}

// delete remove the specific room
func (rm *roomMap) delete(room string) {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	delete(rm.data, room)
}

// getConnections return connMap for specific room
func (rm *roomMap) getConnections(room string) (*connMap, bool) {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()

	cm, ok := rm.data[room]
	return cm, ok
}

// forEach is use for iterate for read purpose only
func (rm *roomMap) forEach(h func(room string, cm *connMap) bool) {
	roomIDList := rm.listRoomID()

	for _, roomID := range roomIDList {
		cm, ok := rm.getConnections(roomID)
		if !ok || cm == nil {
			continue
		}
		if !h(roomID, cm) {
			break
		}
	}
}

// iterableData return a version of data that is suitable for handle concurrent iteration
func (rm *roomMap) iterableData() map[string]*connMap {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()

	return copyMap(rm.data)
}

// ====================================
// ===========CONN MAP=================
// ====================================

func newConnMap() *connMap {
	return &connMap{data: make(map[string]Conn)}
}

type connMap struct {
	mutex sync.RWMutex
	data  map[string]Conn
}

func (cm *connMap) join(conn Conn) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	cm.data[conn.ID()] = conn
}

func (cm *connMap) leave(conn Conn) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	delete(cm.data, conn.ID())
}

// forEach is use for iterate for read purpose only
func (cm *connMap) forEach(h func(connID string, conn Conn) bool) {
	connIDList := cm.listConnID()

	for _, connID := range connIDList {
		c, ok := cm.getConn(connID)
		if !ok || c == nil {
			continue
		}
		if !h(connID, c) {
			break
		}
	}
}

// getConn return Conn
func (cm *connMap) getConn(connID string) (Conn, bool) {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	c, ok := cm.data[connID]
	return c, ok
}

func (cm *connMap) listConnID() []string {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	return getKeysOfMap(cm.data)
}

// iterableData return a version of data that is suitable for handle concurrent iteration
func (cm *connMap) iterableData() map[string]Conn {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	return copyMap(cm.data)
}

// len return the current size of the map
func (cm *connMap) len() int {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	return len(cm.data)
}

func copyMap[K comparable, V any](m map[K]V) map[K]V {
	res := make(map[K]V, len(m))
	for k, v := range m {
		res[k] = v
	}
	return res
}

func getKeysOfMap[K comparable, V any](m map[K]V) []K {
	res := make([]K, len(m))
	for k := range m {
		res = append(res, k)
	}
	return res
}
