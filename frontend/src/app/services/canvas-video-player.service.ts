import { Injectable } from '@angular/core';
import { createBehaviorSubject, createSubject } from '@epikodelabs/streamix';
import { BufferSource, EncodedPacketSink, Input, MATROSKA, MP4, MPEG_TS, QTFF, UrlSource, WEBM, ReadableStreamSource } from 'mediabunny';

/**
 * All mediabunny format parsers - try these in order for generic format handling.
 * This allows us to support any format that mediabunny can demux.
 */
const MEDIABUNNY_FORMAT_PARSERS = [MP4, QTFF, MATROSKA, WEBM, MPEG_TS];

export interface VideoMetadata {
  width: number;
  height: number;
  duration: number;
  format: string;
  videoCodec?: string;
  audioCodec?: string;
  bitrate?: number;
  frameRate?: number;
  hasAudio: boolean;
  hasVideo: boolean;
}

type ContainerFormat = 'mp4' | 'mkv' | 'ts' | 'unknown';

type PlayerState = 'idle' | 'paused' | 'playing' | 'ended';

export type ExtendedVideoDecoderConfig = VideoDecoderConfig & {
  avc?: {
    format: 'annexb' | 'avc';
  };
};

interface TrackInfo {
  id: number;
  codec: string;
  timescale: number;
  duration: number;
  movie_duration?: number;
  movie_timescale?: number;
  samples_duration?: number;
  sample_duration?: number;
  bitrate?: number;
  nb_samples: number;
  track_width?: number;
  track_height?: number;
  video?: {
    width?: number;
    height?: number;
  };
}

export interface SampleInfo {
  index: number;
  timestampMs: number;
  durationMs: number;
  isRap: boolean;
  data: Uint8Array;
}

interface DecodedFrame {
  timestampMs: number;
  frame: VideoFrame;
}

interface AvcSampleConfig {
  lengthSize: number;
  sps: Uint8Array[];
  pps: Uint8Array[];
}

export interface DemuxedPacketCursor {
  token: unknown;
  packet: SampleInfo;
}

export interface DemuxedAudioTrackSource {
  decoderConfig: AudioDecoderConfig;
  codec?: string;
  sampleRate: number;
  numberOfChannels: number;
  getFirstPacket(): Promise<DemuxedPacketCursor | null>;
  getPacketForTime(timeMs: number): Promise<DemuxedPacketCursor | null>;
  getNextPacket(cursor: DemuxedPacketCursor): Promise<DemuxedPacketCursor | null>;
}

export interface DemuxedVideoSource {
  metadata: VideoMetadata;
  decoderConfig: ExtendedVideoDecoderConfig;
  avcSampleConfig?: AvcSampleConfig;
  audio?: DemuxedAudioTrackSource;
  supportsRandomAccess?: boolean;
  getFirstPacket(): Promise<DemuxedPacketCursor | null>;
  getPacketForTime(timeMs: number, options?: { keyframe?: boolean }): Promise<DemuxedPacketCursor | null>;
  getNextPacket(cursor: DemuxedPacketCursor): Promise<DemuxedPacketCursor | null>;
  destroy?(): void;
}

class Mp4DemuxedSource {
  static async load(buffer: ArrayBuffer): Promise<DemuxedVideoSource> {
    this.ensureMp4(buffer);
    return MediaBunnyDemuxedSource.loadFromBuffer(buffer, [MP4, QTFF], (inputFormat) => inputFormat === QTFF ? 'mov' : 'mp4');
  }

  static async loadFromUrl(url: string, durationHintSec = 0): Promise<DemuxedVideoSource> {
    return MediaBunnyDemuxedSource.loadFromUrl(url, [MP4, QTFF], (inputFormat) => inputFormat === QTFF ? 'mov' : 'mp4', durationHintSec);
  }

  private static ensureMp4(buffer: ArrayBuffer): void {
    const bytes = new Uint8Array(buffer.slice(0, 12));
    const boxType = String.fromCharCode(bytes[4], bytes[5], bytes[6], bytes[7]);
    if (boxType !== 'ftyp' && boxType !== 'moov' && boxType !== 'mdat') {
      throw new Error('Canvas playback currently supports MP4/MOV sources only.');
    }
  }
}

export async function loadMp4DemuxedSourceFromBuffer(buffer: ArrayBuffer): Promise<DemuxedVideoSource> {
  return Mp4DemuxedSource.load(buffer);
}

class ArrayBackedDemuxedSource {
  static create(options: {
    metadata: VideoMetadata;
    decoderConfig: ExtendedVideoDecoderConfig;
    avcSampleConfig?: AvcSampleConfig;
    samples: SampleInfo[];
    destroy?(): void;
  }): DemuxedVideoSource {
    const keyframeSampleIndexes = options.samples
      .filter((sample) => sample.isRap)
      .map((sample) => sample.index);

    const getCursorAt = (index: number): DemuxedPacketCursor | null => {
      const packet = options.samples[index];
      return packet ? { token: index, packet } : null;
    };

    return {
      metadata: options.metadata,
      decoderConfig: options.decoderConfig,
      avcSampleConfig: options.avcSampleConfig,
      supportsRandomAccess: true,
      getFirstPacket: async () => getCursorAt(0),
      getPacketForTime: async (timeMs, requestOptions) => {
        if (!options.samples.length) {
          return null;
        }

        if (requestOptions?.keyframe) {
          let selectedIndex = keyframeSampleIndexes[0] ?? 0;
          for (const keyframeIndex of keyframeSampleIndexes) {
            if (options.samples[keyframeIndex].timestampMs > timeMs) {
              break;
            }
            selectedIndex = keyframeIndex;
          }
          return getCursorAt(selectedIndex);
        }

        for (let index = 0; index < options.samples.length; index += 1) {
          if (options.samples[index].timestampMs >= timeMs) {
            return getCursorAt(index);
          }
        }

        return getCursorAt(options.samples.length - 1);
      },
      getNextPacket: async (cursor) => getCursorAt((cursor.token as number) + 1),
      destroy: options.destroy,
    };
  }
}

class MediaBunnyDemuxedSource {
  static async loadFromUrl(
    url: string,
    formats: unknown[],
    resolveFormatName: (inputFormat: unknown) => string,
    durationHintSec = 0,
  ): Promise<DemuxedVideoSource> {
    const controller = new AbortController();
    const response = await fetch(url, {
      headers: { Range: 'bytes=0-' },
      signal: controller.signal
    });

    if (!response.ok) {
      throw new Error(`Failed to stream from URL (status ${response.status})`);
    }

    const isPartial = response.status === 206;
    const hasContentLength = response.headers.has('Content-Length');

    // If server supports ranges or knows final size, use UrlSource to allow proper random-access seeking.
    // This solves issues with MKV files that need to parse cue metadata at the end of the file.
    if (isPartial || hasContentLength) {
      controller.abort();
      const source = await this.loadInput(new UrlSource(url, {
        maxCacheSize: 64 * 1024 * 1024,
        parallelism: 2,
      }), formats, resolveFormatName, durationHintSec);
      source.supportsRandomAccess = true;
      return source;
    }

    if (!response.body) {
      throw new Error('Failed to read response body for sequential stream');
    }

    // Server doesn't support ranges or size (e.g., live transcode or piped stream).
    // Use ReadableStreamSource to incrementally consume the stream, with a large cache
    // to handle minimal backwards-seeking requirements.
    const source = await this.loadInput(new ReadableStreamSource(response.body, {
      maxCacheSize: 256 * 1024 * 1024, // 256MB cache
    }), formats, resolveFormatName, durationHintSec);
    source.supportsRandomAccess = false;
    return source;
  }

