package netctrl

import (
	"fmt"
	"net"
	"time"

	dhcp "github.com/krolaw/dhcp4"
	"github.com/miekg/dns"
)

type bridgeServices struct {
	name         string
	debug        bool
	baseIP, next net.IP
	leases       map[string]net.IP
	options      dhcp.Options // Options to send to DHCP Clients
}

func (h *bridgeServices) ServeDHCP(p dhcp.Packet, msgType dhcp.MessageType, options dhcp.Options) (d dhcp.Packet) {
	if h.debug {
		fmt.Printf("DHCP msg %q from %q\n", msgType.String(), p.CHAddr().String())
		fmt.Printf("Leases: %+v\nNext address: %+v\nBase address: %+v\n", h.leases, h.next, h.baseIP)
	}

	for n, opt := range h.options {
		options[n] = opt
	}

	switch msgType {

	case dhcp.Discover:
		if ip, exists := h.leases[p.CHAddr().String()]; exists {
			return dhcp.ReplyPacket(p, dhcp.Offer, h.baseIP, ip, time.Hour*24,
				h.options.SelectOrderOrAll(options[dhcp.OptionParameterRequestList]))
		}

		return dhcp.ReplyPacket(p, dhcp.Offer, h.baseIP, h.next, time.Hour*24,
			h.options.SelectOrderOrAll(options[dhcp.OptionParameterRequestList]))

	case dhcp.Request:
		if server, ok := options[dhcp.OptionServerIdentifier]; ok && !net.IP(server).Equal(h.baseIP) {
			if h.debug {
				fmt.Printf("DHCP msg for %q, we are %q\n", server, h.baseIP.String())
			}
			return nil // Message not for this dhcp server
		}
		reqIP := dhcp.IPAdd(options[dhcp.OptionRequestedIPAddress], 0)
		if reqIP == nil {
			reqIP = net.IP(dhcp.IPAdd(p.CIAddr(), 0))
		}

		if len(reqIP) == 4 && !reqIP.Equal(net.IPv4zero) && reqIP.Equal(h.next) {
			h.next = dhcp.IPAdd(h.next, 1)
			h.leases[p.CHAddr().String()] = reqIP
			return dhcp.ReplyPacket(p, dhcp.ACK, h.baseIP, reqIP, time.Hour*24,
				h.options.SelectOrderOrAll(options[dhcp.OptionParameterRequestList]))
		} else if ip, exists := h.leases[p.CHAddr().String()]; exists && ip.Equal(reqIP) {
			return dhcp.ReplyPacket(p, dhcp.ACK, h.baseIP, reqIP, time.Hour*24,
				h.options.SelectOrderOrAll(options[dhcp.OptionParameterRequestList]))
		}
		return dhcp.ReplyPacket(p, dhcp.NAK, h.baseIP, nil, 0, nil)
	}
	return nil
}

func (h *bridgeServices) setupUDPDNS(listenerIP string) error {
	laddr, err := net.ResolveUDPAddr("udp", listenerIP+":53")
	if err != nil {
		return err
	}

	listener, err := net.ListenUDP("udp", laddr)
	if err != nil {
		return err
	}

	server := &dns.Server{PacketConn: listener, Handler: h, ReadTimeout: time.Second, WriteTimeout: time.Second}
	go server.ActivateAndServe()
	return nil
}

// ServeDNS handles DNS requests.
func (h *bridgeServices) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	m := new(dns.Msg)
	m.SetReply(r)

	for _, q := range r.Question {
		switch q.Name {
		case h.name:
			m.Answer = append(m.Answer, &dns.A{
				Hdr: dns.RR_Header{Name: q.Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 0},
				A:   h.baseIP,
			})
		case "googleDNS.":
			m.Answer = append(m.Answer, &dns.A{
				Hdr: dns.RR_Header{Name: q.Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 0},
				A:   net.ParseIP("8.8.8.8"),
			})
		}
	}

	m.RecursionDesired = false
	m.RecursionAvailable = false
	w.WriteMsg(m)
}
