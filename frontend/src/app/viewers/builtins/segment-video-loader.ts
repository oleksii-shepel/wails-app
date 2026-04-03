import {
  DemuxedAudioTrackSource,
  DemuxedPacketCursor,
  DemuxedVideoSource,
  ExtendedVideoDecoderConfig,
  SampleInfo,
  VideoMetadata,
  loadMp4DemuxedSourceFromBuffer,
} from '../../services/canvas-video-player.service';
import { createSubject, distinctUntilChanged } from '@epikodelabs/streamix';

interface SegmentedVideoManifestItem {
  index: number;
  startTime: number;
  durationSeconds: number;
  url: string;
}

interface SegmentedVideoManifest {
  mode: 'segmented';
  format: string;
  durationSeconds: number;
  segmentDuration: number;
  totalSegments: number;
  initSegmentUrl: string;
  segments: SegmentedVideoManifestItem[];
}

interface LoadedSegment {
  index: number;
  videoPackets: SampleInfo[];
  audioPackets: SampleInfo[];
}

interface PacketToken {
  segmentIndex: number;
  packetIndex: number;
}

const SEGMENT_PREFETCH_AHEAD = 3;
const SEGMENT_RETENTION_BEHIND = 6;
const SEGMENT_RETENTION_AHEAD = 6;

export class SegmentVideoLoader {
  private manifest: SegmentedVideoManifest | null = null;
  private initSegment: ArrayBuffer | null = null;
  private readonly loadedSegments = new Map<number, LoadedSegment>();
  private readonly pendingSegments = new Map<number, Promise<LoadedSegment>>();
  private readonly pendingSegmentControllers = new Map<number, AbortController>();
  private readonly anchorRequests$ = createSubject<number>();
  private readonly segmentFetches$ = createSubject<number>();
  private metadata: VideoMetadata | null = null;
  private decoderConfig: ExtendedVideoDecoderConfig | null = null;
  private audioMeta: Omit<DemuxedAudioTrackSource, 'getFirstPacket' | 'getPacketForTime' | 'getNextPacket'> | null = null;
  private activeWindowGeneration = 0;
  private currentAnchorIndex = 0;
  private disposed = false;
  private loadController: AbortController | null = null;
  // Segments that recently failed to load — keyed to the failure timestamp.
  // Prevents the render loop from hammering the server on permanent errors.
  private readonly failedSegments = new Map<number, number>();
  private static readonly SEGMENT_FAILURE_COOLDOWN_MS = 5000;

  constructor() {
    // Anchor changes cancel stale prefetch work and update window
    this.anchorRequests$
      .pipe(distinctUntilChanged())
      .subscribe((anchorIndex: number) => {
        this.currentAnchorIndex = anchorIndex;
        this.activeWindowGeneration += 1;
        this.abortPendingOutsideWindow(anchorIndex);
        this.pruneLoadedSegments(anchorIndex);
      });

    // Track segment fetch attempts for concurrency management
    // Deduplicate rapid successive requests for same segment
    this.segmentFetches$
      .pipe(distinctUntilChanged())
      .subscribe();
  }

