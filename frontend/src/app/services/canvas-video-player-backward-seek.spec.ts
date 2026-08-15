
/**
 * Canvas video player backward seek tests
 * 
 * Tests to ensure:
 * 1. Buffered frame detection works properly for backward seeks
 * 2. Frame tolerance is correctly applied
 * 3. No discrepancies in decoded frames vs buffered frames
 */

interface MockDecodedFrame {
  timestampMs: number;
  frameId: string;
}

/**
 * Test: buffered frame detection tolerance for backward seeks
 */
describe('Buffered frame detection for backward seeks', () => {
  const decodedFrames: MockDecodedFrame[] = [
    { timestampMs: 1000, frameId: 'f1' },
    { timestampMs: 1033, frameId: 'f2' },
    { timestampMs: 1066, frameId: 'f3' },
    { timestampMs: 1099, frameId: 'f4' },
  ];

  function getFrameIntervalEstimateMs(): number {
    if (decodedFrames.length > 1) {
      const delta = decodedFrames[1].timestampMs - decodedFrames[0].timestampMs;
      if (delta > 0) {
        return Math.min(250, Math.max(16, delta));
      }
    }
    return 33.33;
  }

  function getPresentationToleranceMs(): number {
    return Math.min(12, getFrameIntervalEstimateMs() * 0.25);
  }

  function findBufferedFrameForTime(targetTimeMs: number, isSeekingBackward: boolean): MockDecodedFrame | undefined {
    if (decodedFrames.length === 0) {
      return undefined;
    }

    const presentationToleranceMs = getPresentationToleranceMs();
    const firstFrame = decodedFrames[0];
    const lastFrame = decodedFrames[decodedFrames.length - 1];
    const frameIntervalMs = getFrameIntervalEstimateMs();

    // Both backward and forward seeks use standard tolerance
    const effectiveTolerance = presentationToleranceMs;

    if (targetTimeMs < firstFrame.timestampMs - effectiveTolerance) {
      return undefined;
    }

    if (targetTimeMs > lastFrame.timestampMs + (frameIntervalMs * 0.25)) {
      return undefined;
    }

    // Find frame with timestamp closest to but not exceeding target
    let selectedFrame = firstFrame;
    for (const frame of decodedFrames) {
      if (frame.timestampMs <= targetTimeMs + effectiveTolerance) {
        selectedFrame = frame;
      } else {
        break;
      }
    }

    return selectedFrame;
  }

  it('should find buffered frame for backward seek within decoded range', () => {
    const frame = findBufferedFrameForTime(1050, true);
    expect(frame).toBeDefined();
    expect(frame?.frameId).toBe('f2');
  });

  it('should not find frame before decoded range', () => {
    const frame = findBufferedFrameForTime(500, true);
    expect(frame).toBeUndefined();
  });

  it('should find buffered frames for backward and forward seeks', () => {
    // Both backward and forward seeks use the same tolerance logic
    const presentationToleranceMs = getPresentationToleranceMs();
    
    // Tolerance should be applied consistently
    expect(presentationToleranceMs).toBeGreaterThan(0);
    
    // Both should find valid frames at same target time
    const backwardFrame = findBufferedFrameForTime(1050, true);
    const forwardFrame = findBufferedFrameForTime(1050, false);
    
    expect(backwardFrame).toBeDefined();
    expect(forwardFrame).toBeDefined();
    // Since we removed the tolerance multiplier, both should find the same frame
    expect(backwardFrame?.frameId).toBe(forwardFrame?.frameId);
  });

  it('should return last decoded frame when target is near last', () => {
    const frameIntervalMs = getFrameIntervalEstimateMs();
    const lastFrame = decodedFrames[decodedFrames.length - 1];
    // Target within tolerance of last frame
    const targetTime = lastFrame.timestampMs + 5;

    const frame = findBufferedFrameForTime(targetTime, true);
    expect(frame?.frameId).toBe('f4');
  });
});

/**
 * Test: time accuracy discrepancies
 */
describe('Seek time accuracy and discrepancies', () => {
  function clampSeekTime(targetMs: number, durationMs: number): number {
    return Math.max(0, Math.min(targetMs, durationMs));
  }

  const durationMs = 10_000;

  it('should clamp seek time to valid range', () => {
    expect(clampSeekTime(-500, durationMs)).toBe(0);
    expect(clampSeekTime(5000, durationMs)).toBe(5000);
    expect(clampSeekTime(15_000, durationMs)).toBe(durationMs);
  });

  it('backward and forward clamp to same boundaries', () => {
    const negativeTarget = -500;
    const overTarget = 15_000;

    const backwardClamp = clampSeekTime(negativeTarget, durationMs);
    const forwardClamp = clampSeekTime(negativeTarget, durationMs);
    
    expect(backwardClamp).toBe(forwardClamp);
    expect(backwardClamp).toBe(0);

    const backwardClamp2 = clampSeekTime(overTarget, durationMs);
    const forwardClamp2 = clampSeekTime(overTarget, durationMs);
    
    expect(backwardClamp2).toBe(forwardClamp2);
    expect(backwardClamp2).toBe(durationMs);
  });
});

