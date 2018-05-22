package hostapd

import (
	"math/rand"
	"net"
	"os"
	"strings"
	"time"
)

const maxResponseSize = 4096

// APStatus represents the status of hostapd.
type APStatus struct {
	State string `json:"state"`
}

// QueryStatus returns the status of the hostapd service.
func QueryStatus(sock string) (*APStatus, error) {
	raw, err := Query(sock, "STATUS")
	if err != nil {
		return nil, err
	}
	set := map[string]string{}
	for _, line := range strings.Split(string(raw), "\n") {
		i := strings.Index(line, "=")
		set[line[:i]] = line[i+1:]
	}
	return &APStatus{
		State: set["state"],
	}, nil
}

// Query sends a request to the hostapd socket at sock.
func Query(sock, command string) ([]byte, error) {
	lf := randStringFname()
	defer os.Remove(lf)
	laddr := net.UnixAddr{Name: lf, Net: "unixgram"}
	c, err := net.DialUnix("unixgram", &laddr, &net.UnixAddr{Name: sock, Net: "unixgram"})
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

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func randStringFname() string {
	b := make([]rune, 12)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return "/tmp/rnd-s-" + string(b)
}
