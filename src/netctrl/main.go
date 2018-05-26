package netctrl

import (
	"config"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"netctrl/hostapd"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/coreos/go-iptables/iptables"
	"github.com/krolaw/dhcp4"
	"github.com/vishvananda/netlink"
)

// Controller manages a VPN connection and wifi hotspot.
type Controller struct {
	setupLock sync.Mutex
	shutdown  chan bool
	wg        sync.WaitGroup

	config *config.Config

	bridgeInterface *net.Interface
	bridgeAddr      net.IP
	subnet          *net.IPNet
	areMasquerading bool
	ipt             *iptables.IPTables

	wlanAddr    net.IP
	hostapdProc *exec.Cmd
	lastAPState *hostapd.APStatus

	vpnProc      *exec.Cmd
	vpnInterface *net.Interface
	vpnAddr      net.IP
	vpnConf      *config.VPNOpt

	breakerUpdated time.Time
	breakerTripped bool
}

// Close shuts down the VPN and hotspot
func (c *Controller) Close() error {
	close(c.shutdown)
	c.wg.Wait()

	if c.vpnProc != nil {
		p, err := os.FindProcess(c.vpnProc.Process.Pid)
		if err != nil {
			return err
		}
		if p.Signal(syscall.Signal(0)) == nil {
			p.Kill()
		}
	}

	if c.hostapdProc != nil {
		p, err := os.FindProcess(c.hostapdProc.Process.Pid)
		if err != nil {
			return err
		}
		if p.Signal(syscall.Signal(0)) == nil {
			p.Kill()
		}
	}

	if c.areMasquerading {
		if err := c.ipt.Delete("nat", "POSTROUTING", "-m", "physdev", "--physdev-in", c.config.Network.Wireless.Interface, "-j", "MASQUERADE"); err != nil {
			return err
		}
	}

	return DeleteNetBridge(c.bridgeInterface.Name)
}

// SetVPN sets the network to tunnel all traffic through the VPN specified.
func (c *Controller) SetVPN(vpn *config.VPNOpt) error {
	c.setupLock.Lock()
	defer c.setupLock.Unlock()

	// Prevent forwarding - so traffic is not routed outside the VPN.
	if forwardingEnabled, err2 := IPv4ForwardingEnabled(); err2 == nil && forwardingEnabled {
		if err3 := IPv4EnableForwarding(false); err3 != nil {
			fmt.Printf("Error disabling IPv4 forwarding: %v\n", err3)
		}
	}

	// kill any existing VPN process.
	if c.vpnProc != nil {
		p, err := os.FindProcess(c.vpnProc.Process.Pid)
		if err != nil {
			return err
		}
		if p.Signal(syscall.Signal(0)) == nil {
			p.Kill()
		}
		c.vpnProc = nil
		c.vpnInterface = nil

		// wait for interface to disappear
		timeout := time.NewTicker(5 * time.Second)
		checker := time.NewTicker(50 * time.Millisecond)
		defer timeout.Stop()
		defer checker.Stop()
		for {
			notFound := false
			select {
			case <-timeout.C:
				return errors.New("timeout waiting for VPN to shut down")
			case <-checker.C:
				if _, err2 := net.InterfaceByName("tun" + c.config.Network.InterfaceIdent); err2 != nil {
					notFound = true
				}
			}
			if notFound {
				break
			}
		}
	}

	c.vpnConf = vpn
	pw, err := ioutil.TempFile("", "")
	if err != nil {
		return err
	}
	if _, err = pw.Write([]byte(vpn.Username + "\n" + vpn.Password)); err != nil {
		return err
	}
	if err = pw.Close(); err != nil {
		return err
	}
	defer os.Remove(pw.Name())

	c.vpnProc = exec.Command("openvpn", "--config", vpn.Path, "--dev", "tun"+c.config.Network.InterfaceIdent, "--auth-user-pass", pw.Name(), "--auth-nocache") //, "--route-noexec")
	c.vpnProc.Stdout = os.Stdout
	c.vpnProc.Stderr = os.Stderr
	if err = c.vpnProc.Start(); err != nil {
		return err
	}

	// wait up to 11 seconds for VPN device to appear
	timeout := time.NewTicker(11 * time.Second)
	checker := time.NewTicker(50 * time.Millisecond)
	defer timeout.Stop()
	defer checker.Stop()
	for {
		found := false
		select {
		case <-timeout.C:
			return errors.New("timeout waiting for VPN to come up")
		case <-checker.C:
			if _, err2 := net.InterfaceByName("tun" + c.config.Network.InterfaceIdent); err2 == nil {
				found = true
			}
		}
		if found {
			break
		}
	}

	// get local IP of VPN interface
	if c.vpnInterface, err = net.InterfaceByName("tun" + c.config.Network.InterfaceIdent); err != nil {
		return err
	}
	addrs, err := c.vpnInterface.Addrs()
	if err != nil {
		return err
	}
	if len(addrs) < 1 {
		return errors.New("expected at least one address assigned to VPN")
	}
	c.vpnAddr = addrs[0].(*net.IPNet).IP

	for {
		found := false
		select {
		case <-timeout.C:
			return errors.New("timeout waiting for VPN to routes up")
		case <-checker.C:
			if rts, err := netlink.RouteGet(net.IP{8, 8, 8, 8}); err != nil || rts[0].LinkIndex != c.vpnInterface.Index {
				fmt.Printf("Eval: %+v\n", rts)
			} else {
				found = true
			}
		}
		if found {
			break
		}
	}

	return IPv4EnableForwarding(true)
}