  static async loadFromBuffer(
    buffer: ArrayBuffer,
    formats: unknown[],
    resolveFormatName: (inputFormat: unknown) => string,
  ): Promise<DemuxedVideoSource> {
    return this.loadInput(new BufferSource(buffer), formats, resolveFormatName);
  }

  private static async loadInput(
    source: BufferSource | UrlSource | ReadableStreamSource,
    formats: unknown[],
    resolveFormatName: (inputFormat: unknown) => string,
    durationHintSec = 0,
  ): Promise<DemuxedVideoSource> {
    const input = new Input({
      formats: formats as any,
      source,
    });
    const normalizeFormatName = (inputFormat: unknown) => resolveFormatName(inputFormat).toUpperCase();

    try {
      const [inputFormat, videoTrack, audioTrack, inputDurationSec] = await Promise.all([
        input.getFormat(),
        input.getPrimaryVideoTrack(),
        input.getPrimaryAudioTrack(),
        durationHintSec > 0 ? Promise.resolve(durationHintSec) : input.computeDuration(),
      ]);

      if (!videoTrack) {
        throw new Error(`No decodable video track was found in this ${normalizeFormatName(inputFormat)} file.`);
      }

      const packetSink = new EncodedPacketSink(videoTrack);
      const audioPacketSink = audioTrack ? new EncodedPacketSink(audioTrack) : null;

      const [decoderConfig, videoCodec, audioCodec, audioDecoderConfig, stats, trackDurationSec, firstPacket] = await Promise.all([
        videoTrack.getDecoderConfig(),
        videoTrack.getCodecParameterString(),
        audioTrack?.getCodecParameterString() ?? Promise.resolve(null),
        audioTrack?.getDecoderConfig() ?? Promise.resolve(null),
        videoTrack.computePacketStats(120),
        durationHintSec > 0 ? Promise.resolve(durationHintSec) : videoTrack.computeDuration(),
        packetSink.getFirstPacket({ metadataOnly: true }),
      ]);

      if (!decoderConfig) {
        const codecLabel = videoCodec ?? String(videoTrack.internalCodecId ?? 'unknown');
        throw new Error(`Unsupported ${normalizeFormatName(inputFormat)} video codec for WebCodecs: ${codecLabel}`);
      }

      const normalizedDecoderConfig: ExtendedVideoDecoderConfig = {
        ...decoderConfig,
        optimizeForLatency: true,
      };

      if (!await this.canUseDecoderConfig(normalizedDecoderConfig)) {
        throw new Error(`This runtime cannot configure WebCodecs for ${videoCodec ?? normalizedDecoderConfig.codec}.`);
      }

      const timestampOffsetMs = Math.max(0, firstPacket ? this.toMilliseconds(firstPacket.timestamp) : 0);
      const durationMs = Math.max(
        0,
        durationHintSec > 0 ? this.toMilliseconds(durationHintSec) : this.toMilliseconds(inputDurationSec),
        this.toMilliseconds(trackDurationSec) - timestampOffsetMs,
      );

      const firstCursor = await this.getFirstUsableCursor(packetSink, timestampOffsetMs);
      if (!firstCursor) {
        throw new Error(`The ${normalizeFormatName(inputFormat)} file has a video track, but no decodable packets were extracted.`);
      }

      const frameRate = Number.isFinite(stats.averagePacketRate)
        ? Math.max(1, Math.round(stats.averagePacketRate))
        : 0;
      const bitrate = Number.isFinite(stats.averageBitrate)
        ? Math.max(0, Math.round(stats.averageBitrate))
        : undefined;
      const metadata: VideoMetadata = {
        width: videoTrack.displayWidth || videoTrack.codedWidth || 1280,
        height: videoTrack.displayHeight || videoTrack.codedHeight || 720,
        duration: durationMs,
        format: resolveFormatName(inputFormat),
        videoCodec: videoCodec ?? String(videoTrack.internalCodecId ?? normalizedDecoderConfig.codec),
        audioCodec: audioCodec ?? undefined,
        bitrate,
        frameRate,
        hasAudio: Boolean(audioTrack),
        hasVideo: true,
      };

      let demuxedAudioTrack: DemuxedAudioTrackSource | undefined;
      if (audioTrack && audioPacketSink && audioDecoderConfig && await this.canUseAudioDecoderConfig(audioDecoderConfig)) {
        demuxedAudioTrack = {
          decoderConfig: audioDecoderConfig,
          codec: audioCodec ?? undefined,
          sampleRate: audioTrack.sampleRate,
          numberOfChannels: audioTrack.numberOfChannels,
          getFirstPacket: async () => this.getFirstUsableCursor(audioPacketSink, timestampOffsetMs),
          getPacketForTime: async (timeMs) => this.getCursorForTime(audioPacketSink, timeMs, timestampOffsetMs),
          getNextPacket: async (cursor) => this.getNextCursor(audioPacketSink, cursor, timestampOffsetMs),
        };
      }

      return {
        metadata,
        decoderConfig: normalizedDecoderConfig,
        audio: demuxedAudioTrack,
        getFirstPacket: async () => this.getFirstUsableCursor(packetSink, timestampOffsetMs),
        getPacketForTime: async (timeMs, options) => this.getCursorForTime(packetSink, timeMs, timestampOffsetMs, options),
        getNextPacket: async (cursor) => this.getNextCursor(packetSink, cursor, timestampOffsetMs),
        destroy: () => input.dispose(),
      };
    } catch (error) {
      input.dispose();
      throw error;
    }
  }

  private static async getFirstUsableCursor(packetSink: EncodedPacketSink, timestampOffsetMs: number): Promise<DemuxedPacketCursor | null> {
    const packet = await packetSink.getFirstPacket();
    return this.findUsableCursor(packetSink, packet, timestampOffsetMs);
  }

  private static async getCursorForTime(
    packetSink: EncodedPacketSink,
    timeMs: number,
    timestampOffsetMs: number,
    options?: { keyframe?: boolean },
  ): Promise<DemuxedPacketCursor | null> {
    const targetSeconds = (timeMs + timestampOffsetMs) / 1000;
    const packet = options?.keyframe
      ? await packetSink.getKeyPacket(targetSeconds, { verifyKeyPackets: true })
      : await packetSink.getPacket(targetSeconds);
    const fallback = options?.keyframe
      ? await packetSink.getFirstKeyPacket({ verifyKeyPackets: true })
      : await packetSink.getFirstPacket();

    return this.findUsableCursor(packetSink, packet ?? fallback, timestampOffsetMs);
  }

  private static async getNextCursor(
    packetSink: EncodedPacketSink,
    cursor: DemuxedPacketCursor,
    timestampOffsetMs: number,
  ): Promise<DemuxedPacketCursor | null> {
    const packet = await packetSink.getNextPacket(cursor.token as any);
    return this.findUsableCursor(packetSink, packet, timestampOffsetMs);
  }

  private static async findUsableCursor(
    packetSink: EncodedPacketSink,
    packet: any,
    timestampOffsetMs: number,
  ): Promise<DemuxedPacketCursor | null> {
    let currentPacket = packet;

    while (currentPacket) {
      const normalizedPacket = this.normalizePacket(currentPacket, timestampOffsetMs);
      if (normalizedPacket.timestampMs + normalizedPacket.durationMs > 0) {
        return {
          token: currentPacket,
          packet: normalizedPacket,
        };
      }

      currentPacket = await packetSink.getNextPacket(currentPacket);
    }

    return null;
  }

