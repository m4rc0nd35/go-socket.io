package main

import (
	"fmt"
	"log"
	"net/http"

	socketio "github.com/m4rc0nd35/go-socket.io"
)

func main() {
	clients := []socketio.Conn{}
	server := socketio.NewServer(nil)

	server.OnConnect("/", func(s socketio.Conn, m map[string]any) error {
		s.SetContext(s.Context())

		clients = append(clients, s)
		fmt.Println("connected:", s.ID(), s.RemoteHeader())
		return nil
	})

	server.OnEvent("/", "notice", func(s socketio.Conn, msg map[string]string) {
		fmt.Println("notice:", msg, len(clients))
		s.Emit("reply", msg["name"])
	})

	server.OnEvent("/chat", "msg", func(s socketio.Conn, msg string) string {
		s.SetContext(s.Context())
		fmt.Println(msg)
		return "recv " + msg
	})

	server.OnEvent("/", "bye", func(s socketio.Conn) string {
		last := s.Context()
		s.Emit("bye", last)
		s.Close()
		return "last.(string)"
	})

	server.OnError("/", func(s socketio.Conn, e error) {
		fmt.Println("meet error:", e)
	})

	server.OnDisconnect("/", func(s socketio.Conn, reason string, m map[string]any) {
		fmt.Println("closed", reason)
	})

	go server.Serve()
	defer server.Close()

	http.Handle("/event/", server)
	http.Handle("/", http.FileServer(http.Dir("./asset")))
	log.Println("Serving at localhost:8090...")
	log.Fatal(http.ListenAndServe(":8090", nil))
}
