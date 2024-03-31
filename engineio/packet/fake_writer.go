package packet

import (
	"io"

	"github.com/m4rc0nd35/go-socket.io/engineio/frame"
)

type fakeConnWriter struct {
	Frames []Frame
}

func newFakeConnWriter() *fakeConnWriter {
	return &fakeConnWriter{}
}

func (w *fakeConnWriter) NextWriter(fType frame.Type) (io.WriteCloser, error) {
	return newFakeFrame(w, fType), nil
}
