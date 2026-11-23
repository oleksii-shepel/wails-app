package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
	"golang.org/x/sys/windows"

	"wails-app/internal/contracts"
)

func eachTrashItem(visit func(*ole.IDispatch) error) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
		var hr *ole.OleError
		if !errors.As(err, &hr) || hr.Code() != 1 {
			return fmt.Errorf("initialize Recycle Bin COM: %w", err)
		}
	}
	defer ole.CoUninitialize()
	shell, err := oleutil.CreateObject("Shell.Application")
	if err != nil {
		return err
	}
	defer shell.Release()
	dispatch, err := shell.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return err
	}
	defer dispatch.Release()
	bin, err := oleutil.CallMethod(dispatch, "NameSpace", int32(10))
	if err != nil {
		return fmt.Errorf("open Recycle Bin namespace: %w", err)
	}
	defer bin.Clear()
	if bin.ToIDispatch() == nil {
		return fmt.Errorf("Recycle Bin is unavailable")
	}
	items, err := oleutil.CallMethod(bin.ToIDispatch(), "Items")
	if err != nil {
		return fmt.Errorf("open Recycle Bin item collection: %w", err)
	}
	defer items.Clear()
	if items.ToIDispatch() == nil {
		return fmt.Errorf("Recycle Bin items are unavailable")
	}
	return visitTrashItems(items.ToIDispatch(), visit)
}

func visitTrashItems(items *ole.IDispatch, visit func(*ole.IDispatch) error) error {
	value, err := trashProperty(items, "Count")
	if err != nil {
		return fmt.Errorf("count Recycle Bin items: %w", err)
	}
	count, ok := value.(int32)
	if !ok {
		return fmt.Errorf("unexpected Recycle Bin item count type: %T", value)
	}
	for index := int32(0); index < count; index++ {
		item, err := oleutil.CallMethod(items, "Item", index)
		if err != nil {
			return fmt.Errorf("read Recycle Bin item %d: %w", index, err)
		}
		if dispatch := item.ToIDispatch(); dispatch != nil {
			err = visit(dispatch)
		}
		_ = item.Clear()
		if err != nil {
			return err
		}
	}
	return nil
}

func trashProperty(item *ole.IDispatch, name string) (any, error) {
	value, err := oleutil.GetProperty(item, name)
	if err != nil {
		return nil, fmt.Errorf("read Recycle Bin property %s: %w", name, err)
	}
	defer value.Clear()
	return value.Value(), nil
}

func extendedTrashProperty(item *ole.IDispatch, name string) any {
	value, err := oleutil.CallMethod(item, "ExtendedProperty", name)
	if err != nil {
		return nil
	}
	defer value.Clear()
	return value.Value()
}

func listTrash() ([]contracts.TrashEntry, error) {
	entries := []contracts.TrashEntry{}
	err := eachTrashItem(func(item *ole.IDispatch) error {
		idValue, err := trashProperty(item, "Path")
		if err != nil {
			return err
		}
		id, _ := idValue.(string)
		if id == "" {
			return nil
		}
		name, _ := extendedTrashProperty(item, "System.FileName").(string)
		if name == "" {
			value, err := trashProperty(item, "Name")
			if err != nil {
				return err
			}
			name, _ = value.(string)
		}
		parent, _ := extendedTrashProperty(item, "System.Recycle.DeletedFrom").(string)
		original := ""
		if filepath.IsAbs(parent) && filepath.IsLocal(name) && filepath.Base(name) == name {
			original = filepath.Join(parent, name)
		}
		value, err := trashProperty(item, "IsFolder")
		if err != nil {
			return err
		}
		dir, _ := value.(bool)
		var size int64
		switch n := extendedTrashProperty(item, "System.Size").(type) {
		case uint64:
			size = int64(n)
		case int64:
			size = n
		case uint32:
			size = int64(n)
		case int32:
			size = int64(n)
		}
		deleted := ""
		if date, ok := extendedTrashProperty(item, "System.Recycle.DateDeleted").(time.Time); ok {
			deleted = date.Format(time.RFC3339)
		}
		entries = append(entries, contracts.TrashEntry{ID: id, Name: name, TrashedPath: id, OriginalPath: original, IsDir: dir, Size: size, DeletedAt: deleted, CanRestore: original != ""})
		return nil
	})
	return entries, err
}

func restoreTrash(entry contracts.TrashEntry) (string, error) {
	if !entry.CanRestore {
		return "", fmt.Errorf("original path is unavailable for %s", entry.Name)
	}
	if _, err := os.Lstat(entry.OriginalPath); err == nil {
		return "", fmt.Errorf("restore destination already exists: %s", entry.OriginalPath)
	} else if !os.IsNotExist(err) {
		return "", err
	}
	found := errors.New("restore requested")
	err := eachTrashItem(func(item *ole.IDispatch) error {
		value, err := trashProperty(item, "Path")
		if err != nil {
			return err
		}
		id, _ := value.(string)
		if !strings.EqualFold(id, entry.ID) {
			return nil
		}
		result, err := oleutil.CallMethod(item, "InvokeVerb", "RESTORE")
		if err != nil {
			return err
		}
		if result != nil {
			_ = result.Clear()
		}
		return found
	})
	if err == nil {
		return "", fmt.Errorf("Recycle Bin item is no longer available: %s", entry.Name)
	}
	if !errors.Is(err, found) {
		return "", err
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Lstat(entry.OriginalPath); err == nil {
			return entry.OriginalPath, nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return "", fmt.Errorf("Windows has not confirmed restoration to %s", entry.OriginalPath)
}

func emptyTrash() error {
	proc := windows.NewLazySystemDLL("shell32.dll").NewProc("SHEmptyRecycleBinW")
	const noConfirmationOrProgressOrSound = 0x7
	result, _, _ := proc.Call(0, 0, noConfirmationOrProgressOrSound)
	if int32(result) < 0 {
		return fmt.Errorf("empty Recycle Bin: HRESULT 0x%08X", uint32(result))
	}
	return nil
}