  private static normalizePacket(packet: any, timestampOffsetMs: number): SampleInfo {
    const packetTimestampMs = this.toMilliseconds(packet.timestamp);
    const packetDurationMs = Math.max(1, this.toMilliseconds(packet.duration));

    return {
      index: packet.sequenceNumber ?? -1,
      timestampMs: Math.max(0, packetTimestampMs - timestampOffsetMs),
      durationMs: packetDurationMs,
      isRap: packet.type === 'key',
      data: packet.data.slice(),
    };
  }

  private static async canUseDecoderConfig(config: VideoDecoderConfig): Promise<boolean> {
    try {
      if (typeof VideoDecoder.isConfigSupported === 'function') {
        const support = await VideoDecoder.isConfigSupported(config);
        if (support.supported) {
          return true;
        }
      }
    } catch {
      // Fall through to a configure probe for embedded runtimes.
    }

    try {
      const probe = new VideoDecoder({
        output: (frame) => frame.close(),
        error: () => undefined,
      });
      probe.configure(config);
      probe.close();
      return true;
    } catch {
      return false;
    }
  }

  private static async canUseAudioDecoderConfig(config: AudioDecoderConfig): Promise<boolean> {
    if (!("AudioDecoder" in window)) {
      return false;
    }

    try {
      if (typeof AudioDecoder.isConfigSupported === 'function') {
        const support = await AudioDecoder.isConfigSupported(config);
        if (support.supported) {
          return true;
        }
      }
    } catch {
      // Fall through to a configure probe for embedded runtimes.
    }

    try {
      const probe = new AudioDecoder({
        output: (audioData) => audioData.close(),
        error: () => undefined,
      });
      probe.configure(config);
      probe.close();
      return true;
    } catch {
      return false;
    }
  }

  private static toMilliseconds(valueSec: number): number {
    return Math.round(valueSec * 1000);
  }
}

class MkvDemuxedSource {
  static async load(buffer: ArrayBuffer): Promise<DemuxedVideoSource> {
    this.ensureMatroska(buffer);

    return MediaBunnyDemuxedSource.loadFromBuffer(buffer, [MATROSKA, WEBM], (inputFormat) => inputFormat === WEBM ? 'webm' : 'mkv');
  }

  static async loadFromUrl(url: string, durationHintSec = 0): Promise<DemuxedVideoSource> {
    return MediaBunnyDemuxedSource.loadFromUrl(url, [MATROSKA, WEBM], (inputFormat) => inputFormat === WEBM ? 'webm' : 'mkv', durationHintSec);
  }

  private static ensureMatroska(buffer: ArrayBuffer): void {
    const bytes = new Uint8Array(buffer.slice(0, 4));
    if (bytes.byteLength < 4 || bytes[0] !== 0x1a || bytes[1] !== 0x45 || bytes[2] !== 0xdf || bytes[3] !== 0xa3) {
      throw new Error('Canvas playback expected a Matroska/WebM source, but the file does not start with an EBML header.');
    }
  }
}

class TsDemuxedSource {
  static async load(buffer: ArrayBuffer): Promise<DemuxedVideoSource> {
    this.ensureTsFormat(buffer);

    return MediaBunnyDemuxedSource.loadFromBuffer(buffer, [MPEG_TS], () => 'ts');
  }

  static async loadFromUrl(url: string, durationHintSec = 0): Promise<DemuxedVideoSource> {
    return MediaBunnyDemuxedSource.loadFromUrl(url, [MPEG_TS], () => 'ts', durationHintSec);
  }

  private static ensureTsFormat(buffer: ArrayBuffer): void {
    const bytes = new Uint8Array(buffer.slice(0, 1));
    // TS/M2TS streams typically start with sync byte 0x47
    if (bytes.byteLength < 1 || bytes[0] !== 0x47) {
      throw new Error('Canvas playback expected an MPEG-TS source, but the file does not start with a TS sync byte.');
    }
  }
}

/**
 * Generic demuxer that tries all mediabunny format parsers.
 * This allows us to support any format mediabunny can handle, including AVI, FLV, MXF, etc.
 */
class GenericMediaBunnyDemuxedSource {
  static async load(buffer: ArrayBuffer): Promise<DemuxedVideoSource> {
    return MediaBunnyDemuxedSource.loadFromBuffer(buffer, MEDIABUNNY_FORMAT_PARSERS, (inputFormat) => {
      // Return a friendly format name based on the detected format
      if (inputFormat === MP4 || inputFormat === QTFF) return 'mp4';
      if (inputFormat === MATROSKA) return 'mkv';
      if (inputFormat === WEBM) return 'webm';
      if (inputFormat === MPEG_TS) return 'ts';
      return 'unknown';
    });
  }

  static async loadFromUrl(url: string, durationHintSec = 0): Promise<DemuxedVideoSource> {
    return MediaBunnyDemuxedSource.loadFromUrl(url, MEDIABUNNY_FORMAT_PARSERS, (inputFormat) => {
      if (inputFormat === MP4 || inputFormat === QTFF) return 'mp4';
      if (inputFormat === MATROSKA) return 'mkv';
      if (inputFormat === WEBM) return 'webm';
      if (inputFormat === MPEG_TS) return 'ts';
      return 'unknown';
    }, durationHintSec);
  }
}

@Injectable({ providedIn: 'root' })
export class CanvasVideoPlayerService {
  /** All video extensions that WebCodecs can handle (via mediabunny demuxing) */
  private static readonly DEMUXER_BACKED_EXTENSIONS = new Set([
    // MP4 family
    'mp4', 'm4v', 'mov',
    // Matroska/WebM family
    'mkv', 'webm',
    // MPEG-TS family
    'ts', 'mts', 'm2ts',
    // Other direct formats supported by mediabunny
    'avi', 'flv', 'mxf', 'ogv',
    // Legacy formats routed through FFmpeg transcoding
    'divx', 'xvid', 'mpg', 'mpeg', 'vob', 'm2p', 'm2v', 'qt', 'mjpeg', 'mjpg'
  ]);

  private static readonly FFMPEG_TRANSCODE_EXTENSIONS = new Set([
    'avi', 'divx', 'xvid',
    'flv',
    'mov', 'qt',
    'mpg', 'mpeg', 'vob', 'm2p', 'm2v',
    'ts', 'mts', 'm2ts',
    'mjpeg', 'mjpg',
  ]);

  async createPlayer(container: HTMLElement): Promise<CanvasVideoPlayer> {
    return new CanvasVideoPlayer(container);
  }

  supportsCanvasVideoFormat(ext: string): boolean {
    return CanvasVideoPlayerService.DEMUXER_BACKED_EXTENSIONS.has(ext.toLowerCase());
  }

  requiresTranscodedPreview(ext: string): boolean {
    return CanvasVideoPlayerService.FFMPEG_TRANSCODE_EXTENSIONS.has(ext.toLowerCase());
  }

  getUnsupportedVideoMessage(ext: string): string {
    const normalizedExt = ext.toLowerCase();
    const upperExt = normalizedExt.toUpperCase();

    return `${upperExt} is detected as a video file, but the browser's WebCodecs API does not support the video codec used in this file, or the container format is not recognized by mediabunny. Try with a different video or contact support if you believe this is an error.`;
  }
}

export class CanvasVideoPlayer {
  private readonly canvas: HTMLCanvasElement;
  private readonly ctx: CanvasRenderingContext2D;

