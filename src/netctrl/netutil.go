package netctrl

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net"

	"github.com/vishvananda/netlink"
)

// ErrDeviceExists indicates a device with that name already exists.
var ErrDeviceExists = errors.New("interface with that name already exists")

// CreateNetBridge creates a new bridge device with the specified name and IP configuration.
// if a device with devName already exists, ErrDeviceExists is returned.
func CreateNetBridge(devName string, ip net.IP, subnet *net.IPNet) (*net.Interface, error) {
	if _, err := net.InterfaceByName(devName); err == nil {
		return nil, ErrDeviceExists
	}

	nlBridge := &netlink.Bridge{LinkAttrs: netlink.LinkAttrs{Name: devName}}
	if err := netlink.LinkAdd(nlBridge); err != nil {
		return nil, err
	}
	ipConfig := &netlink.Addr{IPNet: &net.IPNet{IP: ip, Mask: subnet.Mask}}
	if err := netlink.AddrAdd(nlBridge, ipConfig); err != nil {
		return nil, err
	}

	if err := netlink.LinkSetUp(nlBridge); err != nil {
		return nil, err
	}
	return net.InterfaceByName(devName)
}

// DeleteNetBridge destroys a network bridge.
func DeleteNetBridge(devName string) error {
	return netlink.LinkDel(&netlink.Bridge{LinkAttrs: netlink.LinkAttrs{Name: devName}})
}

// AttachNetBridge attaches an interface to the interface of a bridge.
func AttachNetBridge(bridge, client *net.Interface) error {
	bridgeLink, err := netlink.LinkByName(bridge.Name)
	if err != nil {
		return err
	}
	clientLink, err := netlink.LinkByName(client.Name)
	if err != nil {
		return err
	}

	return netlink.LinkSetMaster(clientLink, bridgeLink.(*netlink.Bridge))
}

// RouteAddViaGatewayFromAddr adds a new route to the given IP network,
// routed by the given gateway when it comes from the given source.
// This is equivalent to 'ip route add <destination> via <gateway>'.
func RouteAddViaGatewayFromAddr(destination *net.IPNet, source, gateway net.IP) error {

	route := &netlink.Route{
		Src:      source,
		Dst:      destination,
		Gw:       gateway,
		Priority: 1337,
	}
	return netlink.RouteAdd(route)
}

// SetInterfaceAddr sets the IP address on that interface.
func SetInterfaceAddr(iName string, addr *net.IPNet) error {
	intf, err := netlink.LinkByName(iName)
	if err != nil {
		return err
	}
	return netlink.AddrAdd(intf, &netlink.Addr{IPNet: addr})
}

// IPv4ForwardingEnabled returns true if the kernel is configured to forward IPv4 packets.
func IPv4ForwardingEnabled() (bool, error) {
	d, err := ioutil.ReadFile("/proc/sys/net/ipv4/ip_forward")
	if err != nil {
		return false, err
	}
	if len(d) != 2 {
		return false, fmt.Errorf("expected single byte read, got %d", len(d))
	}
	return d[0] == '1', nil
}

// IPv4EnableForwarding enables or disables forwarding of IPv4 packets.
func IPv4EnableForwarding(state bool) error {
	outData := "0"
	if state {
		outData = "1"
	}
	return ioutil.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte(outData), 0644)
}
