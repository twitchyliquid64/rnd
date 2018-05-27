package netctrl

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
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
		case h.name + ".":
			m.Answer = append(m.Answer, &dns.A{
				Hdr: dns.RR_Header{Name: q.Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
				A:   h.baseIP,
			})
		case "googleDNS.":
			m.Answer = append(m.Answer, &dns.A{
				Hdr: dns.RR_Header{Name: q.Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 0},
				A:   net.ParseIP("8.8.8.8"),
			})
		default:
			resp, err := http.Get("https://dns.google.com/resolve?name=" + q.Name + "&type=" + fmt.Sprintf("%d", q.Qtype))
			if err != nil {
				fmt.Printf("Failed to lookup DNS for %v: %v\n", q.Name, err)
				continue
			}
			var response dnsResponse
			if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
				fmt.Printf("Failed to decode DNS response for %v: %v\n", q.Name, err)
				continue
			}
			for _, a := range response.Answer {
				switch a.Type {
				case dns.TypeA:
					m.Answer = append(m.Answer, &dns.A{
						Hdr: dns.RR_Header{Name: q.Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: a.TTL},
						A:   net.ParseIP(a.Data),
					})
				case dns.TypeAAAA:
					m.Answer = append(m.Answer, &dns.AAAA{
						Hdr:  dns.RR_Header{Name: q.Name, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: a.TTL},
						AAAA: net.ParseIP(a.Data),
					})
				case dns.TypeNS:
					m.Answer = append(m.Answer, &dns.NS{
						Hdr: dns.RR_Header{Name: q.Name, Rrtype: dns.TypeNS, Class: dns.ClassINET, Ttl: a.TTL},
						Ns:  a.Data,
					})
				case dns.TypeCNAME:
					m.Answer = append(m.Answer, &dns.CNAME{
						Hdr:    dns.RR_Header{Name: q.Name, Rrtype: dns.TypeCNAME, Class: dns.ClassINET, Ttl: a.TTL},
						Target: a.Data,
					})
				case dns.TypeTXT:
					m.Answer = append(m.Answer, &dns.TXT{
						Hdr: dns.RR_Header{Name: q.Name, Rrtype: dns.TypeTXT, Class: dns.ClassINET, Ttl: a.TTL},
						Txt: []string{a.Data},
					})
				}
			}
		}
	}

	m.RecursionDesired = false
	m.RecursionAvailable = true
	w.WriteMsg(m)
}

type dnsResponse struct {
	Status     int32         `json:"Status,omitempty"`
	Question   []dnsQuestion `json:"Question,omitempty"`
	Answer     []dnsRecord   `json:"Answer,omitempty"`
	Authority  []dnsRecord   `json:"Authority,omitempty"`
	Additional []dnsRecord   `json:"Additional,omitempty"`
}

type dnsQuestion struct {
	Name string `json:"name,omitempty"`
	Type int32  `json:"type,omitempty"`
}

type dnsRecord struct {
	Name string `json:"name,omitempty"`
	Type uint16 `json:"type,omitempty"`
	TTL  uint32 `json:"TTL,omitempty"`
	Data string `json:"data,omitempty"`
}
