package netctrl

import (
	"config"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

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

	return DeleteNetBridge(c.bridgeInterface.Name)
}

// SetVPN sets the network to tunnel all traffic through the VPN specified.
func (c *Controller) SetVPN(vpn *config.VPNOpt) error {
	c.setupLock.Lock()
	defer c.setupLock.Unlock()
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

	// wait up to 8 seconds for VPN device to appear
	timeout := time.NewTicker(8 * time.Second)
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
	return nil
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
					break
				}
				if rts[0].LinkIndex != c.vpnInterface.Index {
					c.breakerTripped = rts[0].LinkIndex != c.vpnInterface.Index
					c.breakerUpdated = time.Now()
					if c.breakerTripped {
						fmt.Println("Tripped:", rts, c.vpnInterface)
						// do thing
					}
				}
				c.setupLock.Unlock()
			} else {
				c.breakerUpdated = time.Now()
			}
		}
	}
}

// NewController creates and starts a controller.
func NewController(c *config.Config) (*Controller, error) {
	ctr := &Controller{
		shutdown: make(chan bool),
		config:   c,
	}
	var err error
	ctr.bridgeAddr, ctr.subnet, err = net.ParseCIDR(c.Network.Subnet)
	if err != nil {
		return nil, err
	}
	ctr.bridgeInterface, err = CreateNetBridge("br"+c.Network.InterfaceIdent, ctr.bridgeAddr, &net.IPNet{Mask: ctr.subnet.Mask})
	if err != nil {
		return nil, err
	}

	ctr.wg.Add(1)
	go ctr.circuitBreakerRoutine()
	return ctr, nil
}
