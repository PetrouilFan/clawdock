package terminal

import (
	"io"
	"os/exec"
	"sync"

	"github.com/gorilla/websocket"

	"clawdock/internal/docker"
)

type Terminal struct {
	docker *docker.Client
	mu     sync.Mutex
}

func New(docker *docker.Client) *Terminal {
	return &Terminal{docker: docker}
}

func (t *Terminal) Handle(agentID string, conn *websocket.Conn) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	containerID, err := t.docker.GetContainerIDByLabel("com.openclaw.agent.id", agentID)
	if err != nil {
		return err
	}

	// Use docker exec with interactive TTY
	cmd := exec.Command("docker", "exec", "-it", containerID, "/bin/bash")
	cmd.Stdin = &wsReader{conn: conn}
	cmd.Stdout = &wsWriter{conn: conn, typ: websocket.BinaryMessage}
	cmd.Stderr = &wsWriter{conn: conn, typ: websocket.BinaryMessage}

	if err := cmd.Start(); err != nil {
		return err
	}

	// Wait for the command to finish
	cmd.Wait()
	conn.Close()
	return nil
}

// wsReader wraps websocket.Conn to implement io.Reader
type wsReader struct {
	conn *websocket.Conn
}

func (r *wsReader) Read(p []byte) (int, error) {
	_, reader, err := r.conn.NextReader()
	if err != nil {
		return 0, err
	}
	return reader.Read(p)
}

// wsWriter wraps websocket.Conn to implement io.Writer
type wsWriter struct {
	conn *websocket.Conn
	typ  int
}

func (w *wsWriter) Write(p []byte) (int, error) {
	err := w.conn.WriteMessage(w.typ, p)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

var _ io.Closer = (*websocket.Conn)(nil)
