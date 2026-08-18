package runtime

import (
	"fmt"
	"testing"
	"time"

	contracts "wails-app/internal/contracts"
)

func syntheticDirEntries(count int) []contracts.FileInfo {
	entries := make([]contracts.FileInfo, count)
	for i := 0; i < count; i++ {
		entries[i] = contracts.FileInfo{
			Name: fmt.Sprintf("entry-%06d.txt", count-i),
			Path: fmt.Sprintf("/synthetic/entry-%06d.txt", count-i),
			Size: int64((i % 4096) + 1),
			Type: contracts.FileTypeFile,
		}
	}
	return entries
}

func TestDirectoryPipelineScaleRegression(t *testing.T) {
	for _, count := range []int{1_000, 10_000, 50_000, 100_000} {
		count := count
		t.Run(fmt.Sprintf("%d", count), func(t *testing.T) {
			started := time.Now()
			source := syntheticDirEntries(count)

			const batchSize = 256
			runs := make([][]contracts.FileInfo, 0, (count+batchSize-1)/batchSize)
			options := DirStreamOptions{
				SortField: "name",
				SortDirection: "asc",
				DirectoriesFirst: true,
			}

			for offset := 0; offset < len(source); offset += batchSize {
				end := offset + batchSize
				if end > len(source) {
					end = len(source)
				}
				run := append([]contracts.FileInfo(nil), source[offset:end]...)
				runs = append(runs, prepareStreamEntries(run, options))
			}

			windowStarted := time.Now()
			window := mergeSortedRunsWindow(runs, options, count/2, 1024)
			windowElapsed := time.Since(windowStarted)

			if got, want := len(window), minInt(1024, count-count/2); got != want {
				t.Fatalf("window length = %d, want %d", got, want)
			}
			if countRunEntries(runs) != count {
				t.Fatalf("indexed count = %d, want %d", countRunEntries(runs), count)
			}

			// Wide enough for slower CI, but strict enough to catch accidental
			// quadratic work or full-DOM-style processing in this pure data path.
			if elapsed := time.Since(started); elapsed > 5*time.Second {
				t.Fatalf("pipeline for %d entries took %s", count, elapsed)
			}
			if windowElapsed > 2*time.Second {
				t.Fatalf("window read for %d entries took %s", count, windowElapsed)
			}
		})
	}
}

func TestDirectoryPipelineWindowIsBoundedAt100K(t *testing.T) {
	const count = 100_000
	const batchSize = 256
	options := DirStreamOptions{
		SortField: "name",
		SortDirection: "asc",
		DirectoriesFirst: true,
	}

	source := syntheticDirEntries(count)
	runs := make([][]contracts.FileInfo, 0, (count+batchSize-1)/batchSize)
	for offset := 0; offset < count; offset += batchSize {
		end := offset + batchSize
		if end > count {
			end = count
		}
		runs = append(
			runs,
			prepareStreamEntries(append([]contracts.FileInfo(nil), source[offset:end]...), options),
		)
	}

	for _, offset := range []int{0, 25_000, 50_000, 98_976} {
		window := mergeSortedRunsWindow(runs, options, offset, 1024)
		if len(window) > 1024 {
			t.Fatalf("window at %d returned %d entries", offset, len(window))
		}
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
