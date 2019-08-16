package iterm2

import (
	"fmt"
	"net/http"
	"os"

	"github.com/gorilla/websocket"
)

// ConnectionError is returned whenever an error occurs during the connection
type ConnectionError struct {
	Details string
	Err     error
}

func (ce ConnectionError) Error() string {
	if ce.Err != nil {
		return fmt.Sprintf("error connecting to the iterm2 API: %s, %s", ce.Details, ce.Err)
	}
	return fmt.Sprintf("error connecting to the iterm2 API: %s", ce.Details)
}

func subprotocols() []string {
	return []string{"api.iterm2.com"}
}

func cookie() string {
	return os.Getenv("ITERM2_COOKIE")
}

func key() string {
	return os.Getenv("ITERM2_KEY")
}

func headers() http.Header {
	h := http.Header{}
	h.Set("x-iterm2-library-version", fmt.Sprintf("golang %s", version))
	h.Set("Origin", "ws://localhost/")
	if cookie() != "" {
		h.Set("x-iterm2-cookie", cookie())
	}
	if key() != "" {
		h.Set("x-iterm2-key", key())
	}
	return h
}

// NewConnection connects to the iterm websocket
func NewConnection() (*websocket.Conn, error) {
	d := &websocket.Dialer{}
	*d = *websocket.DefaultDialer
	d.Subprotocols = subprotocols()
	ws, resp, err := d.Dial("ws://localhost:1912", headers())
	if resp != nil {
		if resp.StatusCode == 406 {
			return nil, ConnectionError{"This version of the iterm2 module is too old for the current version of iTerm2. Please upgrade.", nil}
		}
		if resp.StatusCode == http.StatusUnauthorized {
			return nil, ConnectionError{"Access has been denied. You need to accept it in the iterm pop-up.", nil}
		}
	}
	if err != nil {
		return nil, ConnectionError{"error dialing iterm API on ws://localhost:1912", err}
	}
	fmt.Println("Server version:", resp.Header.Get("X-iTerm2-Protocol-Version"))
	return ws, nil
}
