//go:build windows

package platform

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	contracts "wails-app/internal/contracts"
)

const (
	gaaFlagIncludePrefix   = 0x0010
	gaaFlagIncludeGateways = 0x0080

	ipAdapterDhcpEnabled = 0x0004

	ifAdminStatusUp   = 1
	ifAdminStatusDown = 2

	errorBufferOverflow = 111

	shellShowNormal = 1
)

var (
	procGetAdaptersAddresses = iphlpapiDLL.NewProc("GetAdaptersAddresses")
	procGetIfEntry           = iphlpapiDLL.NewProc("GetIfEntry")
	procSetIfEntry           = iphlpapiDLL.NewProc("SetIfEntry")
	procIpRenewAddress       = iphlpapiDLL.NewProc("IpRenewAddress")

	shell32DLL       = windows.NewLazySystemDLL("shell32.dll")
	procShellExecute = shell32DLL.NewProc("ShellExecuteW")
)

type socketAddress struct {
	Sockaddr       *syscall.RawSockaddrAny
	SockaddrLength int32
}

type adapterUnicastAddress struct {
	Length  uint32
	Flags   uint32
	Next    *adapterUnicastAddress
	Address socketAddress
}

type adapterDnsServerAddress struct {
	Length   uint32
	Reserved uint32
	Next     *adapterDnsServerAddress
	Address  socketAddress
}

type adapterGatewayAddress struct {
	Length   uint32
	Reserved uint32
	Next     *adapterGatewayAddress
	Address  socketAddress
}

// ipAdapterAddresses intentionally models the stable prefix of
// IP_ADAPTER_ADDRESSES_LH through FirstGatewayAddress. That is enough for
// identity, addresses, DNS, gateways, DHCP, MTU and link speed.
type ipAdapterAddresses struct {
	Length                 uint32
	IfIndex                uint32
	Next                   *ipAdapterAddresses
	AdapterName            *byte
	FirstUnicastAddress    *adapterUnicastAddress
	FirstAnycastAddress    uintptr
	FirstMulticastAddress  uintptr
	FirstDnsServerAddress  *adapterDnsServerAddress
	DnsSuffix              *uint16
	Description            *uint16
	FriendlyName           *uint16
	PhysicalAddress        [8]byte
	PhysicalAddressLength  uint32
	Flags                  uint32
	Mtu                    uint32
	IfType                 uint32
	OperStatus             uint32
	Ipv6IfIndex            uint32
	ZoneIndices            [16]uint32
	FirstPrefix            uintptr
	TransmitLinkSpeed      uint64
	ReceiveLinkSpeed       uint64
	FirstWinsServerAddress uintptr
	FirstGatewayAddress    *adapterGatewayAddress
}

type mibIfRow struct {
	Name            [256]uint16
	Index           uint32
	Type            uint32
	Mtu             uint32
	Speed           uint32
	PhysAddrLen     uint32
	PhysAddr        [8]byte
	AdminStatus     uint32
	OperStatus      uint32
	LastChange      uint32
	InOctets        uint32
	InUcastPkts     uint32
	InNUcastPkts    uint32
	InDiscards      uint32
	InErrors        uint32
	InUnknownProtos uint32
	OutOctets       uint32
	OutUcastPkts    uint32
	OutNUcastPkts   uint32
	OutDiscards     uint32
	OutErrors       uint32
	OutQLen         uint32
	DescrLen        uint32
	Descr           [256]byte
}

type ipAdapterIndexMap struct {
	Index uint32
	Name  [128]uint16
}