  async load(manifestUrl: string): Promise<DemuxedVideoSource> {
    this.dispose();
    this.disposed = false;
    this.loadController = new AbortController();

    const manifestResponse = await fetch(manifestUrl, { signal: this.loadController.signal });
    if (!manifestResponse.ok) {
      const errText = await manifestResponse.text().catch(() => '');
      throw new Error(`Failed to load segment manifest: ${manifestResponse.status}. Details: ${errText}`);
    }

    this.manifest = await manifestResponse.json() as SegmentedVideoManifest;

    const initResponse = await fetch(this.manifest.initSegmentUrl, { signal: this.loadController.signal });
    if (!initResponse.ok) {
      throw new Error(`Failed to load init segment: ${initResponse.status}`);
    }
    this.initSegment = await initResponse.arrayBuffer();

    const initialWindowGeneration = this.activateWindow(0);
    const firstSegment = await this.ensureSegmentLoaded(0, initialWindowGeneration);
    this.prefetchAround(0, initialWindowGeneration);
    if (!this.metadata || !this.decoderConfig) {
      throw new Error('Segmented preview did not provide video metadata.');
    }

    return {
      metadata: this.metadata,
      decoderConfig: this.decoderConfig,
      audio: this.audioMeta ? {
        ...this.audioMeta,
        getFirstPacket: async () => this.getFirstAudioPacket(),
        getPacketForTime: async (timeMs: number) => this.getAudioPacketForTime(timeMs),
        getNextPacket: async (cursor: DemuxedPacketCursor) => this.getNextAudioPacket(cursor),
      } : undefined,
      supportsRandomAccess: true,
      getFirstPacket: async () => firstSegment.videoPackets.length > 0 ? this.makeCursor(0, 0, firstSegment.videoPackets) : null,
      getPacketForTime: async (timeMs: number, options?: { keyframe?: boolean }) => this.getPacketForTime(timeMs, options),
      getNextPacket: async (cursor: DemuxedPacketCursor) => this.getNextPacket(cursor),
      destroy: () => this.dispose(),
    };
  }

  private async ensureSegmentLoaded(index: number, windowGeneration = this.activeWindowGeneration): Promise<LoadedSegment> {
    const existing = this.loadedSegments.get(index);
    if (existing) {
      return existing;
    }

    // Refuse to retry a recently-failed segment so the render loop
    // cannot hammer the server at 60 fps on a permanent 500 error.
    const failedAt = this.failedSegments.get(index);
    if (failedAt !== undefined) {
      if (Date.now() - failedAt < SegmentVideoLoader.SEGMENT_FAILURE_COOLDOWN_MS) {
        throw new Error(`Segment ${index} failed to load recently`);
      }
      this.failedSegments.delete(index);
    }

    const pending = this.pendingSegments.get(index);
    if (pending) {
      return pending;
    }

    // Create and store promise synchronously to prevent duplicate fetches
    // Multiple concurrent requesters will share this same promise
    const loadPromise = this.loadSegment(index);
    this.pendingSegments.set(index, loadPromise);

    this.segmentFetches$.next(index);

    try {
      const segment = await loadPromise;
      this.loadedSegments.set(index, segment);
      return segment;
    } catch (err) {
      // Cache the failure unless it was an intentional abort (seek/dispose).
      if (!(err instanceof DOMException && err.name === 'AbortError')) {
        this.failedSegments.set(index, Date.now());
      }
      throw err;
    } finally {
      this.pendingSegments.delete(index);
    }
  }

  private async loadSegment(index: number): Promise<LoadedSegment> {
    if (!this.manifest || !this.initSegment) {
      throw new Error('Segment loader not initialized.');
    }
    if (this.disposed) {
      throw new DOMException('Segment loader disposed', 'AbortError');
    }

    const segment = this.manifest.segments[index];
    if (!segment) {
      throw new Error(`Segment ${index} is not available.`);
    }

    const controller = new AbortController();
    this.pendingSegmentControllers.set(index, controller);

    try {
      const segmentResponse = await fetch(segment.url, { signal: controller.signal });
      if (!segmentResponse.ok) {
        throw new Error(`Failed to load segment ${index}: ${segmentResponse.status}`);
      }

      const segmentBytes = await segmentResponse.arrayBuffer();
      const combined = concatenateBuffers(this.initSegment, segmentBytes);
      const source = await loadMp4DemuxedSourceFromBuffer(combined);

      if (!this.metadata) {
        this.metadata = {
          ...source.metadata,
          duration: this.manifest.durationSeconds * 1000,
          format: this.manifest.format || source.metadata.format,
        };
        this.decoderConfig = source.decoderConfig;
        if (source.audio) {
          this.audioMeta = {
            decoderConfig: source.audio.decoderConfig,
            codec: source.audio.codec,
            sampleRate: source.audio.sampleRate,
            numberOfChannels: source.audio.numberOfChannels,
          };
        }
      }

      const startOffsetMs = segment.startTime * 1000;
      const segmentEndMs = getSegmentWindowEndMs(this.manifest.segments, index);
      const videoPackets = trimPacketsToSegmentWindow(await collectPackets(
        () => source.getFirstPacket(),
        (cursor) => source.getNextPacket(cursor),
        startOffsetMs,
      ), startOffsetMs, segmentEndMs);
      const audioPackets = source.audio
        ? trimPacketsToSegmentWindow(await collectPackets(
            () => source.audio!.getFirstPacket(),
            (cursor) => source.audio!.getNextPacket(cursor),
            startOffsetMs,
          ), startOffsetMs, segmentEndMs)
        : [];
      source.destroy?.();

      return { index, videoPackets, audioPackets };
    } finally {
      this.pendingSegmentControllers.delete(index);
    }
  }