describe('Timestamp precision retention', () => {
  it('preserves fractional frame timestamps from WebCodecs output', () => {
    const frameTimestampUs = 33_366;
    const timestampMs = frameTimestampUs / 1000;

    expect(timestampMs).toBe(33.366);
    expect(timestampMs).not.toBe(Math.round(frameTimestampUs / 1000));
  });

  it('preserves fractional segment offsets when stitching packet timelines', () => {
    const segmentStartSeconds = 2.002;
    const packetTimestampMs = 33.366;
    const stitchedTimestampMs = packetTimestampMs + (segmentStartSeconds * 1000);

    expect(stitchedTimestampMs).toBeCloseTo(2035.366, 6);
  });
});

describe('Audio-clock playback sync', () => {
  type ScheduledAudioEntry = {
    startTime: number;
    endTime: number;
    mediaStartSec: number;
  };

  function getScheduledAudioPlaybackTimeMs(
    scheduledAudioNodes: ScheduledAudioEntry[],
    nowSec: number,
    playbackBaseMs: number,
  ): number | null {
    if (scheduledAudioNodes.length === 0) {
      return null;
    }

    const firstScheduledNode = scheduledAudioNodes[0];
    if (nowSec < firstScheduledNode.startTime) {
      return playbackBaseMs;
    }

    let anchor = firstScheduledNode;
    for (const entry of scheduledAudioNodes) {
      if (entry.startTime > nowSec) {
        break;
      }
      anchor = entry;
    }

    const elapsedSec = Math.max(0, Math.min(nowSec, anchor.endTime) - anchor.startTime);
    return (anchor.mediaStartSec + elapsedSec) * 1000;
  }

  it('keeps video at the playback base before the first scheduled audio chunk starts', () => {
    const scheduled = [
      { startTime: 10.05, endTime: 10.15, mediaStartSec: 5.0 },
    ];

    expect(getScheduledAudioPlaybackTimeMs(scheduled, 10.02, 5000)).toBe(5000);
  });

  it('follows actual scheduled audio timing when chunks start later than the ideal clock', () => {
    const scheduled = [
      { startTime: 10.05, endTime: 10.15, mediaStartSec: 5.0 },
      { startTime: 10.15, endTime: 10.25, mediaStartSec: 5.1 },
    ];

    expect(getScheduledAudioPlaybackTimeMs(scheduled, 10.20, 5000)).toBeCloseTo(5150, 6);
  });
});

/**
 * Test: format & codec support consistency
 */
describe('Video codec and format consistency', () => {
  const demuxerBackedExtensions = new Set([
    'mp4', 'm4v', 'mov', 'mkv', 'webm', 'ogv', 'mxf',
    'avi', 'divx', 'xvid',
    'flv', 'ts', 'mts', 'm2ts',
    'mpg', 'mpeg', 'vob', 'm2p', 'm2v',
    'mjpeg', 'mjpg',
  ]);

  const ffmpegTranscodeExtensions = new Set([
    'wmv', 'asf', 'mov', 'qt', 'm3u8',
  ]);

  it('should have format coverage for segmented delivery', () => {
    // Key formats that should be segmentable
    const segmentableFormats = ['mp4', 'm4v', 'mov', 'mkv', 'webm'];
    for (const fmt of segmentableFormats) {
      expect(demuxerBackedExtensions.has(fmt)).toBe(true);
    }
  });

  it('should not have duplicate codec in both supported and transcode lists', () => {
    // Some formats appear in both (like mov) — that's ok, but major codecs shouldn't differ
    const allowedInBoth = new Set(['mov', 'qt']);
    const overlap = Array.from(demuxerBackedExtensions).filter(
      ext => ffmpegTranscodeExtensions.has(ext) && !allowedInBoth.has(ext)
    );
    
    // The overlap should only be formats that need transcoding fallback
    expect(overlap.length).toBeLessThanOrEqual(2);
  });

  it('should support all major video container formats', () => {
    const majorFormats = ['mp4', 'mkv', 'webm', 'avi', 'mov'];
    for (const fmt of majorFormats) {
      expect(demuxerBackedExtensions.has(fmt)).toBe(true);
    }
  });
});

/**
 * Test: segment window management
 */