  private source?: DemuxedVideoSource;
  private decoder?: VideoDecoder;
  private decoderConfig?: ExtendedVideoDecoderConfig;
  private audioSource?: DemuxedAudioTrackSource;
  private audioDecoder?: AudioDecoder;
  private audioDecoderConfig?: AudioDecoderConfig;
  private audioContext?: AudioContext;
  private audioGainNode?: GainNode;
  private metadata?: VideoMetadata;
  private sourceUrl: string | null = null;
  private sourceSupportsRandomAccess = true;
  
  /** Max decoded frames - adaptive based on resolution to limit memory usage */
  private static calculateMaxDecodedFrames(width: number, height: number): number {
    // Each frame consumes approximately (width * height * 4 bytes) for RGBA
    // Estimate frame size in MB
    const frameSizeMB = (width * height * 4) / (1024 * 1024);
    
    // Target: Keep total decoded frame buffer under ~256 MB (reasonable for modern browsers)
    const targetBufferMB = 256;
    const maxFrames = Math.floor(targetBufferMB / frameSizeMB);
    
    // Clamp between 12 and 90 frames for safety
    // Minimum 12 frames (~0.4s at 30fps) for smooth playback
    // Maximum 90 frames (~3s at 30fps) to prevent excessive memory use
    return Math.max(12, Math.min(90, maxFrames));
  }
  
  private static readonly MAX_DECODE_QUEUE = 48;
  private static readonly MAX_AUDIO_DECODE_QUEUE = 24;
  private static readonly AUDIO_SCHEDULE_LEAD_SEC = 0.02;
  private static readonly AUDIO_BUFFER_AHEAD_SEC = 1.25;
  private maxDecodedFrames = 60; // Updated when video loads
  private decodedFrames: DecodedFrame[] = [];
  private nextPacketCursor: DemuxedPacketCursor | null = null;
  private nextAudioPacketCursor: DemuxedPacketCursor | null = null;
  private reachedEndOfStream = false;
  private decodeAheadInFlight = false;
  private decodeAheadQueued = false;
  private decodeGeneration = 0;
  private audioDecodeAheadInFlight = false;
  private audioDecodeAheadQueued = false;
  private awaitingKeyFrame = true;
  private rafId: number | null = null;
  private playbackBaseMs = 0;
  private playbackStartPerfMs = 0;
  private audioPlaybackStartContextSec = 0;
  private scheduledAudioUntilSec = 0;
  private scheduledAudioNodes: Array<{ node: AudioBufferSourceNode; endTime: number }> = [];
  private volume = 1;
  private lastDrawnTimestampMs = -1;
  private isLoaded = false;
  private sourceLoadGeneration = 0;
  private seekGeneration = 0;
  private pendingLoad: Promise<VideoMetadata> | null = null;

  readonly stateChanged$ = createBehaviorSubject<PlayerState>('idle');
  readonly frameReady$ = createSubject<number>();
  readonly error$ = createSubject<string>();
  readonly metadataReady$ = createSubject<VideoMetadata>();
  readonly currentTime$ = createBehaviorSubject<number>(0);

  constructor(private readonly container: HTMLElement) {
    this.container.replaceChildren();
    this.canvas = document.createElement('canvas');
    this.canvas.style.width = '100%';
    this.canvas.style.height = '100%';
    this.canvas.style.display = 'block';
    this.ctx = this.requireContext(this.canvas);
    this.container.appendChild(this.canvas);
  }

  async loadFromUrl(url: string): Promise<VideoMetadata> {
    let currentLoad: Promise<VideoMetadata> | null = null;
    currentLoad = (async () => {
      const generation = ++this.sourceLoadGeneration;
      this.prepareForSourceReload();
      this.sourceUrl = url;
      const durationHintSec = this.parseDurationHintFromUrl(url);

      try {
        const hintedFormat = this.detectContainerFormatFromUrl(url);
        if (hintedFormat === 'mp4' || hintedFormat === 'mkv' || hintedFormat === 'ts') {
          return await this.loadFromStreamUrl(url, hintedFormat, generation, durationHintSec);
        }

        const buffer = await this.fetchFile(url);
        const containerFormat = this.detectContainerFormat(buffer, url);
        return await this.loadFromBuffer(buffer, containerFormat, generation);
      } catch (error) {
        if (generation !== this.sourceLoadGeneration) {
          throw error;
        }
        const message = error instanceof Error ? error.message : 'Failed to load video';
        this.error$.next(message);
        throw error;
      } finally {
        if (this.pendingLoad === currentLoad) {
          this.pendingLoad = null;
        }
      }
    })();

    this.pendingLoad = currentLoad;
    return currentLoad;
  }

  async loadFromDemuxedSource(source: DemuxedVideoSource): Promise<VideoMetadata> {
    const generation = ++this.sourceLoadGeneration;
    this.prepareForSourceReload();

    let currentLoad: Promise<VideoMetadata> | null = null;
    currentLoad = (async () => {
      try {
        return await this.initializeDemuxedSource(source, generation);
      } finally {
        if (this.pendingLoad === currentLoad) {
          this.pendingLoad = null;
        }
      }
    })();

    this.pendingLoad = currentLoad;
    return currentLoad;
  }

  async play(): Promise<void> {
    await this.ensureLoaded();

    if (this.stateChanged$.value === 'playing') {
      return;
    }

    if (this.playbackBaseMs >= this.getDurationMs()) {
      await this.seek(0);
    }

    await this.prepareAudioPlayback(this.playbackBaseMs);
    this.playbackStartPerfMs = performance.now();
    this.stateChanged$.next('playing');
    this.renderLoop();
  }

  pause(): void {
    if (this.stateChanged$.value !== 'playing') {
      return;
    }

    this.playbackBaseMs = this.getCurrentTimeMs();
    this.cancelRenderLoop();
    this.resetAudioPlaybackState();
    this.stateChanged$.next('paused');
    this.currentTime$.next(this.playbackBaseMs / 1000);
  }

  async seek(timeMs: number): Promise<void> {
    await this.ensureLoaded();

    const seekGeneration = ++this.seekGeneration;

    const clampedTimeMs = Math.max(0, Math.min(timeMs, this.getDurationMs()));
    const shouldResumePlayback = this.stateChanged$.value === 'playing' || this.stateChanged$.value === 'ended';
    const currentTimeMs = this.getCurrentTimeMs();
    if (this.stateChanged$.value === 'playing') {
      this.suspendPlaybackForSeek();
    }

    if (!this.isCurrentSeek(seekGeneration)) {
      return;
    }

    if (clampedTimeMs < currentTimeMs && await this.shouldReloadSourceForReverseSeek()) {
      if (!this.isCurrentSeek(seekGeneration)) {
        return;
      }
      await this.reloadCurrentStreamSource(clampedTimeMs, shouldResumePlayback);
      return;
    }

    this.decodeGeneration++;
    this.playbackBaseMs = clampedTimeMs;
    this.currentTime$.next(clampedTimeMs / 1000);
    await this.rebuildDecoder();
    if (!this.isCurrentSeek(seekGeneration)) {
      return;
    }
    this.clearDecodedFrames();
    this.resetAudioPlaybackState();
    this.nextPacketCursor = await this.getSeekCursor(clampedTimeMs);
    if (!this.isCurrentSeek(seekGeneration)) {
      return;
    }
    this.reachedEndOfStream = this.nextPacketCursor === null;
    await this.decodeAhead();
    if (!this.isCurrentSeek(seekGeneration)) {
      return;
    }
    this.drawForTime(clampedTimeMs);

    if (shouldResumePlayback) {
      await this.resumePlaybackAfterSeek();
    }
  }

  getCurrentTime(): number {
    return this.getCurrentTimeMs() / 1000;
  }

