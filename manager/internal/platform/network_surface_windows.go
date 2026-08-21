//go:build windows

package platform

import (
	"encoding/binary"
	"fmt"
	"net"
	"sort"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"

	contracts "wails-app/internal/contracts"
)

const (
	addressFamilyIPv4 = 2
	addressFamilyIPv6 = 23

	tcpTableOwnerPIDAll = 5
	udpTableOwnerPID    = 1

	errorInsufficientBuffer = syscall.Errno(122)

	maxPreferredLength = 0xffffffff
	nerrSuccess        = 0
	errorMoreData      = 234
	svTypeAll          = 0xffffffff
)

var (
	iphlpapiDLL             = windows.NewLazySystemDLL("iphlpapi.dll")
	procGetExtendedTcpTable = iphlpapiDLL.NewProc("GetExtendedTcpTable")
	procGetExtendedUdpTable = iphlpapiDLL.NewProc("GetExtendedUdpTable")

	netapi32DLL          = windows.NewLazySystemDLL("netapi32.dll")
	procNetServerEnum    = netapi32DLL.NewProc("NetServerEnum")
	procNetShareEnum     = netapi32DLL.NewProc("NetShareEnum")
	procNetApiBufferFree = netapi32DLL.NewProc("NetApiBufferFree")
)

type mibTCPRowOwnerPID struct {
	State      uint32
	LocalAddr  uint32
	LocalPort  uint32
	RemoteAddr uint32
	RemotePort uint32
	OwningPID  uint32
}

type mibTCP6RowOwnerPID struct {
	LocalAddr     [16]byte
	LocalScopeID  uint32
	LocalPort     uint32
	RemoteAddr    [16]byte
	RemoteScopeID uint32
	RemotePort    uint32
	State         uint32
	OwningPID     uint32
}

type mibUDPRowOwnerPID struct {
	LocalAddr uint32
	LocalPort uint32
	OwningPID uint32
}

type mibUDP6RowOwnerPID struct {
	LocalAddr    [16]byte
	LocalScopeID uint32
	LocalPort    uint32
	OwningPID    uint32
}

type serverInfo100 struct {
	PlatformID uint32
	Name       *uint16
}

type shareInfo1 struct {
	NetName *uint16
	Type    uint32
	Remark  *uint16
}

// ListNetworkComputers discovers visible Windows network servers/workstations
// through NetServerEnum. It does not spawn a shell or PowerShell.
func ListNetworkComputers() ([]contracts.SystemSurfaceRecord, error) {
	var buffer *byte
	var entriesRead uint32
	var totalEntries uint32
	var resumeHandle uint32
	records := make([]contracts.SystemSurfaceRecord, 0, 32)

	for {
		result, _, callErr := procNetServerEnum.Call(
			0,
			100,
			uintptr(unsafe.Pointer(&buffer)),
			maxPreferredLength,
			uintptr(unsafe.Pointer(&entriesRead)),
			uintptr(unsafe.Pointer(&totalEntries)),
			svTypeAll,
			0,
			uintptr(unsafe.Pointer(&resumeHandle)),
		)

		if buffer != nil {
			base := uintptr(unsafe.Pointer(buffer))
			rowSize := unsafe.Sizeof(serverInfo100{})
			for i := uint32(0); i < entriesRead; i++ {
				row := (*serverInfo100)(unsafe.Pointer(base + uintptr(i)*rowSize))
				name := strings.TrimSpace(windows.UTF16PtrToString(row.Name))
				if name == "" {
					continue
				}
				records = append(records, contracts.SystemSurfaceRecord{
					ID:   strings.ToLower(name),
					Name: name,
					Fields: map[string]any{
						"kind":   "Computer",
						"status": "Discovered",
						"remote": `\\` + name,
					},
				})
			}
			procNetApiBufferFree.Call(uintptr(unsafe.Pointer(buffer)))
			buffer = nil
		}

		if result == nerrSuccess {
			break
		}
		if result != errorMoreData {
			return records, windowsError(result, callErr)
		}
	}

	sort.SliceStable(records, func(i, j int) bool {
		return strings.ToLower(records[i].Name) < strings.ToLower(records[j].Name)
	})
	return records, nil
}

