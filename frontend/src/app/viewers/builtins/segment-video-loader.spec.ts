import { SampleInfo } from '../../services/canvas-video-player.service';

/**
 * findPacketIndex tests — pure function for locating packets by timestamp
 */

// Export test utilities (these would be copy-pasted from segment-video-loader.ts for testing)
function findPacketIndex(packets: SampleInfo[], timeMs: number, requireKeyframe: boolean): number {
  let bestIndex = -1;
  
  if (requireKeyframe) {
    // Find the keyframe with timestamp <= timeMs (for backward seeks)
    for (let index = 0; index < packets.length; index += 1) {
      const packet = packets[index];
      if (packet.isRap && packet.timestampMs <= timeMs) {
        bestIndex = index;
      } else if (packet.timestampMs > timeMs && bestIndex >= 0) {
        break;
      }
    }
    
    if (bestIndex >= 0) {
      return bestIndex;
    }
    
    // If no keyframe before, find first keyframe
    return packets.findIndex(packet => packet.isRap);
  }
  
  // Regular forward seek - find first packet >= timeMs
  for (let index = 0; index < packets.length; index += 1) {
    if (packets[index].timestampMs >= timeMs) {
      return index;
    }
  }
  
  return packets.length > 0 ? packets.length - 1 : -1;
}

const mockPacket = (index: number, timestampMs: number, isRap: boolean): SampleInfo => ({
  index,
  timestampMs,
  durationMs: 33.33,
  isRap,
  data: new Uint8Array(0),
});

describe('findPacketIndex', () => {
  describe('forward seeks (requireKeyframe=false)', () => {
    it('should return -1 for empty packet list', () => {
      expect(findPacketIndex([], 500, false)).toBe(-1);
    });

    it('should return 0 for time before first packet', () => {
      const packets = [mockPacket(0, 100, true), mockPacket(1, 200, false)];
      expect(findPacketIndex(packets, 50, false)).toBe(0);
    });

    it('should find exact timestamp match', () => {
      const packets = [mockPacket(0, 100, true), mockPacket(1, 200, false), mockPacket(2, 300, true)];
      expect(findPacketIndex(packets, 200, false)).toBe(1);
    });

    it('should find first packet >= target time', () => {
      const packets = [mockPacket(0, 100, true), mockPacket(1, 200, false), mockPacket(2, 300, true)];
      // Target 150 should return packet at 200 (index 1)
      expect(findPacketIndex(packets, 150, false)).toBe(1);
    });

    it('should return last packet for time beyond all packets', () => {
      const packets = [mockPacket(0, 100, true), mockPacket(1, 200, false)];
      expect(findPacketIndex(packets, 500, false)).toBe(1);
    });

    it('should handle single packet', () => {
      const packets = [mockPacket(0, 100, true)];
      expect(findPacketIndex(packets, 150, false)).toBe(0);
    });
  });

  describe('backward seeks (requireKeyframe=true)', () => {
    it('should return -1 for empty packet list', () => {
      expect(findPacketIndex([], 500, true)).toBe(-1);
    });

    it('should find last keyframe <= target time', () => {
      const packets = [
        mockPacket(0, 100, true),
        mockPacket(1, 150, false),
        mockPacket(2, 200, true),
        mockPacket(3, 250, false),
        mockPacket(4, 300, true),
      ];
      // Seek to 250 should return keyframe at 200 (index 2)
      expect(findPacketIndex(packets, 250, true)).toBe(2);
    });

    it('should return first keyframe when target is before all keyframes', () => {
      const packets = [
        mockPacket(0, 100, true),
        mockPacket(1, 150, false),
        mockPacket(2, 200, false),
      ];
      // Seek to 50 (before first keyframe at 100)
      expect(findPacketIndex(packets, 50, true)).toBe(0);
    });

    it('should handle all keyframes', () => {
      const packets = [
        mockPacket(0, 100, true),
        mockPacket(1, 200, true),
        mockPacket(2, 300, true),
      ];
      expect(findPacketIndex(packets, 250, true)).toBe(1);
    });

    it('should handle no keyframes', () => {
      const packets = [
        mockPacket(0, 100, false),
        mockPacket(1, 200, false),
      ];
      // Should return first keyframe (-1) since none exist
      expect(findPacketIndex(packets, 500, true)).toBe(-1);
    });

    it('should return exact keyframe match', () => {
      const packets = [
        mockPacket(0, 100, true),
        mockPacket(1, 150, false),
        mockPacket(2, 200, true),
      ];
      expect(findPacketIndex(packets, 200, true)).toBe(2);
    });

    it('should skip non-keyframes in target range', () => {
      const packets = [
        mockPacket(0, 100, true),
        mockPacket(1, 150, false),
        mockPacket(2, 160, false), // These are in range but not keyframes
        mockPacket(3, 170, false),
        mockPacket(4, 200, true),
      ];
      // Seeking to 165 should return keyframe at 100, not 200
      expect(findPacketIndex(packets, 165, true)).toBe(0);
    });
  });

  describe('edge cases', () => {
    it('should handle large timestamp values', () => {
      const packets = [
        mockPacket(0, 1_000_000_000, true),
        mockPacket(1, 2_000_000_000, false),
      ];
      expect(findPacketIndex(packets, 1_500_000_000, false)).toBe(1);
    });

    it('should handle packets at time 0', () => {
      const packets = [
        mockPacket(0, 0, true),
        mockPacket(1, 33, false),
      ];
      expect(findPacketIndex(packets, 0, true)).toBe(0);
    });

    it('should handle negative target (should clamp to 0)', () => {
      const packets = [
        mockPacket(0, 100, true),
        mockPacket(1, 200, false),
      ];
      // Note: the actual function doesn't clamp, so this tests current behavior
      expect(findPacketIndex(packets, -50, false)).toBe(0);
    });
  });
});

