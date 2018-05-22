package fsock

import (
	"net"

	"golang.org/x/net/ipv4"
)

// NewLiveListener creates a listener on all interfaces and then filters packets not received by the given interfaces.
func NewLiveListener(interfaceNames []string, laddr string) (c *serveIfConn, e error) {
	l, err := net.ListenPacket("udp4", laddr)
	if err != nil {
		return nil, err
	}
	defer func() {
		if e != nil {
			l.Close()
		}
	}()
	p := ipv4.NewPacketConn(l)
	if err := p.SetControlMessage(ipv4.FlagInterface, true); err != nil {
		return nil, err
	}
	return &serveIfConn{ifNames: interfaceNames, conn: p}, nil
}

type serveIfConn struct {
	ifNames []string
	conn    *ipv4.PacketConn
	cm      *ipv4.ControlMessage
}

func (s *serveIfConn) ReadFrom(b []byte) (n int, addr net.Addr, err error) {
	for { // Filter all other interfaces
		n, s.cm, addr, err = s.conn.ReadFrom(b)
		if err != nil || s.cm == nil {
			break
		}
		inf, err2 := net.InterfaceByIndex(s.cm.IfIndex)
		if err2 != nil {
			continue
		}
		for _, ifName := range s.ifNames {
			if ifName == inf.Name {
				break
			}
		}
	}
	return
}

func (s *serveIfConn) WriteTo(b []byte, addr net.Addr) (n int, err error) {

	// ipv4 docs state that Src is "specify only", however testing by tfheen
	// shows that Src IS populated.  Therefore, to reuse the control message,
	// we set Src to nil to avoid the error "write udp4: invalid argument"
	s.cm.Src = nil
	return s.conn.WriteTo(b, s.cm, addr)
}

func (s *serveIfConn) Close() error { return s.conn.Close() }
