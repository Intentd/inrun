package proxy

import (
	"fmt"
	"net"
)

// CheckPort returns an error if the local port is already in use.
func CheckPort(port int) error {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return err
	}
	ln.Close()
	return nil
}
