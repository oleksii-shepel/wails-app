//go:build !windows

package fontdata

func decodeEOT(data []byte, family string) ([]byte, bool) {
	return nil, false
}