  private async getPacketForTime(timeMs: number, options?: { keyframe?: boolean }): Promise<DemuxedPacketCursor | null> {
    if (!this.manifest) {
      return null;
    }

    let segmentIndex = this.findSegmentIndex(timeMs);
    const windowGeneration = this.activateWindow(segmentIndex);
    const maxBackwardSegments = options?.keyframe ? 5 : 1;
    let backwardAttempts = 0;

    while (segmentIndex >= 0 && backwardAttempts <= maxBackwardSegments) {
      let segment: LoadedSegment;
      try {
        segment = await this.ensureSegmentLoaded(segmentIndex, windowGeneration);
      } catch {
        // Segment failed to load — if we're searching for a keyframe walk
        // backward to the previous segment rather than propagating the error.
        if (!options?.keyframe) {
          break;
        }
        segmentIndex -= 1;
        backwardAttempts += 1;
        continue;
      }
      const packetIndex = findPacketIndex(segment.videoPackets, timeMs, options?.keyframe === true);
      if (packetIndex >= 0) {
        if (this.isActiveWindow(windowGeneration)) {
          this.prefetchAround(segmentIndex, windowGeneration);
        }
        return this.makeCursor(segmentIndex, packetIndex, segment.videoPackets);
      }

      if (!options?.keyframe) {
        break;
      }

      segmentIndex -= 1;
      backwardAttempts += 1;
    }
    return null;
  }

  private async getNextPacket(cursor: DemuxedPacketCursor): Promise<DemuxedPacketCursor | null> {
    const token = cursor.token as PacketToken;
    const windowGeneration = this.activeWindowGeneration;
    const segment = await this.ensureSegmentLoaded(token.segmentIndex, windowGeneration);
    const nextPacketIndex = token.packetIndex + 1;
    if (nextPacketIndex < segment.videoPackets.length) {
      return this.makeCursor(token.segmentIndex, nextPacketIndex, segment.videoPackets);
    }

    if (!this.manifest || token.segmentIndex + 1 >= this.manifest.totalSegments) {
      return null;
    }

    const nextSegmentIndex = token.segmentIndex + 1;
    const nextWindowGeneration = this.activateWindow(nextSegmentIndex);
    const nextSegment = await this.ensureSegmentLoaded(nextSegmentIndex, nextWindowGeneration);
    if (this.isActiveWindow(nextWindowGeneration)) {
      this.prefetchAround(nextSegmentIndex, nextWindowGeneration);
      }
    return nextSegment.videoPackets.length > 0
      ? this.makeCursor(nextSegmentIndex, 0, nextSegment.videoPackets)
      : null;
  }

  private async getFirstAudioPacket(): Promise<DemuxedPacketCursor | null> {
      const segment = await this.ensureSegmentLoaded(0, this.activeWindowGeneration);
    return segment.audioPackets.length > 0 ? this.makeCursor(0, 0, segment.audioPackets) : null;
  }

