package app

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"wails-app/internal/pathref"
	"wails-app/vfs/archive/cache"
)

const segmentedPreviewSegmentSeconds = 2.0

const segmentedVideoCacheTTL = 5 * time.Minute

const segmentedVideoMaxResidentSegments = 24

type segmentedVideoManifest struct {
	Mode            string                       `json:"mode"`
	Format          string                       `json:"format"`
	DurationSeconds float64                      `json:"durationSeconds"`
	SegmentDuration float64                      `json:"segmentDuration"`
	TotalSegments   int                          `json:"totalSegments"`
	InitSegmentURL  string                       `json:"initSegmentUrl"`
	Segments        []segmentedVideoManifestItem `json:"segments"`
}

type segmentedVideoManifestItem struct {
	Index           int     `json:"index"`
	StartTime       float64 `json:"startTime"`
	DurationSeconds float64 `json:"durationSeconds"`
	URL             string  `json:"url"`
}

type segmentedVideoCacheEntry struct {
	CacheKey        string                `json:"cacheKey"`
	Format          string                `json:"format"`
	DurationSeconds float64               `json:"durationSeconds"`
	SegmentDuration float64               `json:"segmentDuration"`
	TotalSegments   int                   `json:"totalSegments"`
	SegmentRanges   []segmentedVideoRange `json:"-"`
}

type segmentedVideoPlaybackCacheEntry struct {
	mu          sync.Mutex
	manifest    segmentedVideoCacheEntry
	nativePath  string
	initSegment []byte
	segments    map[int][]byte
	segmentSeen map[int]time.Time
	inflight    map[int]*segmentedVideoSegmentBuild
	createdAt   time.Time
	lastAccess  time.Time
	cleanup     func() // called when evicted; used for temp file cleanup
}

type segmentedVideoPlaybackBuild struct {
	done    chan struct{}
	entry   *segmentedVideoPlaybackCacheEntry
	err     error
	cancel  context.CancelFunc
	waiters int
}

type segmentedVideoSegmentBuild struct {
	done    chan struct{}
	init    []byte
	segment []byte
	err     error
	cancel  context.CancelFunc
	waiters int
}

type segmentedVideoPlaybackCache struct {
	mu       sync.Mutex
	entries  map[string]*segmentedVideoPlaybackCacheEntry
	inflight map[string]*segmentedVideoPlaybackBuild
}

var segmentedPlaybackCache = newSegmentedVideoPlaybackCache()

func serveSegmentManifest(w http.ResponseWriter, r *http.Request, publicPath string) {
	entry, err := ensureSegmentedVideoCacheEntry(r.Context(), publicPath)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	manifest := buildSegmentedVideoManifest(requestBaseURL(r), entry)
	setCORSHeaders(w)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(manifest)
}

func serveSegmentManifestByKey(w http.ResponseWriter, r *http.Request, cacheKey string) {
	entry, ok := segmentedPlaybackCache.Get(cacheKey)
	if !ok {
		http.Error(w, "segment cache entry not found or expired", http.StatusNotFound)
		return
	}
	manifest := buildSegmentedVideoManifest(requestBaseURL(r), entry.manifest)
	setCORSHeaders(w)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(manifest)
}

func serveSegmentFile(w http.ResponseWriter, r *http.Request, cacheKey string, fileName string) {
	content, modTime, ok, err := segmentedPlaybackCache.File(r.Context(), cacheKey, fileName)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	setCORSHeaders(w)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "video/mp4")
	http.ServeContent(w, r, fileName, modTime, bytes.NewReader(content))
}

func segmentedVideoFileName(index int) string {
	return fmt.Sprintf("segment_%05d.m4s", index+1)
}

func segmentManifestURLForFile(baseURL, publicPath string) string {
	return joinStreamBaseURL(baseURL) + streamPath + "?path=" + urlEncode(publicPath) + "&segmentManifest=1"
}

func segmentManifestURLForKey(baseURL, cacheKey string) string {
	return joinStreamBaseURL(baseURL) + streamPath + "?segmentKey=" + urlEncode(cacheKey) + "&segmentManifest=1"
}

func segmentInitURL(baseURL, cacheKey string) string {
	return joinStreamBaseURL(baseURL) + streamPath + "?segmentKey=" + urlEncode(cacheKey) + "&segmentInit=1"
}

