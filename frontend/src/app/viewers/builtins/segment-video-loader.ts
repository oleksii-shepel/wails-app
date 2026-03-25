import {
  DemuxedAudioTrackSource,
  DemuxedPacketCursor,
  DemuxedVideoSource,
  ExtendedVideoDecoderConfig,
  SampleInfo,
  VideoMetadata,
  loadMp4DemuxedSourceFromBuffer,
} from '../../services/canvas-video-player.service';

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

export class SegmentVideoLoader {
  private manifest: SegmentedVideoManifest | null = null;
  private initSegment: ArrayBuffer | null = null;
  private readonly loadedSegments = new Map<number, LoadedSegment>();
  private readonly pendingSegments = new Map<number, Promise<LoadedSegment>>();
  private metadata: VideoMetadata | null = null;
  private decoderConfig: ExtendedVideoDecoderConfig | null = null;
  private audioMeta: Omit<DemuxedAudioTrackSource, 'getFirstPacket' | 'getPacketForTime' | 'getNextPacket'> | null = null;

  async load(manifestUrl: string): Promise<DemuxedVideoSource> {
    const manifestResponse = await fetch(manifestUrl);
    if (!manifestResponse.ok) {
      throw new Error(`Failed to load segment manifest: ${manifestResponse.status}`);
    }

    this.manifest = await manifestResponse.json() as SegmentedVideoManifest;

    const initResponse = await fetch(this.manifest.initSegmentUrl);
    if (!initResponse.ok) {
      throw new Error(`Failed to load init segment: ${initResponse.status}`);
    }
    this.initSegment = await initResponse.arrayBuffer();

    const firstSegment = await this.ensureSegmentLoaded(0);
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
    };
  }

  private async ensureSegmentLoaded(index: number): Promise<LoadedSegment> {
    const existing = this.loadedSegments.get(index);
    if (existing) {
      return existing;
    }

    const pending = this.pendingSegments.get(index);
    if (pending) {
      return pending;
    }

    const loadPromise = this.loadSegment(index);
    this.pendingSegments.set(index, loadPromise);
    try {
      const segment = await loadPromise;
      this.loadedSegments.set(index, segment);
      return segment;
    } finally {
      this.pendingSegments.delete(index);
    }
  }

  private async loadSegment(index: number): Promise<LoadedSegment> {
    if (!this.manifest || !this.initSegment) {
      throw new Error('Segment loader not initialized.');
    }

    const segment = this.manifest.segments[index];
    if (!segment) {
      throw new Error(`Segment ${index} is not available.`);
    }

    const segmentResponse = await fetch(segment.url);
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

    const startOffsetMs = Math.round(segment.startTime * 1000);
    const videoPackets = await collectPackets(
      () => source.getFirstPacket(),
      (cursor) => source.getNextPacket(cursor),
      startOffsetMs,
    );
    const audioPackets = source.audio
      ? await collectPackets(
          () => source.audio!.getFirstPacket(),
          (cursor) => source.audio!.getNextPacket(cursor),
          startOffsetMs,
        )
      : [];
    source.destroy?.();

    return { index, videoPackets, audioPackets };
  }

  private async getPacketForTime(timeMs: number, options?: { keyframe?: boolean }): Promise<DemuxedPacketCursor | null> {
    if (!this.manifest) {
      return null;
    }

    let segmentIndex = this.findSegmentIndex(timeMs);
    while (segmentIndex >= 0) {
      const segment = await this.ensureSegmentLoaded(segmentIndex);
      const packetIndex = findPacketIndex(segment.videoPackets, timeMs, options?.keyframe === true);
      if (packetIndex >= 0) {
        return this.makeCursor(segmentIndex, packetIndex, segment.videoPackets);
      }
      if (!options?.keyframe) {
        break;
      }
      segmentIndex -= 1;
    }
    return null;
  }

  private async getNextPacket(cursor: DemuxedPacketCursor): Promise<DemuxedPacketCursor | null> {
    const token = cursor.token as PacketToken;
    const segment = await this.ensureSegmentLoaded(token.segmentIndex);
    const nextPacketIndex = token.packetIndex + 1;
    if (nextPacketIndex < segment.videoPackets.length) {
      return this.makeCursor(token.segmentIndex, nextPacketIndex, segment.videoPackets);
    }

    if (!this.manifest || token.segmentIndex + 1 >= this.manifest.totalSegments) {
      return null;
    }

    const nextSegment = await this.ensureSegmentLoaded(token.segmentIndex + 1);
    return nextSegment.videoPackets.length > 0
      ? this.makeCursor(token.segmentIndex + 1, 0, nextSegment.videoPackets)
      : null;
  }

  private async getFirstAudioPacket(): Promise<DemuxedPacketCursor | null> {
    const segment = await this.ensureSegmentLoaded(0);
    return segment.audioPackets.length > 0 ? this.makeCursor(0, 0, segment.audioPackets) : null;
  }

  private async getAudioPacketForTime(timeMs: number): Promise<DemuxedPacketCursor | null> {
    if (!this.manifest) {
      return null;
    }

    const segmentIndex = this.findSegmentIndex(timeMs);
    const segment = await this.ensureSegmentLoaded(segmentIndex);
    const packetIndex = findPacketIndex(segment.audioPackets, timeMs, false);
    return packetIndex >= 0 ? this.makeCursor(segmentIndex, packetIndex, segment.audioPackets) : null;
  }

  private async getNextAudioPacket(cursor: DemuxedPacketCursor): Promise<DemuxedPacketCursor | null> {
    const token = cursor.token as PacketToken;
    const segment = await this.ensureSegmentLoaded(token.segmentIndex);
    const nextPacketIndex = token.packetIndex + 1;
    if (nextPacketIndex < segment.audioPackets.length) {
      return this.makeCursor(token.segmentIndex, nextPacketIndex, segment.audioPackets);
    }

    if (!this.manifest || token.segmentIndex + 1 >= this.manifest.totalSegments) {
      return null;
    }

    const nextSegment = await this.ensureSegmentLoaded(token.segmentIndex + 1);
    return nextSegment.audioPackets.length > 0
      ? this.makeCursor(token.segmentIndex + 1, 0, nextSegment.audioPackets)
      : null;
  }

  private findSegmentIndex(timeMs: number): number {
    if (!this.manifest) {
      return 0;
    }

    for (let index = 0; index < this.manifest.segments.length; index += 1) {
      const segment = this.manifest.segments[index];
      const startMs = Math.round(segment.startTime * 1000);
      const endMs = startMs + Math.round(segment.durationSeconds * 1000);
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
}

async function collectPackets(
  getFirstPacket: () => Promise<DemuxedPacketCursor | null>,
  getNextPacket: (cursor: DemuxedPacketCursor) => Promise<DemuxedPacketCursor | null>,
  timeOffsetMs: number,
): Promise<SampleInfo[]> {
  const packets: SampleInfo[] = [];
  let cursor = await getFirstPacket();
  while (cursor) {
    packets.push({
      ...cursor.packet,
      timestampMs: cursor.packet.timestampMs + timeOffsetMs,
    });
    cursor = await getNextPacket(cursor);
  }
  return packets;
}

function findPacketIndex(packets: SampleInfo[], timeMs: number, requireKeyframe: boolean): number {
  let bestIndex = -1;
  for (let index = 0; index < packets.length; index += 1) {
    const packet = packets[index];
    if (!requireKeyframe && packet.timestampMs >= timeMs) {
      return index;
    }
    if (packet.timestampMs <= timeMs && (!requireKeyframe || packet.isRap)) {
      bestIndex = index;
    }
  }
  if (bestIndex >= 0) {
    return bestIndex;
  }
  return requireKeyframe ? packets.findIndex(packet => packet.isRap) : (packets.length > 0 ? 0 : -1);
}

function concatenateBuffers(first: ArrayBuffer, second: ArrayBuffer): ArrayBuffer {
  const combined = new Uint8Array(first.byteLength + second.byteLength);
  combined.set(new Uint8Array(first), 0);
  combined.set(new Uint8Array(second), first.byteLength);
  return combined.buffer;
}