  private async getAudioPacketForTime(timeMs: number): Promise<DemuxedPacketCursor | null> {
    if (!this.manifest) {
      return null;
    }

    const segmentIndex = this.findSegmentIndex(timeMs);
      const windowGeneration = this.activeWindowGeneration;
      const segment = await this.ensureSegmentLoaded(segmentIndex, windowGeneration);
    const packetIndex = findPacketIndex(segment.audioPackets, timeMs, false);
      if (packetIndex >= 0 && this.isActiveWindow(windowGeneration)) {
        this.prefetchAround(segmentIndex, windowGeneration);
      }
    return packetIndex >= 0 ? this.makeCursor(segmentIndex, packetIndex, segment.audioPackets) : null;
  }

  private async getNextAudioPacket(cursor: DemuxedPacketCursor): Promise<DemuxedPacketCursor | null> {
    const token = cursor.token as PacketToken;
      const windowGeneration = this.activeWindowGeneration;
      const segment = await this.ensureSegmentLoaded(token.segmentIndex, windowGeneration);
    const nextPacketIndex = token.packetIndex + 1;
    if (nextPacketIndex < segment.audioPackets.length) {
      return this.makeCursor(token.segmentIndex, nextPacketIndex, segment.audioPackets);
    }

    if (!this.manifest || token.segmentIndex + 1 >= this.manifest.totalSegments) {
      return null;
    }

    const nextSegmentIndex = token.segmentIndex + 1;
    const nextWindowGeneration = this.activateWindow(nextSegmentIndex);
    const nextSegment = await this.ensureSegmentLoaded(nextSegmentIndex, nextWindowGeneration);
    if (this.isActiveWindow(nextWindowGeneration)) {
      this.prefetchAround(nextSegmentIndex, nextWindowGeneration);
      }
    return nextSegment.audioPackets.length > 0
      ? this.makeCursor(nextSegmentIndex, 0, nextSegment.audioPackets)
      : null;
  }

  private prefetchAround(anchorIndex: number, windowGeneration: number): void {
    if (!this.manifest) {
      return;
    }

    if (!this.isActiveWindow(windowGeneration)) {
      return;
    }

    // Prefetch forward
    for (let offset = 1; offset <= SEGMENT_PREFETCH_AHEAD; offset += 1) {
      const nextIndex = anchorIndex + offset;
      if (nextIndex >= this.manifest.totalSegments) {
        break;
      }
      if (this.loadedSegments.has(nextIndex) || this.pendingSegments.has(nextIndex)) {
        continue;
      }
      void this.ensureSegmentLoaded(nextIndex, windowGeneration).catch(() => {});
    }
    
    // Also prefetch backward when we're in the middle of the video
    if (anchorIndex > SEGMENT_PREFETCH_AHEAD) {
      for (let offset = 1; offset <= SEGMENT_PREFETCH_AHEAD; offset += 1) {
        const prevIndex = anchorIndex - offset;
        if (prevIndex < 0) {
          break;
        }
        if (this.loadedSegments.has(prevIndex) || this.pendingSegments.has(prevIndex)) {
          continue;
        }
        void this.ensureSegmentLoaded(prevIndex, windowGeneration).catch(() => {});
      }
    }
  }

  private pruneLoadedSegments(anchorIndex: number): void {
    const minIndex = Math.max(0, anchorIndex - SEGMENT_RETENTION_BEHIND);
    const maxIndex = anchorIndex + SEGMENT_RETENTION_AHEAD;
    for (const index of Array.from(this.loadedSegments.keys())) {
      if (index < minIndex || index > maxIndex) {
        this.loadedSegments.delete(index);
      }
    }
  }

  private findSegmentIndex(timeMs: number): number {
    if (!this.manifest) {
      return 0;
    }

    for (let index = 0; index < this.manifest.segments.length; index += 1) {
      const segment = this.manifest.segments[index];
      const startMs = segment.startTime * 1000;
      const endMs = startMs + (segment.durationSeconds * 1000);
      if (timeMs < endMs) {
        return index;
      }
    }

    return Math.max(0, this.manifest.segments.length - 1);
  }