  getDuration(): number {
    return this.getDurationMs() / 1000;
  }

  getMetadata(): VideoMetadata | undefined {
    return this.metadata;
  }

  isPlaying(): boolean {
    return this.stateChanged$.value === 'playing';
  }

  setVolume(volume: number): void {
    this.volume = Math.max(0, Math.min(1, volume));
    if (this.audioGainNode) {
      this.audioGainNode.gain.value = this.volume;
    }
  }

  destroy(): void {
    this.seekGeneration++;
    this.cancelRenderLoop();
    this.clearDecodedFrames();
    this.resetAudioPlaybackState();
    this.source?.destroy?.();
    this.source = undefined;
    this.sourceUrl = null;
    this.decoder?.close();
    this.decoder = undefined;
    this.audioSource = undefined;
    this.audioDecoderConfig = undefined;
    if (this.audioGainNode) {
      this.audioGainNode.disconnect();
      this.audioGainNode = undefined;
    }
    if (this.audioContext) {
      void this.audioContext.close().catch(() => undefined);
      this.audioContext = undefined;
    }
    this.stateChanged$.complete();
    this.frameReady$.complete();
    this.error$.complete();
    this.metadataReady$.complete();
    this.currentTime$.complete();
    this.canvas.remove();
  }

  private async loadFromBuffer(buffer: ArrayBuffer, containerFormat: ContainerFormat, generation: number): Promise<VideoMetadata> {
    const source = await this.demuxBuffer(buffer, containerFormat);
    return this.initializeDemuxedSource(source, generation);
  }

  private async loadFromStreamUrl(url: string, containerFormat: ContainerFormat, generation: number, durationHintSec = 0): Promise<VideoMetadata> {
    const source = await this.demuxUrl(url, containerFormat, durationHintSec);
    return this.initializeDemuxedSource(source, generation, durationHintSec);
  }

  private async demuxBuffer(buffer: ArrayBuffer, containerFormat: ContainerFormat): Promise<DemuxedVideoSource> {
    switch (containerFormat) {
      case 'mp4':
        return Mp4DemuxedSource.load(buffer);
      case 'mkv':
        return MkvDemuxedSource.load(buffer);
      case 'ts':
        return TsDemuxedSource.load(buffer);
      case 'unknown':
        // Try all mediabunny format parsers for unknown containers
        return GenericMediaBunnyDemuxedSource.load(buffer);
      default:
        throw new Error('Canvas playback container format is unknown and could not be determined.');
    }
  }

  private async demuxUrl(url: string, containerFormat: ContainerFormat, durationHintSec = 0): Promise<DemuxedVideoSource> {
    switch (containerFormat) {
      case 'mp4':
        return Mp4DemuxedSource.loadFromUrl(url, durationHintSec);
      case 'mkv':
        return MkvDemuxedSource.loadFromUrl(url, durationHintSec);
      case 'ts':
        return TsDemuxedSource.loadFromUrl(url, durationHintSec);
      case 'unknown':
        // Try all mediabunny format parsers for unknown containers
        return GenericMediaBunnyDemuxedSource.loadFromUrl(url, durationHintSec);
      default:
        throw new Error('Canvas playback container format is unknown and could not be determined.');
    }
  }

  private async initializeDemuxedSource(source: DemuxedVideoSource, generation: number, durationHintSec = 0): Promise<VideoMetadata> {
    if (generation !== this.sourceLoadGeneration) {
      source.destroy?.();
      throw new Error('Video load superseded by a newer request.');
    }

    this.source?.destroy?.();
    this.source = source;
    this.canvas.width = source.metadata.width;
    this.canvas.height = source.metadata.height;
    
    // Calculate adaptive frame buffer based on resolution to prevent memory issues
    this.maxDecodedFrames = CanvasVideoPlayer.calculateMaxDecodedFrames(
      source.metadata.width,
      source.metadata.height
    );
    
    this.metadata = source.metadata;
    this.metadataReady$.next(this.metadata);
    this.decoderConfig = source.decoderConfig;
    this.audioSource = source.audio;
    this.audioDecoderConfig = source.audio?.decoderConfig;
    this.sourceSupportsRandomAccess = source.supportsRandomAccess !== false;

    await this.rebuildDecoder();

    if (generation !== this.sourceLoadGeneration) {
      source.destroy?.();
      throw new Error('Video load superseded by a newer request.');
    }

    this.isLoaded = true;
    this.nextPacketCursor = null;
    this.nextAudioPacketCursor = null;
    this.reachedEndOfStream = false;
    this.playbackBaseMs = 0;
    this.currentTime$.next(0);
    this.stateChanged$.next('paused');

    await this.seek(0);
    return this.metadata;
  }

  private prepareForSourceReload(): void {
    this.seekGeneration++;
    this.decodeGeneration++;
    this.cancelRenderLoop();
    this.clearDecodedFrames();
    this.resetAudioPlaybackState();
    this.source?.destroy?.();
    this.source = undefined;
    this.sourceUrl = null;
    this.isLoaded = false;
    this.nextPacketCursor = null;
    this.nextAudioPacketCursor = null;
    this.reachedEndOfStream = false;
    this.playbackBaseMs = 0;
    this.playbackStartPerfMs = 0;
    this.stateChanged$.next('paused');
  }

  private async rebuildDecoder(): Promise<void> {
    if (!this.decoderConfig) {
      return;
    }

    // Clear any previously decoded frames to free memory
    this.clearDecodedFrames();

    // Close old decoder - flush pending operations first to ensure clean shutdown
    if (this.decoder) {
      try {
        await this.decoder.flush();
      } catch {
        // Flush may fail if decoder is already closed or in error state
        // This is acceptable - we still want to close it
      }
      this.decoder.close();
    }

    this.decoder = new VideoDecoder({
      output: (frame) => this.onDecodedFrame(frame),
      error: (error) => {
        if (this.isExpectedKeyFrameRecoveryError(error)) {
          this.awaitingKeyFrame = true;
          return;
        }

        console.error('[WebCodecs] VideoDecoder error:', error);
        const message = error instanceof Error ? error.message : String(error);
        this.error$.next(`Video decode error: ${message}`);
      },
    });
    this.decoder.configure(this.decoderConfig);
    this.awaitingKeyFrame = true;
  }

  private async rebuildAudioDecoder(): Promise<void> {
    if (!this.audioDecoderConfig || !("AudioDecoder" in window)) {
      this.audioDecoder = undefined;
      return;
    }

    if (this.audioDecoder) {
      try {
        await this.audioDecoder.flush();
      } catch {
        // Ignore flush failures while replacing the decoder.
      }
      this.audioDecoder.close();
    }

    this.audioDecoder = new AudioDecoder({
      output: (audioData) => this.onDecodedAudio(audioData),
      error: (error) => {
        console.error('[WebCodecs] AudioDecoder error:', error);
        this.disableAudioPlayback();
      },
    });
    this.audioDecoder.configure(this.audioDecoderConfig);
  }

  private onDecodedFrame(frame: VideoFrame): void {
    if (this.canvas.width !== frame.displayWidth || this.canvas.height !== frame.displayHeight) {
      this.canvas.width = frame.displayWidth;
      this.canvas.height = frame.displayHeight;
    }

    const timestampMs = Math.round(frame.timestamp / 1000);
    this.decodedFrames.push({ timestampMs, frame });
    this.decodedFrames.sort((left, right) => left.timestampMs - right.timestampMs);

    while (this.decodedFrames.length > this.maxDecodedFrames) {
      const stale = this.decodedFrames.shift();
      stale?.frame.close();
    }

    this.frameReady$.next(timestampMs);

    // If playback is paused (e.g. during initial load or explicit seek),
    // the render loop is stopped, so we must manually draw incoming frames
    // or the canvas will remain empty black. Do not request more decode-ahead
    // here: during startup that causes a fetch cascade while the player is
    // still paused, repeatedly pulling future segments before playback even
    // begins.
    if (this.stateChanged$.value !== 'playing') {
      this.drawForTime(this.getCurrentTimeMs());
    }
  }