func (c *Controller) circuitBreakerRoutine() {
	defer c.wg.Done()
	t := time.NewTicker(time.Second)
	defer t.Stop()

	for {
		select {
		case <-c.shutdown:
			return
		case <-t.C:
			if c.vpnInterface != nil && !c.breakerTripped {
				c.setupLock.Lock()
				rts, err := netlink.RouteGet(net.IP{8, 8, 8, 8})
				if err != nil {
					fmt.Printf("Failed to eval route: %+v\n", err)
					c.setupLock.Unlock()
					break
				}
				if rts[0].LinkIndex != c.vpnInterface.Index {
					c.breakerTripped = rts[0].LinkIndex != c.vpnInterface.Index
					c.breakerUpdated = time.Now()
					if c.breakerTripped {
						fmt.Println("Tripped:", rts, c.vpnInterface)
						if forwardingEnabled, err2 := IPv4ForwardingEnabled(); err2 == nil && forwardingEnabled {
							if err3 := IPv4EnableForwarding(false); err3 != nil {
								fmt.Printf("Error disabling IPv4 forwarding: %v\n", err3)
							}
						}
					}
				}
				c.setupLock.Unlock()
			} else {
				c.breakerUpdated = time.Now()
			}
		}
	}
}

func (c *Controller) dhcpDNSRoutine() {
	laddr, _ := net.ResolveUDPAddr("udp", ":67")
	listener, err := net.ListenUDP("udp", laddr)
	if err != nil {
		fmt.Printf("DHCP listen err: %v\n", err)
		return
	}
	defer listener.Close()

	options := dhcp4.Options{
		dhcp4.OptionSubnetMask:             []byte{255, 255, 255, 0},
		dhcp4.OptionRouter:                 c.bridgeAddr.To4(),
		dhcp4.OptionPerformRouterDiscovery: []byte{0},
		dhcp4.OptionDomainNameServer:       []byte{8, 8, 8, 8},
	}

	next := dhcp4.IPAdd(c.wlanAddr, 1)
	handler := &bridgeServices{
		debug:   c.config.Debug.DHCP,
		baseIP:  c.wlanAddr,
		next:    next,
		options: options,
		leases:  map[string]net.IP{},
	}
	bcast := dhcp4.IPAdd(next, 0)
	bcast[len(bcast)-1] = 255
	if handler.debug {
		fmt.Printf("DHCP broadcast address = %+v\nRouter address = %+v\n", bcast, c.bridgeAddr)
	}

	if err = handler.setupUDPDNS(); err != nil {
		fmt.Printf("DNS setup failed: %v\n", err)
	}

	for {
		err := dhcp4.Serve(&dhcpLimitedBroadcastListener{conn: listener, bcastAddr: bcast}, handler)
		if err, ok := err.(*net.OpError); ok && !err.Temporary() {
			fmt.Printf("DHCP Serve() err: %v\n", err)
			return
		}
	}
}