// ListNetworkShares lists non-administrative shares exposed by one computer.
// Returned remote paths are ordinary UNC paths, allowing DirectoryPane to hand
// browsing back to the filesystem source when the user opens a share.
func ListNetworkShares(computer string) ([]contracts.SystemSurfaceRecord, error) {
	computer = strings.TrimSpace(strings.TrimPrefix(computer, `\\`))
	if computer == "" {
		return []contracts.SystemSurfaceRecord{}, nil
	}

	serverName, err := windows.UTF16PtrFromString(`\\` + computer)
	if err != nil {
		return nil, err
	}

	var buffer *byte
	var entriesRead uint32
	var totalEntries uint32
	var resumeHandle uint32
	records := make([]contracts.SystemSurfaceRecord, 0, 16)

	for {
		result, _, callErr := procNetShareEnum.Call(
			uintptr(unsafe.Pointer(serverName)),
			1,
			uintptr(unsafe.Pointer(&buffer)),
			maxPreferredLength,
			uintptr(unsafe.Pointer(&entriesRead)),
			uintptr(unsafe.Pointer(&totalEntries)),
			uintptr(unsafe.Pointer(&resumeHandle)),
		)

		if buffer != nil {
			base := uintptr(unsafe.Pointer(buffer))
			rowSize := unsafe.Sizeof(shareInfo1{})
			for i := uint32(0); i < entriesRead; i++ {
				row := (*shareInfo1)(unsafe.Pointer(base + uintptr(i)*rowSize))
				name := strings.TrimSpace(windows.UTF16PtrToString(row.NetName))
				if name == "" || strings.HasSuffix(name, "$") {
					continue
				}
				remark := strings.TrimSpace(windows.UTF16PtrToString(row.Remark))
				unc := `\\` + computer + `\` + name
				records = append(records, contracts.SystemSurfaceRecord{
					ID:   strings.ToLower(computer + "/" + name),
					Name: name,
					Fields: map[string]any{
						"kind":      "Share",
						"status":    shareTypeName(row.Type),
						"remote":    unc,
						"remark":    remark,
						"shareType": row.Type,
					},
				})
			}
			procNetApiBufferFree.Call(uintptr(unsafe.Pointer(buffer)))
			buffer = nil
		}

		if result == nerrSuccess {
			break
		}
		if result != errorMoreData {
			return records, windowsError(result, callErr)
		}
	}

	sort.SliceStable(records, func(i, j int) bool {
		return strings.ToLower(records[i].Name) < strings.ToLower(records[j].Name)
	})
	return records, nil
}

func shareTypeName(value uint32) string {
	switch value & 0xff {
	case 0:
		return "Disk"
	case 1:
		return "Printer"
	case 2:
		return "Device"
	case 3:
		return "IPC"
	default:
		return "Share"
	}
}

// ListNetworkSurface returns the Manager-owned network:// rows without launching
// PowerShell, cmd.exe, netstat, or any other child process.
func ListNetworkSurface() ([]contracts.SystemSurfaceRecord, error) {
	records := make([]contracts.SystemSurfaceRecord, 0, 128)

	adapters, adapterErr := networkAdapterRecords()
	records = append(records, adapters...)

	tcp4, tcp4Err := tcpRecords(addressFamilyIPv4)
	records = append(records, tcp4...)

	tcp6, tcp6Err := tcpRecords(addressFamilyIPv6)
	records = append(records, tcp6...)

	udp4, udp4Err := udpRecords(addressFamilyIPv4)
	records = append(records, udp4...)

	udp6, udp6Err := udpRecords(addressFamilyIPv6)
	records = append(records, udp6...)

	// A partially available Network pane is more useful than failing the whole
	// surface because one Windows table is unavailable. Return an error only if
	// every native source failed.
	errors := compactErrors(adapterErr, tcp4Err, tcp6Err, udp4Err, udp6Err)
	if len(records) == 0 && len(errors) > 0 {
		return nil, fmt.Errorf("list network: %s", strings.Join(errors, "; "))
	}

	sort.SliceStable(records, func(i, j int) bool {
		ki := fmt.Sprint(records[i].Fields["kind"])
		kj := fmt.Sprint(records[j].Fields["kind"])
		if ki != kj {
			return networkKindOrder(ki) < networkKindOrder(kj)
		}
		return strings.ToLower(records[i].Name) < strings.ToLower(records[j].Name)
	})

	return records, nil
}

func networkAdapterRecords() ([]contracts.SystemSurfaceRecord, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("interfaces: %w", err)
	}

	metadata, metadataErr := NetworkAdapterMetadata()
	records := make([]contracts.SystemSurfaceRecord, 0, len(interfaces))
	for _, iface := range interfaces {
		addresses, _ := iface.Addrs()
		ipAddresses := make([]string, 0, len(addresses))
		for _, address := range addresses {
			if value := strings.TrimSpace(address.String()); value != "" {
				ipAddresses = append(ipAddresses, value)
			}
		}

		status := "Down"
		if iface.Flags&net.FlagUp != 0 {
			status = "Up"
		}

		fields := map[string]any{
			"kind":      "Adapter",
			"status":    status,
			"local":     strings.Join(ipAddresses, " · "),
			"remote":    "",
			"index":     iface.Index,
			"mtu":       iface.MTU,
			"flags":     iface.Flags.String(),
			"addresses": ipAddresses,
			"address":   firstString(ipAddresses),
			"mac":       iface.HardwareAddr.String(),
		}

		if rich, ok := metadata[uint32(iface.Index)]; ok {
			for key, value := range rich {
				fields[key] = value
			}
			if value := strings.TrimSpace(fmt.Sprint(rich["friendlyName"])); value != "" {
				// Preserve the Go interface name as interfaceName while showing
				// the friendly Windows adapter name in the row.
				fields["interfaceName"] = iface.Name
			}
			if richAddresses, ok := rich["addresses"].([]string); ok && len(richAddresses) > 0 {
				fields["local"] = strings.Join(richAddresses, " · ")
			}
		}

		name := iface.Name
		if rich, ok := metadata[uint32(iface.Index)]; ok {
			if friendly := strings.TrimSpace(fmt.Sprint(rich["friendlyName"])); friendly != "" {
				name = friendly
			}
		}

		records = append(records, contracts.SystemSurfaceRecord{
			ID:     fmt.Sprintf("adapter:%d", iface.Index),
			Name:   name,
			Fields: fields,
		})
	}

	if len(records) == 0 && metadataErr != nil {
		return nil, metadataErr
	}
	return records, nil
}

func tcpRecords(family uint32) ([]contracts.SystemSurfaceRecord, error) {
	buffer, err := extendedTableBuffer(procGetExtendedTcpTable, family, tcpTableOwnerPIDAll)
	if err != nil {
		return nil, fmt.Errorf("tcp table af=%d: %w", family, err)
	}
	if len(buffer) < 4 {
		return []contracts.SystemSurfaceRecord{}, nil
	}

	count := binary.LittleEndian.Uint32(buffer[:4])
	offset := uintptr(4)

	switch family {
	case addressFamilyIPv4:
		rowSize := unsafe.Sizeof(mibTCPRowOwnerPID{})
		records := make([]contracts.SystemSurfaceRecord, 0, count)
		for i := uint32(0); i < count; i++ {
			if offset+rowSize > uintptr(len(buffer)) {
				break
			}
			row := *(*mibTCPRowOwnerPID)(unsafe.Pointer(&buffer[offset]))
			offset += rowSize

			local := endpoint(ipv4FromDWORD(row.LocalAddr), portFromDWORD(row.LocalPort))
			remote := endpoint(ipv4FromDWORD(row.RemoteAddr), portFromDWORD(row.RemotePort))
			records = append(records, networkSocketRecord(
				"TCP",
				tcpState(row.State),
				local,
				remote,
				row.OwningPID,
				"4",
			))
		}
		return records, nil

	case addressFamilyIPv6:
		rowSize := unsafe.Sizeof(mibTCP6RowOwnerPID{})
		records := make([]contracts.SystemSurfaceRecord, 0, count)
		for i := uint32(0); i < count; i++ {
			if offset+rowSize > uintptr(len(buffer)) {
				break
			}
			row := *(*mibTCP6RowOwnerPID)(unsafe.Pointer(&buffer[offset]))
			offset += rowSize

			local := endpoint(ipv6WithScope(row.LocalAddr, row.LocalScopeID), portFromDWORD(row.LocalPort))
			remote := endpoint(ipv6WithScope(row.RemoteAddr, row.RemoteScopeID), portFromDWORD(row.RemotePort))
			records = append(records, networkSocketRecord(
				"TCP",
				tcpState(row.State),
				local,
				remote,
				row.OwningPID,
				"6",
			))
		}
		return records, nil
	default:
		return nil, fmt.Errorf("unsupported address family %d", family)
	}
}

func udpRecords(family uint32) ([]contracts.SystemSurfaceRecord, error) {
	buffer, err := extendedTableBuffer(procGetExtendedUdpTable, family, udpTableOwnerPID)
	if err != nil {
		return nil, fmt.Errorf("udp table af=%d: %w", family, err)
	}
	if len(buffer) < 4 {
		return []contracts.SystemSurfaceRecord{}, nil
	}

	count := binary.LittleEndian.Uint32(buffer[:4])
	offset := uintptr(4)

	switch family {
	case addressFamilyIPv4:
		rowSize := unsafe.Sizeof(mibUDPRowOwnerPID{})
		records := make([]contracts.SystemSurfaceRecord, 0, count)
		for i := uint32(0); i < count; i++ {
			if offset+rowSize > uintptr(len(buffer)) {
				break
			}
			row := *(*mibUDPRowOwnerPID)(unsafe.Pointer(&buffer[offset]))
			offset += rowSize

			local := endpoint(ipv4FromDWORD(row.LocalAddr), portFromDWORD(row.LocalPort))
			records = append(records, networkSocketRecord("UDP", "Listening", local, "", row.OwningPID, "4"))
		}
		return records, nil

	case addressFamilyIPv6:
		rowSize := unsafe.Sizeof(mibUDP6RowOwnerPID{})
		records := make([]contracts.SystemSurfaceRecord, 0, count)
		for i := uint32(0); i < count; i++ {
			if offset+rowSize > uintptr(len(buffer)) {
				break
			}
			row := *(*mibUDP6RowOwnerPID)(unsafe.Pointer(&buffer[offset]))
			offset += rowSize

			local := endpoint(ipv6WithScope(row.LocalAddr, row.LocalScopeID), portFromDWORD(row.LocalPort))
			records = append(records, networkSocketRecord("UDP", "Listening", local, "", row.OwningPID, "6"))
		}
		return records, nil
	default:
		return nil, fmt.Errorf("unsupported address family %d", family)
	}
}

func extendedTableBuffer(proc *windows.LazyProc, family, tableClass uint32) ([]byte, error) {
	var size uint32

	result, _, callErr := proc.Call(
		0,
		uintptr(unsafe.Pointer(&size)),
		0,
		uintptr(family),
		uintptr(tableClass),
		0,
	)
	if result != 0 && syscall.Errno(result) != errorInsufficientBuffer {
		return nil, windowsError(result, callErr)
	}
	if size == 0 {
		return []byte{}, nil
	}

	buffer := make([]byte, size)
	result, _, callErr = proc.Call(
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(unsafe.Pointer(&size)),
		0,
		uintptr(family),
		uintptr(tableClass),
		0,
	)
	if result != 0 {
		return nil, windowsError(result, callErr)
	}
	return buffer[:size], nil
}

func windowsError(result uintptr, callErr error) error {
	if result != 0 {
		return syscall.Errno(result)
	}
	if callErr != nil && callErr != windows.ERROR_SUCCESS {
		return callErr
	}
	return syscall.EINVAL
}

func networkSocketRecord(
	kind, status, local, remote string,
	pid uint32,
	ipVersion string,
) contracts.SystemSurfaceRecord {
	name := kind + " " + local
	id := fmt.Sprintf(
		"%s%s:%d:%s:%s",
		strings.ToLower(kind),
		ipVersion,
		pid,
		local,
		remote,
	)
	return contracts.SystemSurfaceRecord{
		ID:   id,
		Name: name,
		Fields: map[string]any{
			"kind":      kind,
			"status":    status,
			"local":     local,
			"remote":    remote,
			"pid":       pid,
			"ipVersion": ipVersion,
		},
	}
}

func ipv4FromDWORD(value uint32) string {
	ip := net.IPv4(
		byte(value),
		byte(value>>8),
		byte(value>>16),
		byte(value>>24),
	)
	return ip.String()
}

func ipv6WithScope(address [16]byte, scopeID uint32) string {
	ip := net.IP(address[:]).String()
	if scopeID == 0 {
		return ip
	}
	return fmt.Sprintf("%s%%%d", ip, scopeID)
}

func portFromDWORD(value uint32) uint16 {
	raw := uint16(value)
	return raw<<8 | raw>>8
}

func endpoint(address string, port uint16) string {
	if strings.Contains(address, ":") {
		return fmt.Sprintf("[%s]:%d", address, port)
	}
	return fmt.Sprintf("%s:%d", address, port)
}

func tcpState(state uint32) string {
	switch state {
	case 1:
		return "Closed"
	case 2:
		return "Listen"
	case 3:
		return "SynSent"
	case 4:
		return "SynReceived"
	case 5:
		return "Established"
	case 6:
		return "FinWait1"
	case 7:
		return "FinWait2"
	case 8:
		return "CloseWait"
	case 9:
		return "Closing"
	case 10:
		return "LastAck"
	case 11:
		return "TimeWait"
	case 12:
		return "DeleteTCB"
	default:
		return fmt.Sprintf("State %d", state)
	}
}

func compactErrors(errors ...error) []string {
	result := make([]string, 0, len(errors))
	for _, err := range errors {
		if err != nil {
			result = append(result, err.Error())
		}
	}
	return result
}

func networkKindOrder(kind string) int {
	switch kind {
	case "Adapter":
		return 0
	case "TCP":
		return 1
	case "UDP":
		return 2
	default:
		return 3
	}
}
