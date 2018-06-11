# Research Net Daemon

Run a wireless network from a Raspberry pi, which is always behind a VPN.

## Features

 * Circuit breaker - If the VPN fails for any reason or traffic stops getting routed to its interface, packet forwarding is disabled within a second.
 * Easy web interface - A web UI makes it easy for you to switch between your VPNs.
 * DNS over HTTPs - All DNS requests transit via HTTPS (to `dns.google.com`, you can change this in `src/netctrl/bridgeServices.go` if you prefer a different provider).

## Setup

#### Requirements

 * Raspberry Pi 3 (preferable 3 B+)
 * Go version 1.10+
 * A set of openVPN configurations

#### Install hostapd

```shell

sudo apt install -y openvpn easy-rsa hostapd
sudo systemctl mask hostapd.service
sudo systemctl mask wpa_supplicant.service
```

#### Install rnd

```shell

git clone https://github.com/twitchyliquid64/rnd
cd rnd
export GOPATH=`pwd`
go build -o rnd *.go #assumes you have Go install successfully.
```

#### Start daemon

Clear out any state on the wireless NIC: `ip addr flush dev wlan0 && ip link set dev wlan0 down`

Run it with your configuration file: `./rnd myconfig.hcl`

## Example config

```hcl

name = "VPN Controller"
listener = ":1234"

network = {
  interface_ident = "vpn"
  subnet = "192.168.101.1/24"
  wireless = {
    interface = "wlan0"
    SSID = "my_network_name"
    password = "my_password"
  }
}

vpn_configs = [
  {
    name = "USA config 1"
    icon = "flag-icon flag-icon-us"
    path = "us1.ovpn"
    username = "..."
    password = "..."
  },
  {
    name = "USA Config 2"
    icon = "flag-icon flag-icon-us"
    path = "us2.ovpn"
    username = "..."
    password = "..."
  }
]

firewall = {
  vpnbox_blocked_ports = [22, 80]
  blocked_subnets = [
    "192.168.1.1/24"
  ]
}
```

## TODO

Feel free to help out!

 * 'Reboot' button on the web interface
 * Configurable circuit-checker duration
 * Do a circuit check immediately if the VPN process stops
 * Service config so its easier to install on Raspberry pi.
 * 'Stations' page on WebUI.

## Legal

```
Copyright (c) 2018 twitchyliquid64

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```
