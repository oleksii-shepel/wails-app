//go:build windows

package pluginpackage

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	logonWithProfile    = 0x00000001
	startfUseStdHandles = 0x00000100
	startfUseShowWindow = 0x00000001
	handleFlagInherit   = 0x00000001
	createNoWindow      = 0x08000000
	swHide              = 0
)

var (
	advapi32                    = windows.NewLazySystemDLL("advapi32.dll")
	procCreateProcessWithTokenW = advapi32.NewProc("CreateProcessWithTokenW")
)

func startBackendForPrivilege(entry, privilege string) (*BackendProcess, error) {
	switch strings.TrimSpace(privilege) {
	case "", "normal":
		if !windows.GetCurrentProcessToken().IsElevated() {
			return startInheritedBackend(entry)
		}
		return startUnelevatedBackend(entry)
	case "elevated":
		if !windows.GetCurrentProcessToken().IsElevated() {
			return nil, fmt.Errorf("plugin requires an elevated Manager session")
		}
		return startInheritedBackend(entry)
	default:
		return nil, fmt.Errorf("unsupported backend privilege %q", privilege)
	}
}

func startUnelevatedBackend(entry string) (*BackendProcess, error) {
	token, err := desktopShellToken()
	if err != nil {
		return nil, fmt.Errorf("acquire standard-user shell token: %w", err)
	}
	defer token.Close()

	stdinRead, stdinWrite, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	stdoutRead, stdoutWrite, err := os.Pipe()
	if err != nil {
		_ = stdinRead.Close()
		_ = stdinWrite.Close()
		return nil, err
	}
	cleanup := func() {
		_ = stdinRead.Close()
		_ = stdinWrite.Close()
		_ = stdoutRead.Close()
		_ = stdoutWrite.Close()
	}

	if err := setHandleInherit(stdinRead, true); err != nil {
		cleanup()
		return nil, err
	}
	if err := setHandleInherit(stdoutWrite, true); err != nil {
		cleanup()
		return nil, err
	}
	_ = setHandleInherit(stdinWrite, false)
	_ = setHandleInherit(stdoutRead, false)

	appName, err := windows.UTF16PtrFromString(entry)
	if err != nil {
		cleanup()
		return nil, err
	}
	currentDir, err := windows.UTF16PtrFromString(filepath.Dir(entry))
	if err != nil {
		cleanup()
		return nil, err
	}

	var startup windows.StartupInfo
	startup.Cb = uint32(unsafe.Sizeof(startup))
	startup.Flags = startfUseStdHandles | startfUseShowWindow
	startup.StdInput = windows.Handle(stdinRead.Fd())
	startup.StdOutput = windows.Handle(stdoutWrite.Fd())
	startup.StdErr = windows.Handle(stdoutWrite.Fd())
	startup.ShowWindow = swHide

	var processInfo windows.ProcessInformation
	r1, _, callErr := procCreateProcessWithTokenW.Call(
		uintptr(token),
		uintptr(logonWithProfile),
		uintptr(unsafe.Pointer(appName)),
		0,
		uintptr(createNoWindow),
		0,
		uintptr(unsafe.Pointer(currentDir)),
		uintptr(unsafe.Pointer(&startup)),
		uintptr(unsafe.Pointer(&processInfo)),
	)
	if r1 == 0 {
		cleanup()
		return nil, fmt.Errorf("CreateProcessWithTokenW: %w", callErr)
	}

	_ = stdinRead.Close()
	_ = stdoutWrite.Close()
	_ = windows.CloseHandle(processInfo.Thread)

	processHandle := processInfo.Process
	process := &BackendProcess{
		stdin:  stdinWrite,
		reader: bufio.NewReader(stdoutRead),
	}
	process.stop = func() error {
		_ = stdinWrite.Close()
		_ = stdoutRead.Close()
		err := windows.TerminateProcess(processHandle, 0)
		_, _ = windows.WaitForSingleObject(processHandle, windows.INFINITE)
		_ = windows.CloseHandle(processHandle)
		return err
	}
	return process, nil
}

func desktopShellToken() (windows.Token, error) {
	var currentSession uint32
	if err := windows.ProcessIdToSessionId(uint32(os.Getpid()), &currentSession); err != nil {
		return 0, err
	}

	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return 0, err
	}
	defer windows.CloseHandle(snapshot)

	var pe windows.ProcessEntry32
	pe.Size = uint32(unsafe.Sizeof(pe))
	if err := windows.Process32First(snapshot, &pe); err != nil {
		return 0, err
	}

	for {
		if strings.EqualFold(windows.UTF16ToString(pe.ExeFile[:]), "explorer.exe") {
			var session uint32
			if windows.ProcessIdToSessionId(pe.ProcessID, &session) == nil && session == currentSession {
				process, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pe.ProcessID)
				if err == nil {
					var shellToken windows.Token
					err = windows.OpenProcessToken(
						process,
						windows.TOKEN_QUERY|windows.TOKEN_DUPLICATE|windows.TOKEN_ASSIGN_PRIMARY,
						&shellToken,
					)
					_ = windows.CloseHandle(process)
					if err == nil {
						var primary windows.Token
						err = windows.DuplicateTokenEx(
							shellToken,
							windows.MAXIMUM_ALLOWED,
							nil,
							windows.SecurityImpersonation,
							windows.TokenPrimary,
							&primary,
						)
						_ = shellToken.Close()
						if err == nil {
							return primary, nil
						}
					}
				}
			}
		}
		if err := windows.Process32Next(snapshot, &pe); err != nil {
			break
		}
	}
	return 0, fmt.Errorf("standard desktop shell token not found")
}

func setHandleInherit(file *os.File, inherit bool) error {
	flags := uint32(0)
	if inherit {
		flags = handleFlagInherit
	}
	return windows.SetHandleInformation(
		windows.Handle(file.Fd()),
		handleFlagInherit,
		flags,
	)
}