  private onDecodedAudio(audioData: AudioData): void {
    if (this.stateChanged$.value !== 'playing' || !this.audioContext || !this.audioGainNode) {
      audioData.close();
      return;
    }

    try {
      const audioBuffer = this.createAudioBuffer(audioData);
      const timestampSec = audioData.timestamp / 1_000_000;
      const playbackBaseSec = this.playbackBaseMs / 1000;
      const targetStartTime = this.audioPlaybackStartContextSec + (timestampSec - playbackBaseSec);
      const startTime = Math.max(targetStartTime, this.scheduledAudioUntilSec);
      audioData.close();

      if (startTime + audioBuffer.duration <= this.audioContext.currentTime) {
        return;
      }

      const sourceNode = this.audioContext.createBufferSource();
      sourceNode.buffer = audioBuffer;
      sourceNode.connect(this.audioGainNode);
      sourceNode.onended = () => {
        this.scheduledAudioNodes = this.scheduledAudioNodes.filter((entry) => entry.node !== sourceNode);
      };
      sourceNode.start(startTime);

      this.scheduledAudioNodes.push({
        node: sourceNode,
        endTime: startTime + audioBuffer.duration,
      });
      this.scheduledAudioUntilSec = Math.max(this.scheduledAudioUntilSec, startTime + audioBuffer.duration);
    } catch (error) {
      console.error('[WebCodecs] Failed to schedule decoded audio:', error);
      audioData.close();
      this.disableAudioPlayback();
    }
  }

  private renderLoop(): void {
    if (this.stateChanged$.value !== 'playing') {
      return;
    }

    const currentTimeMs = this.getPlaybackTimeMs();
    this.currentTime$.next(currentTimeMs / 1000);
    this.requestDecodeAhead();
    this.requestAudioDecodeAhead();
    this.drawForTime(currentTimeMs);

    if (currentTimeMs >= this.getDurationMs() && this.isPlaybackExhausted()) {
      this.playbackBaseMs = this.getDurationMs();
      this.cancelRenderLoop();
      this.stateChanged$.next('ended');
      return;
    }

    this.rafId = requestAnimationFrame(() => this.renderLoop());
  }

  private requestDecodeAhead(): void {
    if (this.decodeAheadInFlight) {
      this.decodeAheadQueued = true;
      return;
    }

    void this.decodeAhead();
  }

  private requestAudioDecodeAhead(): void {
    if (!this.audioDecoder || !this.audioSource || this.stateChanged$.value !== 'playing') {
      return;
    }

    if (this.audioDecodeAheadInFlight) {
      this.audioDecodeAheadQueued = true;
      return;
    }

    void this.decodeAudioAhead();
  }

  private async decodeAhead(): Promise<void> {
    if (this.decodeAheadInFlight || !this.decoder || !this.source || !this.nextPacketCursor) {
      return;
    }

    this.decodeAheadInFlight = true;
    const generation = this.decodeGeneration;

    try {
      while (
        this.nextPacketCursor &&
        this.decoder &&
        this.decoder.decodeQueueSize < CanvasVideoPlayer.MAX_DECODE_QUEUE &&
        this.decodedFrames.length + this.decoder.decodeQueueSize < this.maxDecodedFrames &&
        generation === this.decodeGeneration
      ) {
        let cursor = this.nextPacketCursor;
        let packet = cursor.packet;

        if (this.awaitingKeyFrame && !packet.isRap) {
          const keyframeCursor = await this.findNextKeyframeCursor(cursor, generation);
          // If generation changed while scanning, bail out.
          if (generation !== this.decodeGeneration) {
            break;
          }
          if (!keyframeCursor) {
            this.error$.next('Decoder requires a keyframe before playback can continue, but no keyframe was found.');
            this.nextPacketCursor = null;
            this.reachedEndOfStream = true;
            break;
          }
          cursor = keyframeCursor;
          this.nextPacketCursor = cursor;
          packet = cursor.packet;
        }

        this.nextPacketCursor = await this.source.getNextPacket(cursor);
        // If generation changed while getting the next packet, bail out before using the wrong decoder.
        if (generation !== this.decodeGeneration) {
          break;
        }

        if (!this.nextPacketCursor) {
          this.reachedEndOfStream = true;
        }

        try {
          this.decoder.decode(new EncodedVideoChunk({
            type: packet.isRap ? 'key' : 'delta',
            timestamp: packet.timestampMs * 1000,
            duration: packet.durationMs * 1000,
            data: packet.data,
          }));
          if (packet.isRap) {
            this.awaitingKeyFrame = false;
          }
        } catch (error) {
          if (this.isExpectedKeyFrameRecoveryError(error)) {
            this.awaitingKeyFrame = true;
            continue;
          }

          const message = error instanceof Error ? error.message : String(error);
          this.error$.next(`Failed to decode packet ${packet.index}: ${message}`);
          break;
        }
      }
    } finally {
      this.decodeAheadInFlight = false;

      if (this.decodeAheadQueued) {
        this.decodeAheadQueued = false;
        this.requestDecodeAhead();
      }
    }
  }

  private async decodeAudioAhead(): Promise<void> {
    if (
      this.audioDecodeAheadInFlight ||
      !this.audioDecoder ||
      !this.audioSource ||
      !this.nextAudioPacketCursor ||
      !this.audioContext ||
      this.stateChanged$.value !== 'playing'
    ) {
      return;
    }

    this.audioDecodeAheadInFlight = true;

    try {
      this.pruneScheduledAudioNodes();

      while (
        this.nextAudioPacketCursor &&
        this.audioDecoder &&
        this.audioDecoder.decodeQueueSize < CanvasVideoPlayer.MAX_AUDIO_DECODE_QUEUE &&
        this.scheduledAudioUntilSec - this.audioContext.currentTime < CanvasVideoPlayer.AUDIO_BUFFER_AHEAD_SEC
      ) {
        const cursor = this.nextAudioPacketCursor;
        this.nextAudioPacketCursor = await this.audioSource.getNextPacket(cursor);

        try {
          this.audioDecoder.decode(new EncodedAudioChunk({
            type: cursor.packet.isRap ? 'key' : 'delta',
            timestamp: cursor.packet.timestampMs * 1000,
            duration: cursor.packet.durationMs * 1000,
            data: cursor.packet.data,
          }));
        } catch (error) {
          console.error('[WebCodecs] Failed to decode audio packet:', error);
          this.disableAudioPlayback();
          break;
        }

        if (!this.nextAudioPacketCursor) {
          break;
        }
      }
    } finally {
      this.audioDecodeAheadInFlight = false;

      if (this.audioDecodeAheadQueued) {
        this.audioDecodeAheadQueued = false;
        this.requestAudioDecodeAhead();
      }
    }
  }

  private async getSeekCursor(timeMs: number): Promise<DemuxedPacketCursor | null> {
    if (!this.source) {
      return null;
    }

    const keyframeCursor = await this.source.getPacketForTime(timeMs, { keyframe: true });
    return keyframeCursor ?? this.source.getFirstPacket();
  }