  private makeCursor(segmentIndex: number, packetIndex: number, packets: SampleInfo[]): DemuxedPacketCursor | null {
    const packet = packets[packetIndex];
    if (!packet) {
      return null;
    }

    return {
      token: { segmentIndex, packetIndex },
      packet,
    };
  }

  private activateWindow(anchorIndex: number): number {
    if (this.activeWindowGeneration == 0 || this.currentAnchorIndex !== anchorIndex) {
      this.anchorRequests$.next(anchorIndex);
    }
    return this.activeWindowGeneration;
  }

  private isActiveWindow(windowGeneration: number): boolean {
    return windowGeneration === this.activeWindowGeneration;
  }

  private abortPendingOutsideWindow(anchorIndex: number): void {
    const minIndex = Math.max(0, anchorIndex - SEGMENT_RETENTION_BEHIND);
    const maxIndex = anchorIndex + SEGMENT_RETENTION_AHEAD + SEGMENT_PREFETCH_AHEAD;
    for (const [index, controller] of this.pendingSegmentControllers.entries()) {
      if (index < minIndex || index > maxIndex) {
        controller.abort();
      }
    }
  }

  private dispose(): void {
    if (this.loadController) {
      this.loadController.abort();
      this.loadController = null;
    }
    this.disposed = true;
    this.activeWindowGeneration += 1;
    for (const controller of this.pendingSegmentControllers.values()) {
      controller.abort();
    }
    this.pendingSegmentControllers.clear();
    this.pendingSegments.clear();
    this.loadedSegments.clear();
    this.failedSegments.clear();
    this.currentAnchorIndex = 0;
    this.manifest = null;
    this.initSegment = null;
    this.metadata = null;
    this.decoderConfig = null;
    this.audioMeta = null;
  }
}

async function collectPackets(
  getFirstPacket: () => Promise<DemuxedPacketCursor | null>,
  getNextPacket: (cursor: DemuxedPacketCursor) => Promise<DemuxedPacketCursor | null>,
  timeOffsetMs: number,
): Promise<SampleInfo[]> {
  const packets: SampleInfo[] = [];
  const seenTimestamps = new Set<number>();
  let cursor = await getFirstPacket();
  
  while (cursor) {
    const timestampMs = cursor.packet.timestampMs + timeOffsetMs;
    // Skip perfectly duplicate frames that have the identical Presentation Timestamp
    if (!seenTimestamps.has(timestampMs)) {
      seenTimestamps.add(timestampMs);
      packets.push({
        ...cursor.packet,
        timestampMs,
      });
    } else {
      console.warn(`Skipping repetitive packet at ${timestampMs}ms`);
    }
    cursor = await getNextPacket(cursor);
  }
  return packets;
}

function getSegmentWindowEndMs(segments: SegmentedVideoManifestItem[], segmentIndex: number): number {
  const segment = segments[segmentIndex];
  const nextSegment = segments[segmentIndex + 1];
  const segmentEndMs = (segment.startTime + segment.durationSeconds) * 1000;
  if (!nextSegment) {
    return segmentEndMs;
  }
  return Math.min(segmentEndMs, nextSegment.startTime * 1000);
}

export function trimPacketsToSegmentWindow(
  packets: SampleInfo[],
  windowStartMs: number,
  windowEndMs: number,
): SampleInfo[] {
  return packets.filter((packet) => packet.timestampMs >= windowStartMs && packet.timestampMs < windowEndMs);
}

export function findPacketIndex(packets: SampleInfo[], timeMs: number, requireKeyframe: boolean): number {
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

function concatenateBuffers(first: ArrayBuffer, second: ArrayBuffer): ArrayBuffer {
  const combined = new Uint8Array(first.byteLength + second.byteLength);
  combined.set(new Uint8Array(first), 0);
  combined.set(new Uint8Array(second), first.byteLength);
  return combined.buffer;
}
