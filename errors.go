package socketio

import (
	"errors"
	"fmt"
)

// connect errors.
var (
	errFailedConnectNamespace = errors.New("failed connect to namespace without handler")
)

// common connection gotAck errors.
var (
	errHandleDispatch = errors.New("handler gotAck error")

	errDecodeArgs = errors.New("decode args error")
)

type errorMessage struct {
	namespace string

	err error
}

func (e errorMessage) Error() string {
	return fmt.Sprintf("error in namespace: (%s) with error: (%s)", e.namespace, e.err.Error())
}

func newErrorMessage(namespace string, err error) *errorMessage {
	return &errorMessage{
		namespace: namespace,
		err:       err,
	}
}
