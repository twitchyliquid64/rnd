package netctrl

import (
	"fmt"
	"net"
	"time"

	dhcp "github.com/krolaw/dhcp4"
)

type bridgeServices struct {
	bridgeIP, next net.IP
	leases         map[string]net.IP
	options        dhcp.Options // Options to send to DHCP Clients
}

func (h *bridgeServices) ServeDHCP(p dhcp.Packet, msgType dhcp.MessageType, options dhcp.Options) (d dhcp.Packet) {
	fmt.Printf("DHCP msg from %q: %+v\n", p.CHAddr().String(), p)
	for n, opt := range h.options {
		options[n] = opt
	}

	switch msgType {

	case dhcp.Discover:
		if ip, exists := h.leases[p.CHAddr().String()]; exists {
			return dhcp.ReplyPacket(p, dhcp.Offer, h.bridgeIP, ip, time.Hour*24,
				h.options.SelectOrderOrAll(options[dhcp.OptionParameterRequestList]))
		}

		return dhcp.ReplyPacket(p, dhcp.Offer, h.bridgeIP, h.next, time.Hour*24,
			h.options.SelectOrderOrAll(options[dhcp.OptionParameterRequestList]))

	case dhcp.Request:
		if server, ok := options[dhcp.OptionServerIdentifier]; ok && !net.IP(server).Equal(h.bridgeIP) {
			return nil // Message not for this dhcp server
		}
		reqIP := net.IP(options[dhcp.OptionRequestedIPAddress])
		if reqIP == nil {
			reqIP = net.IP(p.CIAddr())
		}

		if len(reqIP) == 4 && !reqIP.Equal(net.IPv4zero) && reqIP.Equal(h.next) {
			h.next = dhcp.IPAdd(h.next, 1)
			h.leases[p.CHAddr().String()] = reqIP
			return dhcp.ReplyPacket(p, dhcp.ACK, h.bridgeIP, reqIP, time.Hour*24,
				h.options.SelectOrderOrAll(options[dhcp.OptionParameterRequestList]))
		} else if ip, exists := h.leases[p.CHAddr().String()]; exists && ip.Equal(reqIP) {
			return dhcp.ReplyPacket(p, dhcp.ACK, h.bridgeIP, reqIP, time.Hour*24,
				h.options.SelectOrderOrAll(options[dhcp.OptionParameterRequestList]))
		}
		return dhcp.ReplyPacket(p, dhcp.NAK, h.bridgeIP, nil, 0, nil)
	}
	return nil
}