/**
 * collectPackets simulation tests
 */
describe('collectPackets pattern', () => {
  it('should preserve packet offsets when collecting', () => {
    const source: SampleInfo[] = [
      mockPacket(0, 0, true),
      mockPacket(1, 33, false),
      mockPacket(2, 66, true),
    ];

    const collected: SampleInfo[] = [];
    let cursor: SampleInfo | null = source[0];
    const timeOffsetMs = 1000;

    while (cursor) {
      collected.push({
        ...cursor,
        timestampMs: cursor.timestampMs + timeOffsetMs,
      });
      const nextIndex: number = source.indexOf(cursor) + 1;
      cursor = nextIndex < source.length ? source[nextIndex] : null;
    }

    // Verify offset was applied
    expect(collected[0].timestampMs).toBe(1000);
    expect(collected[1].timestampMs).toBe(1033);
    expect(collected[2].timestampMs).toBe(1066);
  });
});

/**
 * Segment index calculation tests
 */
describe('segment timing calculations', () => {
  const SEGMENT_DURATION = 2.0; // seconds

  function findSegmentIndex(timeMs: number, totalSegments: number, segmentDuration = SEGMENT_DURATION): number {
    for (let index = 0; index < totalSegments; index += 1) {
      const startMs = Math.round(index * segmentDuration * 1000);
      const endMs = startMs + Math.round(segmentDuration * 1000);
      if (timeMs < endMs) {
        return index;
      }
    }
    return Math.max(0, totalSegments - 1);
  }

  it('should find correct segment for times within range', () => {
    const totalSegments = 10;
    expect(findSegmentIndex(0, totalSegments)).toBe(0);
    expect(findSegmentIndex(1000, totalSegments)).toBe(0);
    expect(findSegmentIndex(2000, totalSegments)).toBe(1);
    expect(findSegmentIndex(4000, totalSegments)).toBe(2);
  });

  it('should return last segment for times beyond duration', () => {
    const totalSegments = 10;
    const duration = totalSegments * SEGMENT_DURATION * 1000; // 20 seconds in ms
    expect(findSegmentIndex(duration + 1000, totalSegments)).toBe(totalSegments - 1);
  });

  it('should handle boundary between segments', () => {
    const totalSegments = 5;
    // Boundary is at 2000ms
    expect(findSegmentIndex(1999, totalSegments)).toBe(0);
    expect(findSegmentIndex(2000, totalSegments)).toBe(1);
    expect(findSegmentIndex(2001, totalSegments)).toBe(1);
  });
});

/**
 * Backward vs forward seek parity tests
 */
describe('backward vs forward seek consistency', () => {
  it('seeking backward then forward to same time should find same packet', () => {
    const packets = [
      mockPacket(0, 0, true),
      mockPacket(1, 100, false),
      mockPacket(2, 200, true),
      mockPacket(3, 300, false),
      mockPacket(4, 400, true),
    ];

    // Forward seek to 250: should find packet at 300 (index 3)
    const forwardIndex = findPacketIndex(packets, 250, false);
    // Backward seek to 250: should find keyframe at 200 (index 2)
    const backwardIndex = findPacketIndex(packets, 250, true);

    // They're different (forward finds next, backward finds last keyframe)
    // but both should be valid
    expect(forwardIndex).toBeGreaterThanOrEqual(0);
    expect(backwardIndex).toBeGreaterThanOrEqual(0);
    expect(packets[forwardIndex].timestampMs).toBeGreaterThanOrEqual(250);
    expect(packets[backwardIndex].timestampMs).toBeLessThanOrEqual(250);
  });

  it('backward seek should not skip to future keyframes', () => {
    const packets = [
      mockPacket(0, 0, true),
      mockPacket(1, 100, false),
      mockPacket(2, 200, false),
      mockPacket(3, 300, true),
      mockPacket(4, 400, false),
    ];

    // Seeking backward to 250 should find keyframe at 0, not 300
    const index = findPacketIndex(packets, 250, true);
    expect(packets[index].timestampMs).toBeLessThanOrEqual(250);
  });
});