func segmentFileURL(baseURL, cacheKey string, index int) string {
	return joinStreamBaseURL(baseURL) + streamPath + "?segmentKey=" + urlEncode(cacheKey) + "&segmentIndex=" + urlEncode(strconv.Itoa(index))
}

func buildSegmentedVideoManifest(baseURL string, entry segmentedVideoCacheEntry) segmentedVideoManifest {
	if len(entry.SegmentRanges) > 0 {
		segments := make([]segmentedVideoManifestItem, 0, len(entry.SegmentRanges))
		for index, segmentRange := range entry.SegmentRanges {
			segments = append(segments, segmentedVideoManifestItem{
				Index:           index,
				StartTime:       segmentRange.StartSeconds,
				DurationSeconds: segmentRange.DurationSeconds,
				URL:             segmentFileURL(baseURL, entry.CacheKey, index),
			})
		}

		return segmentedVideoManifest{
			Mode:            string(VideoPlaybackModeSegmented),
			Format:          entry.Format,
			DurationSeconds: entry.DurationSeconds,
			SegmentDuration: entry.SegmentDuration,
			TotalSegments:   len(entry.SegmentRanges),
			InitSegmentURL:  segmentInitURL(baseURL, entry.CacheKey),
			Segments:        segments,
		}
	}

	segments := make([]segmentedVideoManifestItem, 0, entry.TotalSegments)
	for index := 0; index < entry.TotalSegments; index++ {
		startTime := float64(index) * entry.SegmentDuration
		segmentDuration := entry.SegmentDuration
		if remaining := entry.DurationSeconds - startTime; remaining < segmentDuration {
			segmentDuration = remaining
		}
		if segmentDuration < 0 {
			segmentDuration = 0
		}
		segments = append(segments, segmentedVideoManifestItem{
			Index:           index,
			StartTime:       startTime,
			DurationSeconds: segmentDuration,
			URL:             segmentFileURL(baseURL, entry.CacheKey, index),
		})
	}

	return segmentedVideoManifest{
		Mode:            string(VideoPlaybackModeSegmented),
		Format:          entry.Format,
		DurationSeconds: entry.DurationSeconds,
		SegmentDuration: entry.SegmentDuration,
		TotalSegments:   entry.TotalSegments,
		InitSegmentURL:  segmentInitURL(baseURL, entry.CacheKey),
		Segments:        segments,
	}
}

func ensureSegmentedVideoCacheEntry(ctx context.Context, publicPath string) (segmentedVideoCacheEntry, error) {
	publicPath = pathref.Normalize(publicPath)
	nativePath := pathref.ToNativePath(publicPath)
	if strings.Contains(nativePath, "..") {
		return segmentedVideoCacheEntry{}, fmt.Errorf("invalid path")
	}

	fi, err := os.Stat(nativePath)
	if err != nil {
		return segmentedVideoCacheEntry{}, err
	}

	cacheKey := segmentedVideoCacheKey(nativePath, fi)
	entry, err := segmentedPlaybackCache.Ensure(ctx, cacheKey, func(buildCtx context.Context) (*segmentedVideoPlaybackCacheEntry, error) {
		return buildSegmentedVideoCache(buildCtx, nativePath, cacheKey)
	})
	if err != nil {
		return segmentedVideoCacheEntry{}, err
	}
	return entry.manifest, nil
}

func archiveSegmentedVideoCacheKey(fsPath string, nestedChain []string, entryPath string) string {
	parts := make([]string, 0, 2+len(nestedChain))
	parts = append(parts, fsPath)
	parts = append(parts, nestedChain...)
	parts = append(parts, entryPath)
	hash := sha1.Sum([]byte(strings.Join(parts, "|")))
	return "arch-" + fmt.Sprintf("%x", hash)
}

func ensureArchiveSegmentedVideoCacheEntry(ctx context.Context, archCache *cache.Cache, fsPath string, nestedChain []string, entryPath string) (segmentedVideoCacheEntry, error) {
	if archCache == nil {
		return segmentedVideoCacheEntry{}, fmt.Errorf("archive cache unavailable")
	}
	cacheKey := archiveSegmentedVideoCacheKey(fsPath, nestedChain, entryPath)
	entry, err := segmentedPlaybackCache.Ensure(ctx, cacheKey, func(buildCtx context.Context) (*segmentedVideoPlaybackCacheEntry, error) {
		return buildArchiveSegmentedVideoCache(buildCtx, archCache, fsPath, nestedChain, entryPath, cacheKey)
	})
	if err != nil {
		return segmentedVideoCacheEntry{}, err
	}
	return entry.manifest, nil
}

