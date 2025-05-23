package core

import (
	"bytes"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/jackpal/gateway"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/mem"
)

func AutoConnect(MONITOR chan int) {
	defer func() {
		time.Sleep(30 * time.Second)
		MONITOR <- 200
	}()

	for {
	next:
		for _, v := range C.Connections {
			if v == nil || !v.AutoConnect {
				continue
			}
			for _, vc := range TunList {
				if vc == nil {
					continue
				}
				if vc.Interface != nil {
					x := *vc.Interface.tunnel.Load()
					if x == nil {
						continue
					}
					if x.Meta.Tag == v.Tag {
						continue next
					}
				}
			}

			CR := ConnectionRequest{
				Tag:        v.Tag,
				DeviceKey:  CLIDeviceKey,
				ServerIP:   CLIHost,
				ServerPort: CLIPort,
				EncType:    v.EncryptionType,
				CurveType:  v.CurveType,
			}

			code, err := PublicConnect(CR)
			if err != nil {
				ERROR("Unable to connect, return code: ", code, " // error: ", err)
			}
		}

		break
	}
}

var PingPongStatsBuffer = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

func PopulatePingBufferWithStats() {
	cpuPercent, err := cpu.Percent(0, false)
	if err != nil {
		ERROR("Unable to get cpu percent", err)
		return
	}
	PingPongStatsBuffer[1] = byte(int(cpuPercent[0]))

	memStats, err := mem.VirtualMemory()
	if err != nil {
		ERROR("Unable to get mem stats", err)
		return

	}
	PingPongStatsBuffer[2] = byte(int(memStats.UsedPercent))

	diskUsage, err := disk.Usage("/")
	if err != nil {
		ERROR("Unable to get disk usage", err)
		return
	}
	PingPongStatsBuffer[3] = byte(int(diskUsage.UsedPercent))
}

var prevAllowedHosts = []string{}

func applyFirewallRules(t *Tunnel) {
	defer RecoverAndLogToFile()
	out := t.EH.SEAL.Seal1(PingPongStatsBuffer, t.Index)
	if len(out) > 0 {
		_, _ = t.Con.Write(out)
	}
}

func PingConnections(MONITOR chan int) {
	defer func() {
		time.Sleep(30 * time.Second)
		MONITOR <- 3
	}()
	defer RecoverAndLogToFile()
	if MINIMAL {
		PopulatePingBufferWithStats()
	}
	for _, v := range TunList {
		if v == nil {
			continue
		}

		var err error
		if v.EH != nil {
			out := v.EH.SEAL.Seal1(PingPongStatsBuffer, v.Index)
			if len(out) > 0 {
				_, err = v.Con.Write(out)
				if err != nil {
					ERROR("unable to ping tunnel: ", v.Meta.Tag)
				}
			}

		}

		if time.Since(v.TunnelSTATS.PingTime).Seconds() > 30 || err != nil {
			if v.Meta.AutoReconnect {
				DEBUG("30+ Seconds since ping from ", v.Meta.Tag, " attempting reconnection")
				_, _ = PublicConnect(v.ClientCR)
			} else {
				DEBUG("30+ Seconds since ping from ", v.Meta.Tag, " disconnecting")
				_ = Disconnect(v.Meta.WindowsGUID, true, false)
			}
		}
	}
}

func isGatewayATunnel(gateway net.IP) (isTunnel bool) {
	for _, v := range C.Connections {
		if v.IPv4Address == gateway.To4().String() {
			return true
		}
	}
	return false
}

func getDefaultGatewayAndInterface() {
	defer RecoverAndLogToFile()
	var err error
	var onDefault bool = false

	for _, v := range GLOBAL_STATE.ActiveConnections {
		if v.Tag == DefaultTunnelName {
			onDefault = true
		}
	}

	OLD_GATEWAY := make([]byte, 4)
	var NEW_GATEWAY net.IP
	copy(OLD_GATEWAY, DEFAULT_GATEWAY.To4())

	NEW_GATEWAY, err = gateway.DiscoverGateway()
	if err != nil {
		if !onDefault {
			con, err2 := net.Dial("tcp4", "9.9.9.9")
			if err2 == nil {
				newGate := net.ParseIP(strings.Split(con.LocalAddr().String(), ":")[0])
				if !isGatewayATunnel(newGate) {
					NEW_GATEWAY = newGate
				}
			}
			ERROR("default gateway not found, err:", err, "//  dial err:", err2)
		}
		return
	}

	if isGatewayATunnel(NEW_GATEWAY) {
		return
	}

	if bytes.Equal(OLD_GATEWAY, NEW_GATEWAY.To4()) {
		return
	}

	DEBUG("new gateway discovered", NEW_GATEWAY)
	DEFAULT_GATEWAY = NEW_GATEWAY

	DEFAULT_INTERFACE, err = gateway.DiscoverInterface()
	if err != nil {
		ERROR("could not find default interface", err)
		return
	}

	if DNSClient != nil && DNSClient.Dialer != nil {
		DNSClient.Dialer.LocalAddr = &net.UDPAddr{
			IP: DEFAULT_INTERFACE,
		}
	}

	ifList, _ := net.Interfaces()

LOOP:
	for _, v := range ifList {
		addrs, e := v.Addrs()
		if e != nil {
			continue
		}
		for _, iv := range addrs {
			if strings.Split(iv.String(), "/")[0] == DEFAULT_INTERFACE.To4().String() {
				DEFAULT_INTERFACE_ID = v.Index
				DEFAULT_INTERFACE_NAME = v.Name
				_ = GetDNSServers(strconv.Itoa(v.Index))
				break LOOP
			}
		}
	}

	if DEFAULT_DNS_SERVERS == nil {
		if C.DNS2Default != "" {
			DEFAULT_DNS_SERVERS = []string{C.DNS1Default, C.DNS2Default}
		} else {
			DEFAULT_DNS_SERVERS = []string{C.DNS1Default}
		}
	}

	DEBUG(
		"Default interface",
		DEFAULT_INTERFACE_NAME,
		DEFAULT_INTERFACE_ID,
		DEFAULT_INTERFACE,
	)
}

func GetDefaultGateway(MONITOR chan int) {
	defer func() {
		if DEFAULT_GATEWAY != nil {
			time.Sleep(5 * time.Second)
		} else {
			time.Sleep(2 * time.Second)
		}

		MONITOR <- 4
	}()
	getDefaultGatewayAndInterface()
}
