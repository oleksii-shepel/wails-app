package runtime

import (
	"context"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func BindNativeFileDrop(ctx context.Context, onDrop func(x, y int, paths []string)) {
	wailsruntime.EventsOn(ctx, "wails:file-drop", func(optionalData ...interface{}) {
		x, y, paths, ok := parseNativeFileDrop(optionalData)
		if !ok {
			return
		}

		onDrop(x, y, paths)
	})
}

func parseNativeFileDrop(optionalData []interface{}) (int, int, []string, bool) {
	if len(optionalData) == 0 {
		return 0, 0, nil, false
	}

	data, ok := optionalData[0].(map[string]interface{})
	if !ok {
		return 0, 0, nil, false
	}

	x := 0
	y := 0
	if xVal, ok := data["x"].(float64); ok {
		x = int(xVal)
	}
	if yVal, ok := data["y"].(float64); ok {
		y = int(yVal)
	}

	paths := make([]string, 0)
	if pathsVal, ok := data["paths"].([]interface{}); ok {
		for _, pathValue := range pathsVal {
			if path, ok := pathValue.(string); ok {
				paths = append(paths, path)
			}
		}
	}

	return x, y, paths, true
}