func buildArchiveSegmentedVideoCache(ctx context.Context, archCache *cache.Cache, fsPath string, nestedChain []string, entryPath string, cacheKey string) (*segmentedVideoPlaybackCacheEntry, error) {
	nativeFsPath := pathref.ToNativePath(fsPath)
	if strings.Contains(nativeFsPath, "..") || strings.Contains(entryPath, "..") {
		return nil, fmt.Errorf("invalid path")
	}

	extractCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	arch, err := archCache.Get(extractCtx, nativeFsPath, nestedChain, "")
	if err != nil {
		return nil, fmt.Errorf("cannot open archive: %w", err)
	}

	entryFile, err := arch.Open(extractCtx, entryPath)
	if err != nil {
		return nil, fmt.Errorf("cannot open entry: %w", err)
	}
	defer entryFile.Close()

	ext := filepath.Ext(entryPath)
	tmp, err := os.CreateTemp("", "wails-seg-*"+ext)
	if err != nil {
		return nil, fmt.Errorf("cannot create temp file: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := io.Copy(tmp, entryFile); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return nil, fmt.Errorf("cannot extract entry: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return nil, fmt.Errorf("cannot finalize temp file: %w", err)
	}

	result, err := buildSegmentedVideoCache(ctx, tmpPath, cacheKey)
	if err != nil {
		os.Remove(tmpPath)
		return nil, err
	}
	result.cleanup = func() { os.Remove(tmpPath) } //nolint:errcheck
	return result, nil
}

func buildSegmentedVideoCache(ctx context.Context, nativePath string, cacheKey string) (*segmentedVideoPlaybackCacheEntry, error) {

	probe, err := probeVideo(nativePath)
	if err != nil {
		return nil, err
	}
	videoCodec, _ := extractProbeCodecs(probe)
	if !supportsSegmentedVideoCodec(videoCodec) {
		return nil, fmt.Errorf("segmented playback requires transcoding for video codec %q", videoCodec)
	}
	durationSeconds := resolveVideoDurationSeconds(ctx, nativePath, probe)
	segmentRanges := buildFixedSegmentRanges(durationSeconds)
	if shouldUseKeyframeAlignedSegments(nativePath) {
		if keyframeRanges, err := buildKeyframeAlignedSegmentRanges(ctx, nativePath, durationSeconds); err == nil && len(keyframeRanges) > 0 {
			segmentRanges = keyframeRanges
		}
	}

	manifest := segmentedVideoCacheEntry{
		CacheKey:        cacheKey,
		Format:          "mp4",
		DurationSeconds: durationSeconds,
		SegmentDuration: segmentedPreviewSegmentSeconds,
		TotalSegments:   len(segmentRanges),
		SegmentRanges:   segmentRanges,
	}

	firstSegmentStart := 0.0
	firstSegmentDuration := segmentedPreviewSegmentSeconds
	if len(segmentRanges) > 0 {
		firstSegmentStart = segmentRanges[0].StartSeconds
		firstSegmentDuration = segmentRanges[0].DurationSeconds
	}
	if firstSegmentDuration <= 0 {
		firstSegmentDuration = segmentedPreviewSegmentSeconds
	}

	initSegment, firstSegment, err := buildSegmentedVideoFragment(ctx, nativePath, firstSegmentStart, firstSegmentDuration)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	return &segmentedVideoPlaybackCacheEntry{
		manifest:    manifest,
		nativePath:  nativePath,
		initSegment: initSegment,
		segments: map[int][]byte{
			0: firstSegment,
		},
		segmentSeen: map[int]time.Time{
			0: now,
		},
		inflight:   make(map[int]*segmentedVideoSegmentBuild),
		createdAt:  now,
		lastAccess: now,
	}, nil
}

func shouldUseKeyframeAlignedSegments(nativePath string) bool {
	switch strings.ToLower(filepath.Ext(nativePath)) {
	case ".mp4", ".m4v", ".webm":
		return false
	default:
		return shouldAttemptSegmentedDiskPlayback(nativePath, "", "")
	}
}

func requiresAccurateSegmentSeek(nativePath string) bool {
	switch strings.ToLower(filepath.Ext(nativePath)) {
	case ".avi", ".divx", ".xvid":
		return true
	default:
		return false
	}
}

func splitFragmentedMP4(data []byte) ([]byte, []byte, error) {
	if len(data) < 8 {
		return nil, nil, fmt.Errorf("failed to build segmented preview: fragmented mp4 output too small")
	}

	splitOffset := -1
	for offset := 0; offset+8 <= len(data); {
		boxSize := int(uint32(data[offset])<<24 | uint32(data[offset+1])<<16 | uint32(data[offset+2])<<8 | uint32(data[offset+3]))
		headerSize := 8
		if boxSize == 1 {
			if offset+16 > len(data) {
				return nil, nil, fmt.Errorf("failed to build segmented preview: invalid extended mp4 box")
			}
			largeSize := uint64(data[offset+8])<<56 | uint64(data[offset+9])<<48 | uint64(data[offset+10])<<40 | uint64(data[offset+11])<<32 |
				uint64(data[offset+12])<<24 | uint64(data[offset+13])<<16 | uint64(data[offset+14])<<8 | uint64(data[offset+15])
			if largeSize > uint64(len(data)-offset) || largeSize < 16 {
				return nil, nil, fmt.Errorf("failed to build segmented preview: invalid extended mp4 size")
			}
			boxSize = int(largeSize)
			headerSize = 16
		} else if boxSize == 0 {
			boxSize = len(data) - offset
		}
		if boxSize < headerSize || offset+boxSize > len(data) {
			return nil, nil, fmt.Errorf("failed to build segmented preview: invalid mp4 box size")
		}

		boxType := string(data[offset+4 : offset+8])
		if boxType == "sidx" || boxType == "styp" || boxType == "moof" || boxType == "mdat" {
			splitOffset = offset
			break
		}
		offset += boxSize
	}

	if splitOffset <= 0 || splitOffset >= len(data) {
		return nil, nil, fmt.Errorf("failed to build segmented preview: could not split initialization segment")
	}

	initSegment := append([]byte(nil), data[:splitOffset]...)
	mediaSegment := append([]byte(nil), data[splitOffset:]...)
	if len(mediaSegment) == 0 {
		return nil, nil, fmt.Errorf("failed to build segmented preview: empty media segment")
	}
	return initSegment, mediaSegment, nil
}

func segmentedVideoCacheKey(nativePath string, fi os.FileInfo) string {
	hash := sha1.Sum([]byte(strings.Join([]string{nativePath, strconv.FormatInt(fi.Size(), 10), strconv.FormatInt(fi.ModTime().UnixNano(), 10)}, "|")))
	return fmt.Sprintf("%x", hash)
}

func newSegmentedVideoPlaybackCache() *segmentedVideoPlaybackCache {
	return &segmentedVideoPlaybackCache{
		entries:  make(map[string]*segmentedVideoPlaybackCacheEntry),
		inflight: make(map[string]*segmentedVideoPlaybackBuild),
	}
}

func (c *segmentedVideoPlaybackCache) Ensure(ctx context.Context, cacheKey string, build func(context.Context) (*segmentedVideoPlaybackCacheEntry, error)) (*segmentedVideoPlaybackCacheEntry, error) {
	for {
		c.mu.Lock()
		c.evictExpiredLocked(time.Now())
		if entry, ok := c.entries[cacheKey]; ok {
			entry.lastAccess = time.Now()
			c.mu.Unlock()
			return entry, nil
		}
		pending, ok := c.inflight[cacheKey]
		if !ok {
			buildCtx, cancel := context.WithCancel(context.Background())
			pending = &segmentedVideoPlaybackBuild{done: make(chan struct{}), cancel: cancel}
			c.inflight[cacheKey] = pending
			go c.runPlaybackBuild(cacheKey, pending, buildCtx, build)
		}
		pending.waiters++
		c.mu.Unlock()

		select {
		case <-pending.done:
			if pending.err != nil {
				return nil, pending.err
			}
			if pending.entry != nil {
				return pending.entry, nil
			}
		case <-ctx.Done():
			c.releasePlaybackBuildWaiter(cacheKey, pending)
			return nil, ctx.Err()
		}
	}
}

func (c *segmentedVideoPlaybackCache) runPlaybackBuild(cacheKey string, pending *segmentedVideoPlaybackBuild, buildCtx context.Context, build func(context.Context) (*segmentedVideoPlaybackCacheEntry, error)) {
	entry, err := build(buildCtx)

	c.mu.Lock()
	defer c.mu.Unlock()
	if err == nil && entry != nil {
		entry.lastAccess = time.Now()
		c.entries[cacheKey] = entry
	}
	pending.entry = entry
	pending.err = err
	if current, ok := c.inflight[cacheKey]; ok && current == pending {
		delete(c.inflight, cacheKey)
	}
	close(pending.done)
}

func (c *segmentedVideoPlaybackCache) releasePlaybackBuildWaiter(cacheKey string, pending *segmentedVideoPlaybackBuild) {
	c.mu.Lock()
	defer c.mu.Unlock()
	current, ok := c.inflight[cacheKey]
	if !ok || current != pending {
		return
	}
	if pending.waiters > 0 {
		pending.waiters--
	}
	if pending.waiters == 0 && pending.cancel != nil {
		pending.cancel()
	}
}

func (c *segmentedVideoPlaybackCache) File(ctx context.Context, cacheKey string, fileName string) ([]byte, time.Time, bool, error) {
	now := time.Now()
	c.mu.Lock()
	c.evictExpiredLocked(now)
	entry, ok := c.entries[cacheKey]
	if ok {
		entry.lastAccess = now
	}
	c.mu.Unlock()
	if !ok {
		return nil, time.Time{}, false, nil
	}

	content, err := entry.file(ctx, fileName)
	if err != nil {
		return nil, time.Time{}, false, err
	}
	if len(content) == 0 {
		return nil, time.Time{}, false, nil
	}

	c.mu.Lock()
	entry.lastAccess = time.Now()
	c.mu.Unlock()
	return content, entry.createdAt, true, nil
}

func (c *segmentedVideoPlaybackCache) Clear() {
	c.mu.Lock()
	for _, entry := range c.entries {
		if entry.cleanup != nil {
			go entry.cleanup()
		}
	}
	clear(c.entries)
	clear(c.inflight)
	c.mu.Unlock()
}

func (c *segmentedVideoPlaybackCache) Get(cacheKey string) (*segmentedVideoPlaybackCacheEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[cacheKey]
	if ok {
		entry.lastAccess = time.Now()
	}
	return entry, ok
}

func (c *segmentedVideoPlaybackCache) evictExpiredLocked(now time.Time) {
	for cacheKey, entry := range c.entries {
		if now.Sub(entry.lastAccess) > segmentedVideoCacheTTL {
			delete(c.entries, cacheKey)
			if entry.cleanup != nil {
				go entry.cleanup()
			}
		}
	}
}

func cleanupSegmentedVideoPlaybackCache() {
	segmentedPlaybackCache.Clear()
}

func (e *segmentedVideoPlaybackCacheEntry) file(ctx context.Context, fileName string) ([]byte, error) {
	if fileName == "init.mp4" {
		initSegment, _, err := e.ensureSegment(ctx, 0)
		return initSegment, err
	}

	segmentIndex, ok := parseSegmentedVideoFileName(fileName)
	if !ok {
		return nil, nil
	}
	_, segment, err := e.ensureSegment(ctx, segmentIndex)
	return segment, err
}

func (e *segmentedVideoPlaybackCacheEntry) ensureSegment(ctx context.Context, segmentIndex int) ([]byte, []byte, error) {
	if segmentIndex < 0 || segmentIndex >= e.manifest.TotalSegments {
		return nil, nil, nil
	}

	for {
		now := time.Now()
		e.mu.Lock()
		if len(e.initSegment) > 0 {
			if segment, ok := e.segments[segmentIndex]; ok {
				e.segmentSeen[segmentIndex] = now
				e.lastAccess = now
				initSegment := append([]byte(nil), e.initSegment...)
				mediaSegment := append([]byte(nil), segment...)
				e.mu.Unlock()
				return initSegment, mediaSegment, nil
			}
		}
		pending, ok := e.inflight[segmentIndex]
		if !ok {
			buildCtx, cancel := context.WithCancel(context.Background())
			pending = &segmentedVideoSegmentBuild{done: make(chan struct{}), cancel: cancel}
			e.inflight[segmentIndex] = pending

			startSeconds, durationSeconds := e.segmentRange(segmentIndex)

			go e.runSegmentBuild(segmentIndex, pending, buildCtx, startSeconds, durationSeconds)
		}
		pending.waiters++
		e.mu.Unlock()

		select {
		case <-pending.done:
			if pending.err != nil {
				return nil, nil, pending.err
			}
			return pending.init, pending.segment, nil
		case <-ctx.Done():
			e.releaseSegmentBuildWaiter(segmentIndex, pending)
			return nil, nil, ctx.Err()
		}
	}
}

func (e *segmentedVideoPlaybackCacheEntry) segmentRange(segmentIndex int) (float64, float64) {
	if segmentIndex >= 0 && segmentIndex < len(e.manifest.SegmentRanges) {
		segmentRange := e.manifest.SegmentRanges[segmentIndex]
		durationSeconds := segmentRange.DurationSeconds
		if durationSeconds <= 0 {
			durationSeconds = e.manifest.SegmentDuration
		}
		return segmentRange.StartSeconds, durationSeconds
	}

	startSeconds := float64(segmentIndex) * e.manifest.SegmentDuration
	durationSeconds := e.manifest.SegmentDuration
	if remaining := e.manifest.DurationSeconds - startSeconds; remaining > 0 && remaining < durationSeconds {
		durationSeconds = remaining
	}
	if durationSeconds <= 0 {
		durationSeconds = e.manifest.SegmentDuration
	}
	return startSeconds, durationSeconds
}

func (e *segmentedVideoPlaybackCacheEntry) runSegmentBuild(segmentIndex int, pending *segmentedVideoSegmentBuild, buildCtx context.Context, startSeconds float64, durationSeconds float64) {
	initSegment, mediaSegment, err := buildSegmentedVideoFragment(buildCtx, e.nativePath, startSeconds, durationSeconds)

	e.mu.Lock()
	defer e.mu.Unlock()
	if err == nil {
		now := time.Now()
		if len(e.initSegment) == 0 {
			e.initSegment = initSegment
		}
		e.segments[segmentIndex] = mediaSegment
		e.segmentSeen[segmentIndex] = now
		e.lastAccess = now
		e.pruneSegmentsLocked()
	}
	pending.init = initSegment
	pending.segment = mediaSegment
	pending.err = err
	if current, ok := e.inflight[segmentIndex]; ok && current == pending {
		delete(e.inflight, segmentIndex)
	}
	close(pending.done)
}

func (e *segmentedVideoPlaybackCacheEntry) releaseSegmentBuildWaiter(segmentIndex int, pending *segmentedVideoSegmentBuild) {
	e.mu.Lock()
	defer e.mu.Unlock()
	current, ok := e.inflight[segmentIndex]
	if !ok || current != pending {
		return
	}
	if pending.waiters > 0 {
		pending.waiters--
	}
	if pending.waiters == 0 && pending.cancel != nil {
		pending.cancel()
	}
}

func (e *segmentedVideoPlaybackCacheEntry) pruneSegmentsLocked() {
	if len(e.segments) <= segmentedVideoMaxResidentSegments {
		return
	}

	type residentSegment struct {
		index int
		seen  time.Time
	}
	resident := make([]residentSegment, 0, len(e.segments))
	for index, seenAt := range e.segmentSeen {
		resident = append(resident, residentSegment{index: index, seen: seenAt})
	}
	sort.Slice(resident, func(left int, right int) bool {
		return resident[left].seen.Before(resident[right].seen)
	})
	for len(resident) > segmentedVideoMaxResidentSegments {
		oldest := resident[0]
		delete(e.segments, oldest.index)
		delete(e.segmentSeen, oldest.index)
		resident = resident[1:]
	}
}

func parseSegmentedVideoFileName(fileName string) (int, bool) {
	if !strings.HasPrefix(fileName, "segment_") || !strings.HasSuffix(fileName, ".m4s") {
		return 0, false
	}
	rawIndex := strings.TrimSuffix(strings.TrimPrefix(fileName, "segment_"), ".m4s")
	segmentNumber, err := strconv.Atoi(rawIndex)
	if err != nil || segmentNumber <= 0 {
		return 0, false
	}
	return segmentNumber - 1, true
}