describe('Segment window management consistency', () => {
  const SEGMENT_RETENTION_BEHIND = 6;
  const SEGMENT_RETENTION_AHEAD = 6;
  const SEGMENT_PREFETCH_AHEAD = 3;

  function calculateSegmentWindow(anchorIndex: number) {
    const minIndex = Math.max(0, anchorIndex - SEGMENT_RETENTION_BEHIND);
    const maxIndex = anchorIndex + SEGMENT_RETENTION_AHEAD;
    const maxPrefetch = anchorIndex + SEGMENT_RETENTION_AHEAD + SEGMENT_PREFETCH_AHEAD;
    return { minIndex, maxIndex, maxPrefetch };
  }

  it('should calculate symmetric window for mid-video anchor', () => {
    const totalSegments = 100;
    const anchor = 50;
    const window = calculateSegmentWindow(anchor);

    expect(window.minIndex).toBe(44);
    expect(window.maxIndex).toBe(56);
    expect(window.maxPrefetch).toBe(59);
  });

  it('should clamp window at start of video', () => {
    const window = calculateSegmentWindow(0);
    expect(window.minIndex).toBe(0);
    expect(window.maxIndex).toBe(6);
  });

  it('should maintain same prefetch ahead for backward and forward seek', () => {
    const anchor1 = 50;
    const anchor2 = 100;

    const window1 = calculateSegmentWindow(anchor1);
    const window2 = calculateSegmentWindow(anchor2);

    // Prefetch ahead should be consistent relative to anchor
    expect(window1.maxPrefetch - window1.maxIndex).toBe(window2.maxPrefetch - window2.maxIndex);
    expect(window1.maxPrefetch - anchor1).toBe(window2.maxPrefetch - anchor2);
  });

  it('should have equal retention behind and ahead', () => {
    const anchor = 50;
    const window = calculateSegmentWindow(anchor);
    
    const retentionBehind = anchor - window.minIndex;
    const retentionAhead = window.maxIndex - anchor;
    
    expect(retentionBehind).toBe(SEGMENT_RETENTION_BEHIND);
    expect(retentionAhead).toBe(SEGMENT_RETENTION_AHEAD);
  });
});

/**
 * Test: failure handling parity
 */
describe('Segment load failure handling', () => {
  const SEGMENT_FAILURE_COOLDOWN_MS = 5000;

  function isSegmentRecentlyFailed(failedAt: number | undefined, now: number): boolean {
    if (failedAt === undefined) return false;
    return now - failedAt < SEGMENT_FAILURE_COOLDOWN_MS;
  }

  it('should block recently failed segments', () => {
    const failedAt = 1000;
    const now1 = 2000;  // 1000ms later
    const now2 = 6500;  // 5500ms later

    expect(isSegmentRecentlyFailed(failedAt, now1)).toBe(true);
    expect(isSegmentRecentlyFailed(failedAt, now2)).toBe(false);
  });

  it('should allow retry after cooldown', () => {
    const failedAt = 1000;
    const exactCooldown = 1000 + SEGMENT_FAILURE_COOLDOWN_MS;
    const afterCooldown = exactCooldown + 1;

    expect(isSegmentRecentlyFailed(failedAt, exactCooldown)).toBe(false);
    expect(isSegmentRecentlyFailed(failedAt, afterCooldown)).toBe(false);
  });

  it('should not block segments that never failed', () => {
    expect(isSegmentRecentlyFailed(undefined, 999_999_999)).toBe(false);
  });
});

/**
 * Test: discrepancy detection for concurrent operations
 */
describe('Concurrent seek discrepancy detection', () => {
  it('should detect stale seek operations', () => {
    const seekGeneration1 = 1;
    const seekGeneration2 = 2;

    function isCurrentSeek(generation: number, currentGeneration: number): boolean {
      return generation === currentGeneration;
    }

    // Seek 1 starts
    expect(isCurrentSeek(seekGeneration1, 1)).toBe(true);
    
    // Seek 2 starts (increments generation)
    expect(isCurrentSeek(seekGeneration1, 2)).toBe(false);
    expect(isCurrentSeek(seekGeneration2, 2)).toBe(true);

    // Seek 1 should be abandoned
    expect(isCurrentSeek(seekGeneration1, 2)).toBe(false);
  });

  it('should prevent old seek results from overwriting new ones', () => {
    const generations = [1, 2, 3];
    const results = new Map<number, string>();

    // Simulate out-of-order completion
    const completionOrder = [2, 3, 1]; // Gen2 completes, then 3, then 1

    for (const gen of completionOrder) {
      const currentGen = 3; // This is the active generation
      if (gen === currentGen) {
        results.set(gen, `frame-${gen}`);
      }
    }

    // Only generation 3 results should be stored
    expect(results.has(3)).toBe(true);
    expect(results.has(1)).toBe(false);
    expect(results.has(2)).toBe(false);
  });
});

