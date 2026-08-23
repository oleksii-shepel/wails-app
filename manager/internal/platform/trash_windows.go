//go:build windows

package platform

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	contracts "wails-app/internal/contracts"
)

var (
	shell32         = syscall.NewLazyDLL("shell32.dll")
	shFileOperation = shell32.NewProc("SHFileOperationW")
)

type shFileOpStruct struct {
	hwnd                  uintptr
	wFunc                 uint32
	_                     [4]byte
	pFrom                 uintptr
	pTo                   uintptr
	fFlags                uint16
	_                     [2]byte
	fAnyOperationsAborted int32
	hNameMappings         uintptr
	lpszProgressTitle     uintptr
}

const (
	foDelete        uint32 = 0x0003
	fofSilent       uint16 = 0x0004
	fofNoConfirm    uint16 = 0x0010
	fofAllowUndo    uint16 = 0x0040
	fofNoErrorUI    uint16 = 0x0400
	fofWantNukeWarn uint16 = 0x4000
)

func MoveToTrash(path string) error {
	path = normalizeShellTrashPath(path)
	if path == "" {
		return fmt.Errorf("trash: empty path")
	}

	from, err := syscall.UTF16FromString(path)
	if err != nil {
		return fmt.Errorf("trash: invalid path %q: %w", path, err)
	}
	from = append(from, 0)

	op := shFileOpStruct{
		wFunc:  foDelete,
		pFrom:  uintptr(unsafe.Pointer(&from[0])),
		fFlags: fofAllowUndo | fofNoConfirm | fofSilent | fofNoErrorUI | fofWantNukeWarn,
	}

	ret, _, _ := shFileOperation.Call(uintptr(unsafe.Pointer(&op)))
	if ret != 0 {
		return fmt.Errorf("trash: SHFileOperationW returned error code 0x%X (path=%q)", ret, path)
	}
	if op.fAnyOperationsAborted != 0 {
		return fmt.Errorf("trash: operation was aborted by the user or system")
	}
	return nil
}

func ListTrashEntries() ([]contracts.TrashEntry, error) {
	const script = `
$ErrorActionPreference = 'Stop'
$shell = New-Object -ComObject Shell.Application
$bin = $shell.Namespace(10)
$result = @()
foreach ($item in @($bin.Items())) {
  $from = [string]$item.ExtendedProperty('System.Recycle.DeletedFrom')
  $deleted = $item.ExtendedProperty('System.Recycle.DateDeleted')
  # ShellItem.Name is a display name and follows Explorer's "hide extensions"
  # setting. System.FileName preserves the actual basename, including extension.
  $fileName = [string]$item.ExtendedProperty('System.FileName')
  $name = if ($fileName) { $fileName } else { [string]$item.Name }
  $original = if ($from) { Join-Path $from $name } else { '' }
  $deletedText = if ($deleted -is [datetime]) { $deleted.ToString('o') } else { [string]$deleted }
  $result += [pscustomobject]@{
    id = [string]$item.Path
    name = [string]$name
    originalPath = [string]$original
    trashedPath = [string]$item.Path
    deletedAt = [string]$deletedText
    size = [int64]$item.Size
    isDir = [bool]$item.IsFolder
    canRestore = [bool]($original -ne '')
  }
}
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
ConvertTo-Json -InputObject $result -Compress -Depth 4
`
	output, err := runTrashPowerShell(script, nil)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(output) == "" {
		return []contracts.TrashEntry{}, nil
	}

	var entries []contracts.TrashEntry
	if err := json.Unmarshal([]byte(output), &entries); err != nil {
		return nil, fmt.Errorf("trash: parse recycle bin: %w", err)
	}
	return entries, nil
}

