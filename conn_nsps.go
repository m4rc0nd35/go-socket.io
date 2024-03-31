package socketio

import "sync"

type namespaceConns struct {
	namespaces sync.Map
}

func newNamespaceConns() *namespaceConns {
	return &namespaceConns{}
}

func (n *namespaceConns) Get(ns string) (*namespaceConn, bool) {
	raw, ok := n.namespaces.Load(ns)
	if !ok {
		return nil, false
	}
	namespace, ok := raw.(*namespaceConn)
	return namespace, ok
}

func (n *namespaceConns) Set(ns string, conn *namespaceConn) {
	n.namespaces.Store(ns, conn)
}

func (n *namespaceConns) Delete(ns string) {
	n.namespaces.Delete(ns)
}

func (n *namespaceConns) Range(fn func(ns string, nc *namespaceConn)) {
	n.namespaces.Range(func(rawKey, rawVal any) bool {
		key, ok := rawKey.(string)
		if !ok {
			// continue on next
			return true
		}
		val, ok := rawVal.(*namespaceConn)
		if !ok {
			// continue on next
			return true
		}
		fn(key, val)
		return true
	})
}
