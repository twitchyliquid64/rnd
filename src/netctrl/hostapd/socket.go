package hostapd

import (
	"net"
	"time"
)

const maxResponseSize = 4096

// Query sends a request to the hostapd socket at sock.
func Query(sock, command string) ([]byte, error) {
	c, err := net.Dial("unixgram", sock)
	if err != nil {
		return nil, err
	}
	defer c.Close()
	if _, err2 := c.Write([]byte(command)); err2 != nil {
		return nil, err
	}
	c.SetReadDeadline(time.Now().Add(300 * time.Millisecond))

	buff := make([]byte, maxResponseSize)
	n, err := c.Read(buff)
	if err != nil {
		return nil, err
	}

	return buff[:n], nil
}