func RestoreTrashEntry(id string) (string, error) {
	const script = `
$ErrorActionPreference = 'Stop'
$target = $env:FM_TRASH_ID
$shell = New-Object -ComObject Shell.Application
$bin = $shell.Namespace(10)
$item = @($bin.Items()) | Where-Object { [string]$_.Path -eq $target } | Select-Object -First 1
if ($null -eq $item) { throw 'Recycle Bin item not found.' }
$from = [string]$item.ExtendedProperty('System.Recycle.DeletedFrom')
$fileName = [string]$item.ExtendedProperty('System.FileName')
$name = if ($fileName) { $fileName } else { [string]$item.Name }
$original = if ($from) { Join-Path $from $name } else { '' }
if (-not $original) { throw 'Original path is unavailable.' }
if (Test-Path -LiteralPath $original) { throw ('Restore destination already exists: ' + $original) }
$item.InvokeVerb('RESTORE')
Start-Sleep -Milliseconds 100
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
Write-Output $original
`
	output, err := runTrashPowerShell(script, map[string]string{"FM_TRASH_ID": id})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output), nil
}

func CopyTrashEntryTo(id, destinationDir string) (string, error) {
	destinationDir = normalizeShellTrashPath(destinationDir)
	if destinationDir == "" {
		return "", fmt.Errorf("trash copy: empty destination")
	}

	const script = `
$ErrorActionPreference = 'Stop'
$target = $env:FM_TRASH_ID
$destinationDir = $env:FM_TRASH_DESTINATION
if (-not (Test-Path -LiteralPath $destinationDir -PathType Container)) {
  throw ('Destination directory does not exist: ' + $destinationDir)
}
$shell = New-Object -ComObject Shell.Application
$bin = $shell.Namespace(10)
$item = @($bin.Items()) | Where-Object { [string]$_.Path -eq $target } | Select-Object -First 1
if ($null -eq $item) { throw 'Recycle Bin item not found.' }
$fileName = [string]$item.ExtendedProperty('System.FileName')
$name = if ($fileName) { $fileName } else { [string]$item.Name }
$source = [string]$item.Path
$destination = Join-Path $destinationDir $name
if (Test-Path -LiteralPath $destination) { throw ('Destination already exists: ' + $destination) }
Copy-Item -LiteralPath $source -Destination $destination -Recurse
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
Write-Output $destination
`
	output, err := runTrashPowerShell(script, map[string]string{
		"FM_TRASH_ID":          id,
		"FM_TRASH_DESTINATION": destinationDir,
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output), nil
}

func RestoreTrashEntryTo(id, destinationDir string) (string, error) {
	destination, err := CopyTrashEntryTo(id, destinationDir)
	if err != nil {
		return "", err
	}
	if err := DeleteTrashEntry(id); err != nil {
		return destination, fmt.Errorf(
			"trash restore: copied to %q but could not remove recycle-bin entry: %w",
			destination,
			err,
		)
	}
	return destination, nil
}

func DeleteTrashEntry(id string) error {
	const script = `
$ErrorActionPreference = 'Stop'
$target = $env:FM_TRASH_ID
$shell = New-Object -ComObject Shell.Application
$bin = $shell.Namespace(10)
$item = @($bin.Items()) | Where-Object { [string]$_.Path -eq $target } | Select-Object -First 1
if ($null -eq $item) { throw 'Recycle Bin item not found.' }
Remove-Item -LiteralPath ([string]$item.Path) -Force -Recurse
`
	_, err := runTrashPowerShell(script, map[string]string{"FM_TRASH_ID": id})
	return err
}

func EmptyTrash() error {
	const script = `
$ErrorActionPreference = 'Stop'
Clear-RecycleBin -Force -ErrorAction Stop
`
	_, err := runTrashPowerShell(script, nil)
	return err
}

func runTrashPowerShell(script string, extraEnv map[string]string) (string, error) {
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script)
	// This is an implementation detail of the Recycle Bin provider. Never flash
	// a console window when bin:// is opened from the GUI address bar.
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	cmd.Env = os.Environ()
	for key, value := range extraEnv {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("trash: powershell: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}

func normalizeShellTrashPath(path string) string {
	p := strings.TrimSpace(path)
	if p == "" {
		return ""
	}
	p = filepath.FromSlash(p)
	p = filepath.Clean(p)
	return strings.TrimPrefix(p, `\\?\`)
}