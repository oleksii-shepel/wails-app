package app

type displayRect struct {
	left   int
	top    int
	right  int
	bottom int
}

func (r displayRect) width() int {
	return maxInt(0, r.right-r.left)
}

func (r displayRect) height() int {
	return maxInt(0, r.bottom-r.top)
}

func fitBoundsToDisplay(area displayRect, x, y, width, height int, hasPosition bool) (int, int, int, int) {
	if area.width() <= 0 || area.height() <= 0 {
		return x, y, width, height
	}

	width = clampWindowDimension(width, area.width())
	height = clampWindowDimension(height, area.height())

	if !hasPosition {
		return x, y, width, height
	}

	if x < area.left {
		x = area.left
	}
	if y < area.top {
		y = area.top
	}
	if x+width > area.right {
		x = area.right - width
	}
	if y+height > area.bottom {
		y = area.bottom - height
	}
	if x < area.left {
		x = area.left
	}
	if y < area.top {
		y = area.top
	}

	return x, y, width, height
}

func clampWindowDimension(value, limit int) int {
	if limit <= 0 {
		return value
	}
	if value > limit {
		return limit
	}
	return value
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
