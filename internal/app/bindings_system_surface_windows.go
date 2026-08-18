//go:build windows

package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	contracts "wails-app/internal/contracts"
)

func (a *App) SystemSurfaceList(kind string) ([]contracts.SystemSurfaceRecord, error) {
	script, ok := systemSurfaceScript(strings.ToLower(strings.TrimSpace(kind)))
	if !ok {
		return nil, fmt.Errorf("unsupported system surface %q", kind)
	}
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return nil, fmt.Errorf("list %s: %s", kind, message)
	}
	trimmed := bytes.TrimSpace(out)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return []contracts.SystemSurfaceRecord{}, nil
	}
	var raw any
	if err := json.Unmarshal(trimmed, &raw); err != nil {
		return nil, fmt.Errorf("decode %s surface: %w", kind, err)
	}
	items, ok := raw.([]any)
	if !ok {
		items = []any{raw}
	}
	result := make([]contracts.SystemSurfaceRecord, 0, len(items))
	for index, item := range items {
		object, ok := item.(map[string]any)
		if !ok {
			continue
		}
		id := fmt.Sprint(object["id"])
		if id == "<nil>" || id == "" {
			id = fmt.Sprint(index)
		}
		name := fmt.Sprint(object["name"])
		if name == "<nil>" || name == "" {
			name = id
		}
		fields := map[string]any{}
		if value, ok := object["fields"].(map[string]any); ok {
			fields = value
		}
		result = append(result, contracts.SystemSurfaceRecord{ID: id, Name: name, Fields: fields})
	}
	return result, nil
}

func systemSurfaceScript(kind string) (string, bool) {
	const prefix = `$ErrorActionPreference='Stop'; [Console]::OutputEncoding=[Text.UTF8Encoding]::new(); `
	scripts := map[string]string{
		"tasks":            `@(Get-ScheduledTask | ForEach-Object { $i=$_; $info=$null; try {$info=Get-ScheduledTaskInfo -TaskName $i.TaskName -TaskPath $i.TaskPath -ErrorAction Stop} catch {}; [pscustomobject]@{id=($i.TaskPath+$i.TaskName);name=$i.TaskName;fields=@{state=[string]$i.State;path=$i.TaskPath;nextRun=if($info){[string]$info.NextRunTime}else{''}}} }) | ConvertTo-Json -Depth 5 -Compress`,
		"services":         `@(Get-CimInstance Win32_Service | ForEach-Object { [pscustomobject]@{id=$_.Name;name=$_.Name;fields=@{status=$_.State;startType=$_.StartMode;displayName=$_.DisplayName;path=$_.PathName}} }) | ConvertTo-Json -Depth 5 -Compress`,
		"processes":        `@(Get-Process | ForEach-Object { $p=$_; $path=''; try {$path=$p.Path} catch {}; [pscustomobject]@{id=[string]$p.Id;name=$p.ProcessName;fields=@{pid=$p.Id;cpu=if($p.CPU){[math]::Round($p.CPU,2)}else{0};workingSet=$p.WorkingSet64;size=$p.WorkingSet64;path=$path}} }) | ConvertTo-Json -Depth 5 -Compress`,
		"devices":          `@(Get-PnpDevice | ForEach-Object { [pscustomobject]@{id=$_.InstanceId;name=if($_.FriendlyName){$_.FriendlyName}else{$_.InstanceId};fields=@{status=[string]$_.Status;class=$_.Class;instanceId=$_.InstanceId}} }) | ConvertTo-Json -Depth 5 -Compress`,
		"network":          `@((Get-NetAdapter | ForEach-Object { [pscustomobject]@{id=('adapter:'+[string]$_.InterfaceGuid);name=$_.Name;fields=@{kind='Adapter';status=[string]$_.Status;local=$_.MacAddress;remote='';speed=$_.LinkSpeed}} }), (Get-NetTCPConnection -ErrorAction SilentlyContinue | ForEach-Object { [pscustomobject]@{id=('tcp:'+$_.OwningProcess+':'+$_.LocalAddress+':'+$_.LocalPort+':'+$_.RemoteAddress+':'+$_.RemotePort);name=('TCP '+$_.LocalAddress+':'+$_.LocalPort);fields=@{kind='TCP';status=[string]$_.State;local=($_.LocalAddress+':'+$_.LocalPort);remote=($_.RemoteAddress+':'+$_.RemotePort);pid=$_.OwningProcess}} })) | ForEach-Object {$_} | ConvertTo-Json -Depth 5 -Compress`,
		"environment":      `@((Get-ChildItem Env: | ForEach-Object { [pscustomobject]@{id=('process:'+$_.Name);name=$_.Name;fields=@{scope='Process';value=$_.Value}} }), (Get-ItemProperty 'HKCU:\Environment' -ErrorAction SilentlyContinue | ForEach-Object {$o=$_; $o.PSObject.Properties | Where-Object {$_.Name -notmatch '^PS'} | ForEach-Object {[pscustomobject]@{id=('user:'+$_.Name);name=$_.Name;fields=@{scope='User';value=[string]$_.Value}}}}), (Get-ItemProperty 'HKLM:\SYSTEM\CurrentControlSet\Control\Session Manager\Environment' -ErrorAction SilentlyContinue | ForEach-Object {$o=$_; $o.PSObject.Properties | Where-Object {$_.Name -notmatch '^PS'} | ForEach-Object {[pscustomobject]@{id=('machine:'+$_.Name);name=$_.Name;fields=@{scope='Machine';value=[string]$_.Value}}}})) | ForEach-Object {$_} | ConvertTo-Json -Depth 5 -Compress`,
		"shell-extensions": `$roots=@(@{scope='Machine';root='HKLM:\Software\Classes'},@{scope='User';root='HKCU:\Software\Classes'}); $targets=@('*\shellex\ContextMenuHandlers','Directory\shellex\ContextMenuHandlers','Directory\Background\shellex\ContextMenuHandlers','AllFileSystemObjects\shellex\ContextMenuHandlers'); @($roots | ForEach-Object {$r=$_; $targets | ForEach-Object {$t=$_; Get-ChildItem ($r.root+'\'+$t) -ErrorAction SilentlyContinue | ForEach-Object {$v=(Get-Item $_.PSPath).GetValue(''); [pscustomobject]@{id=($r.scope+':'+$t+':'+$_.PSChildName);name=$_.PSChildName;fields=@{scope=$r.scope;target=$t;clsid=[string]$v}}}}}) | ConvertTo-Json -Depth 5 -Compress`,
	}
	script, ok := scripts[kind]
	if !ok {
		return "", false
	}
	return prefix + script, true
}