  private async getAudioSeekCursor(timeMs: number): Promise<DemuxedPacketCursor | null> {
    if (!this.audioSource) {
      return null;
    }

    return (await this.audioSource.getPacketForTime(timeMs)) ?? this.audioSource.getFirstPacket();
  }

  private async findNextKeyframeCursor(cursor: DemuxedPacketCursor, generation: number): Promise<DemuxedPacketCursor | null> {
    if (cursor.packet.isRap) {
      return cursor;
    }

    if (!this.source) {
      return null;
    }

    let nextCursor = await this.source.getNextPacket(cursor);
    while (nextCursor && generation === this.decodeGeneration) {
      if (nextCursor.packet.isRap) {
        return nextCursor;
      }
      nextCursor = await this.source.getNextPacket(nextCursor);
    }

    return null;
  }

  private isExpectedKeyFrameRecoveryError(error: unknown): boolean {
    if (!(error instanceof DOMException)) {
      return false;
    }

    return error.message.includes('A key frame is required after configure() or flush().');
  }

  private drawForTime(currentTimeMs: number): void {
    while (this.decodedFrames.length > 1 && this.decodedFrames[1].timestampMs <= currentTimeMs) {
      const stale = this.decodedFrames.shift();
      stale?.frame.close();
    }

    const frame = this.decodedFrames[0];
    if (!frame) {
      if (this.lastDrawnTimestampMs < 0) {
        this.drawPlaceholder();
      }
      return;
    }

    const presentationToleranceMs = this.getPresentationToleranceMs();
    if (frame.timestampMs > currentTimeMs + presentationToleranceMs) {
      if (this.lastDrawnTimestampMs < 0) {
        // Unconditionally draw the VERY FIRST decoded frame even if its PTS is slightly ahead of 0.
        // This prevents black screen when paused at the beginning without audio sync yet.
      } else {
        return;
      }
    }

    if (frame.timestampMs === this.lastDrawnTimestampMs) {
      return;
    }

    this.ctx.drawImage(frame.frame, 0, 0, this.canvas.width, this.canvas.height);
    this.lastDrawnTimestampMs = frame.timestampMs;
  }

  private drawPlaceholder(): void {
    this.ctx.fillStyle = '#000';
    this.ctx.fillRect(0, 0, this.canvas.width || this.container.clientWidth || 1, this.canvas.height || this.container.clientHeight || 1);
  }

  private clearDecodedFrames(): void {
    for (const decoded of this.decodedFrames) {
      decoded.frame.close();
    }
    this.decodedFrames = [];
    this.lastDrawnTimestampMs = -1;
  }

  private pruneScheduledAudioNodes(): void {
    if (!this.audioContext) {
      this.scheduledAudioNodes = [];
      return;
    }

    const now = this.audioContext.currentTime;
    this.scheduledAudioNodes = this.scheduledAudioNodes.filter((entry) => entry.endTime > now - 0.05);
    this.scheduledAudioUntilSec = this.scheduledAudioNodes.reduce(
      (maxTime, entry) => Math.max(maxTime, entry.endTime),
      this.audioPlaybackStartContextSec,
    );
  }

  private clearScheduledAudioNodes(): void {
    for (const entry of this.scheduledAudioNodes) {
      try {
        entry.node.stop();
      } catch {
        // Ignore nodes that already ended.
      }
      entry.node.disconnect();
    }
    this.scheduledAudioNodes = [];
    this.scheduledAudioUntilSec = 0;
  }

  private resetAudioPlaybackState(): void {
    this.clearScheduledAudioNodes();
    this.audioDecodeAheadInFlight = false;
    this.audioDecodeAheadQueued = false;
    this.audioPlaybackStartContextSec = 0;
    this.nextAudioPacketCursor = null;

    if (this.audioDecoder) {
      this.audioDecoder.close();
      this.audioDecoder = undefined;
    }

    if (this.audioContext?.state === 'running') {
      void this.audioContext.suspend().catch(() => undefined);
    }
  }

  private disableAudioPlayback(): void {
    this.resetAudioPlaybackState();
    this.audioSource = undefined;
    this.audioDecoderConfig = undefined;
  }

  private cancelRenderLoop(): void {
    if (this.rafId !== null) {
      cancelAnimationFrame(this.rafId);
      this.rafId = null;
    }
  }

  private getCurrentTimeMs(): number {
    if (this.stateChanged$.value !== 'playing') {
      return this.playbackBaseMs;
    }

    if (this.audioContext?.state === 'running' && this.audioPlaybackStartContextSec > 0) {
      const elapsedMs = Math.max(0, (this.audioContext.currentTime - this.audioPlaybackStartContextSec) * 1000);
      return Math.min(this.getDurationMs(), this.playbackBaseMs + elapsedMs);
    }

    const elapsedMs = performance.now() - this.playbackStartPerfMs;
    return Math.min(this.getDurationMs(), this.playbackBaseMs + elapsedMs);
  }

  private getPlaybackTimeMs(): number {
    const targetTimeMs = this.getCurrentTimeMs();
    const bufferedUntilMs = this.getBufferedUntilMs();
    const frameIntervalMs = this.getFrameIntervalEstimateMs();

    if (
      bufferedUntilMs >= 0 &&
      targetTimeMs > bufferedUntilMs + frameIntervalMs &&
      !this.isPlaybackExhausted()
    ) {
      this.playbackBaseMs = bufferedUntilMs;
      this.playbackStartPerfMs = performance.now();
      return bufferedUntilMs;
    }

    return targetTimeMs;
  }

  private getBufferedUntilMs(): number {
    if (this.decodedFrames.length > 0) {
      return this.decodedFrames[this.decodedFrames.length - 1].timestampMs;
    }

    return this.lastDrawnTimestampMs;
  }

  private isPlaybackExhausted(): boolean {
    return Boolean(
      this.reachedEndOfStream &&
      (!this.decoder || this.decoder.decodeQueueSize === 0) &&
      this.decodedFrames.length <= 1,
    );
  }

  private getFrameIntervalEstimateMs(): number {
    if (this.decodedFrames.length > 1) {
      const delta = this.decodedFrames[1].timestampMs - this.decodedFrames[0].timestampMs;
      if (delta > 0) {
        return Math.min(250, Math.max(16, delta));
      }
    }

    if (this.decodedFrames.length > 0 && this.lastDrawnTimestampMs >= 0) {
      const delta = this.decodedFrames[0].timestampMs - this.lastDrawnTimestampMs;
      if (delta > 0) {
        return Math.min(250, Math.max(16, delta));
      }
    }

    if (this.metadata?.frameRate && Number.isFinite(this.metadata.frameRate) && this.metadata.frameRate > 0) {
      return Math.min(250, Math.max(16, 1000 / this.metadata.frameRate));
    }

    return 33.33;
  }

  private getPresentationToleranceMs(): number {
    return Math.min(12, this.getFrameIntervalEstimateMs() * 0.25);
  }

  private async shouldReloadSourceForReverseSeek(): Promise<boolean> {
    return !this.sourceSupportsRandomAccess && this.sourceUrl !== null;
  }

  private async reloadCurrentStreamSource(targetTimeMs: number, resumePlayback: boolean): Promise<void> {
    if (!this.sourceUrl) {
      return;
    }

    const sourceUrl = this.sourceUrl;
    await this.loadFromUrl(sourceUrl);

    if (targetTimeMs > 0) {
      await this.seek(targetTimeMs);
    }

    if (resumePlayback) {
      await this.play();
    }
  }

  private getDurationMs(): number {
    return this.metadata?.duration ?? 0;
  }