// NetworkAdapterMetadata returns rich native adapter metadata keyed by interface
// index. It is used to enrich the ordinary network:// adapter rows.
func NetworkAdapterMetadata() (map[uint32]map[string]any, error) {
	var size uint32 = 16 * 1024
	flags := uintptr(gaaFlagIncludePrefix | gaaFlagIncludeGateways)

	for attempts := 0; attempts < 3; attempts++ {
		buffer := make([]byte, size)
		result, _, callErr := procGetAdaptersAddresses.Call(
			0, // AF_UNSPEC
			flags,
			0,
			uintptr(unsafe.Pointer(&buffer[0])),
			uintptr(unsafe.Pointer(&size)),
		)

		if result == errorBufferOverflow {
			continue
		}
		if result != 0 {
			return nil, windowsError(result, callErr)
		}

		resultMap := make(map[uint32]map[string]any)
		current := (*ipAdapterAddresses)(unsafe.Pointer(&buffer[0]))
		for current != nil {
			index := current.IfIndex
			if index == 0 {
				index = current.Ipv6IfIndex
			}
			if index != 0 {
				fields := map[string]any{
					"index":         index,
					"friendlyName":  windows.UTF16PtrToString(current.FriendlyName),
					"description":   windows.UTF16PtrToString(current.Description),
					"dnsSuffix":     windows.UTF16PtrToString(current.DnsSuffix),
					"mtu":           current.Mtu,
					"ifType":        current.IfType,
					"status":        adapterOperStatus(current.OperStatus),
					"dhcpEnabled":   current.Flags&ipAdapterDhcpEnabled != 0,
					"linkSpeed":     maxU64(current.TransmitLinkSpeed, current.ReceiveLinkSpeed),
					"transmitSpeed": current.TransmitLinkSpeed,
					"receiveSpeed":  current.ReceiveLinkSpeed,
				}

				macLength := int(current.PhysicalAddressLength)
				if macLength > len(current.PhysicalAddress) {
					macLength = len(current.PhysicalAddress)
				}
				if macLength > 0 {
					fields["mac"] = formatMAC(current.PhysicalAddress[:macLength])
				}

				addresses := collectAdapterAddresses(current.FirstUnicastAddress)
				gateways := collectGatewayAddresses(current.FirstGatewayAddress)
				dnsServers := collectDNSServerAddresses(current.FirstDnsServerAddress)
				fields["addresses"] = addresses
				fields["address"] = firstString(addresses)
				fields["gateways"] = gateways
				fields["gateway"] = firstString(gateways)
				fields["dnsServers"] = dnsServers
				fields["dns"] = strings.Join(dnsServers, ", ")

				if row, err := readIfRow(index); err == nil {
					fields["adminStatus"] = adapterAdminStatus(row.AdminStatus)
					fields["bytesReceived"] = uint64(row.InOctets)
					fields["bytesSent"] = uint64(row.OutOctets)
					fields["receiveErrors"] = uint64(row.InErrors)
					fields["sendErrors"] = uint64(row.OutErrors)
					fields["receiveDiscards"] = uint64(row.InDiscards)
					fields["sendDiscards"] = uint64(row.OutDiscards)
				}

				resultMap[index] = fields
			}
			current = current.Next
		}
		return resultMap, nil
	}

	return nil, fmt.Errorf("GetAdaptersAddresses: buffer size kept changing")
}

func SetNetworkAdapterEnabled(index uint32, enabled bool) error {
	row, err := readIfRow(index)
	if err != nil {
		return err
	}

	if enabled {
		row.AdminStatus = ifAdminStatusUp
	} else {
		row.AdminStatus = ifAdminStatusDown
	}

	result, _, callErr := procSetIfEntry.Call(uintptr(unsafe.Pointer(row)))
	if result != 0 {
		return windowsError(result, callErr)
	}
	return nil
}

func RenewNetworkAdapterDHCP(index uint32) error {
	mapping := ipAdapterIndexMap{Index: index}
	result, _, callErr := procIpRenewAddress.Call(uintptr(unsafe.Pointer(&mapping)))
	if result != 0 {
		return windowsError(result, callErr)
	}
	return nil
}

func DiagnoseNetworkAdapter(index uint32) (contracts.SystemSurfaceRecord, error) {
	metadata, err := NetworkAdapterMetadata()
	if err != nil {
		return contracts.SystemSurfaceRecord{}, err
	}
	fields, ok := metadata[index]
	if !ok {
		return contracts.SystemSurfaceRecord{}, fmt.Errorf("network adapter %d not found", index)
	}

	checks := make([]string, 0, 6)
	healthy := true

	status := strings.ToLower(fmt.Sprint(fields["status"]))
	if status == "up" {
		checks = append(checks, "Interface is up")
	} else {
		healthy = false
		checks = append(checks, "Interface is not operational")
	}

	if strings.TrimSpace(fmt.Sprint(fields["address"])) != "" {
		checks = append(checks, "IP address is configured")
	} else {
		healthy = false
		checks = append(checks, "No IP address is configured")
	}

	if strings.TrimSpace(fmt.Sprint(fields["gateway"])) != "" {
		checks = append(checks, "Default gateway is configured")
	} else {
		checks = append(checks, "No default gateway is configured")
	}

	if strings.TrimSpace(fmt.Sprint(fields["dns"])) != "" {
		checks = append(checks, "DNS servers are configured")
	} else {
		checks = append(checks, "No DNS servers are configured")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2500*time.Millisecond)
	defer cancel()
	if _, lookupErr := net.DefaultResolver.LookupHost(ctx, "www.microsoft.com"); lookupErr == nil {
		checks = append(checks, "DNS resolution succeeds")
	} else {
		healthy = false
		checks = append(checks, "DNS resolution failed: "+lookupErr.Error())
	}

	conn, dialErr := net.DialTimeout("tcp", "1.1.1.1:443", 2500*time.Millisecond)
	if dialErr == nil {
		_ = conn.Close()
		checks = append(checks, "Outbound TCP connectivity succeeds")
	} else {
		healthy = false
		checks = append(checks, "Outbound TCP connectivity failed: "+dialErr.Error())
	}

	return contracts.SystemSurfaceRecord{
		ID:   fmt.Sprintf("adapter-diagnostic:%d", index),
		Name: fmt.Sprint(fields["friendlyName"]),
		Fields: map[string]any{
			"healthy": healthy,
			"summary": strings.Join(checks, "\n"),
			"checks":  checks,
		},
	}, nil
}

