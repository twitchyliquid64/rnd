package netctrl

import (
	"config"
	"errors"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// Controller manages a VPN connection and wifi hotspot.
type Controller struct {
	shutdown chan bool
	wg       sync.WaitGroup

	config *config.Config

	bridgeInterface *net.Interface
	bridgeAddr      net.IP
	subnet          *net.IPNet

	vpnProc      *exec.Cmd
	vpnInterface *net.Interface
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

	vpnAddr := c.bridgeAddr
	vpnAddr[len(vpnAddr)-1]++
	c.vpnProc = exec.Command("openvpn", "--config", vpn.Path, "--dev", "tun"+c.config.Network.InterfaceIdent, "--auth-user-pass", pw.Name(), "--auth-nocache", "--route-noexec")
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

	if c.vpnInterface, err = net.InterfaceByName("tun" + c.config.Network.InterfaceIdent); err != nil {
		return err
	}

	return nil
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
	return ctr, nil
}