  private toMilliseconds(value: number, timescale: number): number {
    return Math.round((value * 1000) / Math.max(1, timescale));
  }

  private detectContainerFormat(buffer: ArrayBuffer, url?: string): ContainerFormat {
    if (url && this.detectTranscodedContainerHint(url) === 'mp4') {
      return 'mp4';
    }

    const bytes = new Uint8Array(buffer.slice(0, 16));
    const boxType = bytes.byteLength >= 8
      ? String.fromCharCode(bytes[4], bytes[5], bytes[6], bytes[7])
      : '';

    if (boxType === 'ftyp' || boxType === 'moov' || boxType === 'mdat') {
      return 'mp4';
    }

    if (bytes.byteLength >= 4 && bytes[0] === 0x1a && bytes[1] === 0x45 && bytes[2] === 0xdf && bytes[3] === 0xa3) {
      return 'mkv';
    }

    if (bytes.byteLength >= 1 && bytes[0] === 0x47) {
      return 'ts';
    }

    const normalizedUrl = (url ?? '').toLowerCase();
    if (/\.(mp4|m4v|mov)(?:$|[?#])/.test(normalizedUrl)) {
      return 'mp4';
    }
    if (/\.(mkv|webm)(?:$|[?#])/.test(normalizedUrl)) {
      return 'mkv';
    }
    if (/\.(ts|mts|m2ts)(?:$|[?#])/.test(normalizedUrl)) {
      return 'ts';
    }

    return 'unknown';
  }

  private detectContainerFormatFromUrl(url: string): ContainerFormat {
    const candidates: string[] = [];

    try {
      const resolvedUrl = new URL(url, window.location.href);
      const transcoded = this.detectTranscodedContainerHint(resolvedUrl.toString());
      if (transcoded !== 'unknown') {
        return transcoded;
      }

      candidates.push(resolvedUrl.pathname);

      const streamPath = resolvedUrl.searchParams.get('path');
      if (streamPath) {
        candidates.push(streamPath);
      }

      const archiveEntry = resolvedUrl.searchParams.get('entry');
      if (archiveEntry) {
        candidates.push(archiveEntry);
      }
    } catch {
      candidates.push(url);
    }

    for (const candidate of candidates) {
      const normalized = candidate.toLowerCase();
      if (/\.(mp4|m4v|mov)(?:$|[?#])/.test(normalized)) {
        return 'mp4';
      }
      if (/\.(mkv|webm)(?:$|[?#])/.test(normalized)) {
        return 'mkv';
      }
      if (/\.(ts|mts|m2ts)(?:$|[?#])/.test(normalized)) {
        return 'ts';
      }
    }

    return 'unknown';
  }

  private detectTranscodedContainerHint(url: string): ContainerFormat {
    try {
      const resolvedUrl = new URL(url, window.location.href);
      const transcodeFormat = (resolvedUrl.searchParams.get('transcode') ?? '').toLowerCase();
      if (transcodeFormat === 'mp4') {
        return 'mp4';
      }
    } catch {
      // Ignore malformed URLs and fall back to content sniffing.
    }

    return 'unknown';
  }

  private parseDurationHintFromUrl(url: string): number {
    try {
      const resolvedUrl = new URL(url, window.location.href);
      const rawDuration = Number(resolvedUrl.searchParams.get('duration') ?? '0');
      return Number.isFinite(rawDuration) && rawDuration > 0 ? rawDuration : 0;
    } catch {
      return 0;
    }
  }

  private ensureMp4(buffer: ArrayBuffer): void {
    const bytes = new Uint8Array(buffer.slice(0, 12));
    const boxType = String.fromCharCode(bytes[4], bytes[5], bytes[6], bytes[7]);
    if (boxType !== 'ftyp' && boxType !== 'moov' && boxType !== 'mdat') {
      throw new Error('Canvas playback currently supports MP4/MOV sources only.');
    }
  }

  private async ensureLoaded(): Promise<void> {
    if (!this.isLoaded && this.pendingLoad) {
      await this.pendingLoad;
    }

    if (!this.isLoaded && this.sourceUrl && !this.pendingLoad) {
      await this.loadFromUrl(this.sourceUrl);
    }

    if (!this.isLoaded || !this.metadata) {
      throw new Error('Video player is not initialized.');
    }
  }

  private suspendPlaybackForSeek(): void {
    this.playbackBaseMs = this.getCurrentTimeMs();
    this.cancelRenderLoop();
    this.resetAudioPlaybackState();
  }

  private async resumePlaybackAfterSeek(): Promise<void> {
    await this.prepareAudioPlayback(this.playbackBaseMs);
    this.playbackStartPerfMs = performance.now();
    this.stateChanged$.next('playing');
    this.renderLoop();
  }

  private isCurrentSeek(generation: number): boolean {
    return generation === this.seekGeneration;
  }

  private async prepareAudioPlayback(startTimeMs: number): Promise<void> {
    if (!this.audioSource || !this.audioDecoderConfig) {
      return;
    }

    const audioContext = this.getOrCreateAudioContext();
    if (!audioContext) {
      this.disableAudioPlayback();
      return;
    }

    try {
      if (audioContext.state !== 'running') {
        await audioContext.resume();
      }
      await this.rebuildAudioDecoder();
      this.nextAudioPacketCursor = await this.getAudioSeekCursor(startTimeMs);
      this.clearScheduledAudioNodes();
      this.audioPlaybackStartContextSec = audioContext.currentTime + CanvasVideoPlayer.AUDIO_SCHEDULE_LEAD_SEC;
      this.scheduledAudioUntilSec = this.audioPlaybackStartContextSec;
      await this.decodeAudioAhead();
    } catch (error) {
      console.error('[WebCodecs] Audio playback initialization failed:', error);
      this.disableAudioPlayback();
    }
  }

  private getOrCreateAudioContext(): AudioContext | undefined {
    if (this.audioContext) {
      return this.audioContext;
    }

    const AudioContextCtor = window.AudioContext ?? (window as Window & typeof globalThis & { webkitAudioContext?: typeof AudioContext }).webkitAudioContext;
    if (!AudioContextCtor) {
      return undefined;
    }

    this.audioContext = new AudioContextCtor({ latencyHint: 'interactive' });
    this.audioGainNode = this.audioContext.createGain();
    this.audioGainNode.gain.value = this.volume;
    this.audioGainNode.connect(this.audioContext.destination);
    return this.audioContext;
  }

  private createAudioBuffer(audioData: AudioData): AudioBuffer {
    const audioContext = this.getOrCreateAudioContext();
    if (!audioContext) {
      throw new Error('AudioContext is unavailable.');
    }

    const audioBuffer = audioContext.createBuffer(
      audioData.numberOfChannels,
      audioData.numberOfFrames,
      audioData.sampleRate,
    );

    for (let channelIndex = 0; channelIndex < audioData.numberOfChannels; channelIndex += 1) {
      const channelData = audioBuffer.getChannelData(channelIndex);
      audioData.copyTo(channelData, {
        planeIndex: channelIndex,
        format: 'f32-planar',
      } as AudioDataCopyToOptions);
    }

    return audioBuffer;
  }

  private requireContext(canvas: HTMLCanvasElement): CanvasRenderingContext2D {
    const context = canvas.getContext('2d');
    if (!context) {
      throw new Error('2D canvas rendering context is unavailable.');
    }
    return context;
  }

  private async fetchFile(url: string): Promise<ArrayBuffer> {
    const response = await fetch(url);
    if (!response.ok) {
      throw new Error(`HTTP ${response.status}: ${response.statusText}`);
    }
    return response.arrayBuffer();
  }
}
