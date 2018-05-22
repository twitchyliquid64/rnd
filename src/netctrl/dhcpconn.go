package netctrl

import "net"

type dhcpLimitedBroadcastListener struct {
	conn      *net.UDPConn
	bcastAddr net.IP
}

func (s *dhcpLimitedBroadcastListener) ReadFrom(b []byte) (n int, addr net.Addr, err error) {
	return s.conn.ReadFrom(b)
}

func (s *dhcpLimitedBroadcastListener) WriteTo(b []byte, addr net.Addr) (n int, err error) {
	if addr.(*net.UDPAddr).IP.String() == net.IPv4bcast.String() {
		newAddr := *addr.(*net.UDPAddr)
		newAddr.IP = s.bcastAddr
		return s.conn.WriteTo(b, &newAddr)
	}
	return s.conn.WriteTo(b, addr)
}

func (s *dhcpLimitedBroadcastListener) Close() error { return s.conn.Close() }