func OpenNetworkSettings() error {
	target, err := windows.UTF16PtrFromString("ms-settings:network-status")
	if err != nil {
		return err
	}
	result, _, callErr := procShellExecute.Call(
		0,
		0,
		uintptr(unsafe.Pointer(target)),
		0,
		0,
		shellShowNormal,
	)
	if result <= 32 {
		if callErr != nil && callErr != windows.ERROR_SUCCESS {
			return callErr
		}
		return fmt.Errorf("ShellExecuteW(ms-settings:network-status) failed: %d", result)
	}
	return nil
}

func readIfRow(index uint32) (*mibIfRow, error) {
	row := &mibIfRow{Index: index}
	result, _, callErr := procGetIfEntry.Call(uintptr(unsafe.Pointer(row)))
	if result != 0 {
		return nil, windowsError(result, callErr)
	}
	return row, nil
}

func collectAdapterAddresses(head *adapterUnicastAddress) []string {
	result := make([]string, 0, 4)
	for current := head; current != nil; current = current.Next {
		if value := socketAddressIP(current.Address); value != "" {
			result = appendUnique(result, value)
		}
	}
	sort.Strings(result)
	return result
}

func collectDNSServerAddresses(head *adapterDnsServerAddress) []string {
	result := make([]string, 0, 4)
	for current := head; current != nil; current = current.Next {
		if value := socketAddressIP(current.Address); value != "" {
			result = appendUnique(result, value)
		}
	}
	return result
}

func collectGatewayAddresses(head *adapterGatewayAddress) []string {
	result := make([]string, 0, 2)
	for current := head; current != nil; current = current.Next {
		if value := socketAddressIP(current.Address); value != "" {
			result = appendUnique(result, value)
		}
	}
	return result
}

func socketAddressIP(address socketAddress) string {
	if address.Sockaddr == nil || address.SockaddrLength <= 0 {
		return ""
	}

	family := *(*uint16)(unsafe.Pointer(address.Sockaddr))
	switch family {
	case addressFamilyIPv4:
		raw := (*syscall.RawSockaddrInet4)(unsafe.Pointer(address.Sockaddr))
		return net.IP(raw.Addr[:]).String()
	case addressFamilyIPv6:
		raw := (*syscall.RawSockaddrInet6)(unsafe.Pointer(address.Sockaddr))
		ip := net.IP(raw.Addr[:]).String()
		if raw.Scope_id != 0 {
			return fmt.Sprintf("%s%%%d", ip, raw.Scope_id)
		}
		return ip
	default:
		return ""
	}
}

func adapterOperStatus(status uint32) string {
	switch status {
	case 1:
		return "Up"
	case 2:
		return "Down"
	case 3:
		return "Testing"
	case 4:
		return "Unknown"
	case 5:
		return "Dormant"
	case 6:
		return "Not present"
	case 7:
		return "Lower layer down"
	default:
		return fmt.Sprintf("Status %d", status)
	}
}

func adapterAdminStatus(status uint32) string {
	switch status {
	case ifAdminStatusUp:
		return "Enabled"
	case ifAdminStatusDown:
		return "Disabled"
	default:
		return fmt.Sprintf("Admin %d", status)
	}
}

func formatMAC(address []byte) string {
	parts := make([]string, len(address))
	for i, value := range address {
		parts[i] = fmt.Sprintf("%02X", value)
	}
	return strings.Join(parts, ":")
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func firstString(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func maxU64(a, b uint64) uint64 {
	if a > b {
		return a
	}
	return b
}