// startHostapd starts the hostapd process to manage the AP.
func (c *Controller) startHostapd() error {
	pw, err := ioutil.TempFile("", "")
	if err != nil {
		return err
	}
	conf, err := hostapd.GenerateConfig(c.config)
	if err != nil {
		return err
	}

	if c.config.Debug.Hostapd {
		fmt.Printf("Hostapd Config: %q\n\n", conf)
	}

	if _, err = pw.Write([]byte(conf)); err != nil {
		return err
	}
	if err = pw.Close(); err != nil {
		return err
	}
	defer os.Remove(pw.Name())

	c.hostapdProc = exec.Command("hostapd", "-dd", pw.Name())
	c.hostapdProc.Stdout = os.Stdout
	c.hostapdProc.Stderr = os.Stderr
	if err := c.hostapdProc.Start(); err != nil {
		return err
	}

	// wait up to 8 seconds for Hostapd socket to start responding
	timeout := time.NewTicker(8 * time.Second)
	checker := time.NewTicker(220 * time.Millisecond)
	defer timeout.Stop()
	defer checker.Stop()
	for {
		found := false
		select {
		case <-timeout.C:
			return errors.New("timeout waiting for hostapd to come up")
		case <-checker.C:
			p, err := os.FindProcess(c.hostapdProc.Process.Pid)
			if err != nil {
				return err
			}
			if p.Signal(syscall.Signal(0)) != nil {
				return errors.New("hostapd has stopped")
			}

			resp, err := hostapd.Query("/var/run/hostapd/"+c.config.Network.Wireless.Interface, "STATUS")
			if err != nil {
				continue
			}
			if strings.Contains(string(resp), "state=ENABLED") {
				found = true
			}
		}
		if found {
			break
		}
	}

	if err := c.ipt.AppendUnique("nat", "POSTROUTING", "-m", "physdev", "--physdev-in", c.config.Network.Wireless.Interface, "-j", "MASQUERADE"); err != nil {
		return err
	}
	c.areMasquerading = true
	return nil
}

func (c *Controller) hostapdStatusRoutine() {
	defer c.wg.Done()
	t := time.NewTicker(3 * time.Second)
	defer t.Stop()

	for {
		select {
		case <-c.shutdown:
			return
		case <-t.C:
			if c.hostapdProc != nil {
				c.setupLock.Lock()
				resp, err := hostapd.QueryStatus("/var/run/hostapd/" + c.config.Network.Wireless.Interface)
				if err != nil {
					c.setupLock.Unlock()
					continue
				}
				c.lastAPState = resp
				c.setupLock.Unlock()
			}
		}
	}
}

func (c *Controller) setupFirewall() error {
	for _, port := range c.config.Firewall.VPNBoxBlockedPorts {
		if err := c.ipt.AppendUnique("filter", "INPUT", "-s", c.config.Network.Subnet, "-p", "tcp", "--destination-port", strconv.Itoa(port), "-j", "DROP"); err != nil {
			return err
		}
		if err := c.ipt.AppendUnique("filter", "INPUT", "-s", c.config.Network.Subnet, "-p", "udp", "--destination-port", strconv.Itoa(port), "-j", "DROP"); err != nil {
			return err
		}
	}
	for _, subnet := range c.config.Firewall.BlockedSubnets {
		if err := c.ipt.AppendUnique("filter", "FORWARD", "-s", c.config.Network.Subnet, "-d", subnet, "-j", "DROP"); err != nil {
			return err
		}
	}
	return nil
}

// NewController creates and starts a controller.
func NewController(c *config.Config) (*Controller, error) {
	ipt, err := iptables.New()
	if err != nil {
		return nil, err
	}

	ctr := &Controller{
		shutdown: make(chan bool),
		config:   c,
		ipt:      ipt,
	}
	ctr.bridgeAddr, ctr.subnet, err = net.ParseCIDR(c.Network.Subnet)
	if err != nil {
		return nil, err
	}
	ctr.bridgeInterface, err = CreateNetBridge("br"+c.Network.InterfaceIdent, ctr.bridgeAddr, &net.IPNet{Mask: ctr.subnet.Mask})
	if err != nil {
		return nil, err
	}

	ctr.wlanAddr = dhcp4.IPAdd(ctr.bridgeAddr, 1)
	if c.Network.Wireless.Interface != "" {
		if err := SetInterfaceAddr(c.Network.Wireless.Interface, &net.IPNet{IP: ctr.wlanAddr, Mask: ctr.subnet.Mask}); err != nil {
			DeleteNetBridge(ctr.bridgeInterface.Name)
			return nil, err
		}
	}

	if err := ctr.setupFirewall(); err != nil {
		DeleteNetBridge(ctr.bridgeInterface.Name)
		return nil, err
	}

	if err := ctr.startHostapd(); err != nil {
		DeleteNetBridge(ctr.bridgeInterface.Name)
		return nil, err
	}

	ctr.wg.Add(1)
	go ctr.circuitBreakerRoutine()
	ctr.wg.Add(1)
	go ctr.hostapdStatusRoutine()
	go ctr.dhcpDNSRoutine()
	return ctr, nil
}
