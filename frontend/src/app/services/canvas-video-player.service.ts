import { Injectable } from '@angular/core';
import { BehaviorSubject, Subject } from 'rxjs';
import * as MP4Box from 'mp4box';

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

type PlayerState = 'idle' | 'paused' | 'playing' | 'ended';

type ExtendedVideoDecoderConfig = VideoDecoderConfig & {
  avc?: {
    format: 'annexb' | 'avc';
  };
};

interface TrackInfo {
  id: number;
  codec: string;
  timescale: number;
  duration: number;
  bitrate?: number;
  nb_samples: number;
  track_width?: number;
  track_height?: number;
  video?: {
    width?: number;
    height?: number;
  };
}

interface SampleInfo {
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

@Injectable({ providedIn: 'root' })
export class CanvasVideoPlayerService {
  async createPlayer(container: HTMLElement): Promise<CanvasVideoPlayer> {
    return new CanvasVideoPlayer(container);
  }
}

export class CanvasVideoPlayer {
  private readonly canvas: HTMLCanvasElement;
  private readonly ctx: CanvasRenderingContext2D;

  private decoder?: VideoDecoder;
  private decoderConfig?: ExtendedVideoDecoderConfig;
  private metadata?: VideoMetadata;
  private avcSampleConfig?: AvcSampleConfig;
  private samples: SampleInfo[] = [];
  private static readonly MAX_DECODED_FRAMES = 24;
  private static readonly MAX_DECODE_QUEUE = 16;
  private keyframeSampleIndexes: number[] = [];
  private decodedFrames: DecodedFrame[] = [];
  private sampleCursor = 0;
  private awaitingKeyFrame = true;
  private rafId: number | null = null;
  private playbackBaseMs = 0;
  private playbackStartPerfMs = 0;
  private lastDrawnTimestampMs = -1;
  private isLoaded = false;

  readonly stateChanged$ = new BehaviorSubject<PlayerState>('idle');
  readonly frameReady$ = new Subject<number>();
  readonly error$ = new Subject<string>();
  readonly metadataReady$ = new Subject<VideoMetadata>();
  readonly currentTime$ = new BehaviorSubject<number>(0);

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
    try {
      const buffer = await this.fetchFile(url);
      const metadata = await this.loadFromBuffer(buffer);
      return metadata;
    } catch (error) {
      const message = error instanceof Error ? error.message : 'Failed to load video';
      this.error$.next(message);
      throw error;
    }
  }

  async play(): Promise<void> {
    this.ensureLoaded();

    if (this.stateChanged$.value === 'playing') {
      return;
    }

    if (this.playbackBaseMs >= this.getDurationMs()) {
      await this.seek(0);
    }

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
    this.stateChanged$.next('paused');
    this.currentTime$.next(this.playbackBaseMs / 1000);
  }

  async seek(timeMs: number): Promise<void> {
    this.ensureLoaded();

    const clampedTimeMs = Math.max(0, Math.min(timeMs, this.getDurationMs()));
    const wasPlaying = this.stateChanged$.value === 'playing';
    if (wasPlaying) {
      this.pause();
    }

    this.playbackBaseMs = clampedTimeMs;
    this.currentTime$.next(clampedTimeMs / 1000);
    await this.rebuildDecoder();
    this.sampleCursor = this.findDecodeStartIndex(clampedTimeMs);
    this.clearDecodedFrames();
    this.decodeAhead();
    this.drawForTime(clampedTimeMs);

    if (wasPlaying) {
      await this.play();
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

  destroy(): void {
    this.cancelRenderLoop();
    this.clearDecodedFrames();
    this.decoder?.close();
    this.decoder = undefined;
    this.stateChanged$.complete();
    this.frameReady$.complete();
    this.error$.complete();
    this.metadataReady$.complete();
    this.currentTime$.complete();
    this.canvas.remove();
  }

  private async loadFromBuffer(buffer: ArrayBuffer): Promise<VideoMetadata> {
    this.ensureMp4(buffer);

    const mp4File = MP4Box.createFile();
    const info = await this.parseInfo(mp4File, buffer);
    const videoTrack = this.pickVideoTrack(info);
    const audioTrack = info.audioTracks?.[0] ?? null;
    const width = videoTrack.video?.width ?? videoTrack.track_width ?? this.canvas.clientWidth ?? 1280;
    const height = videoTrack.video?.height ?? videoTrack.track_height ?? this.canvas.clientHeight ?? 720;
    const durationMs = this.toMilliseconds(videoTrack.duration, videoTrack.timescale);
    const frameRate = this.inferFrameRate(videoTrack, durationMs);

    this.canvas.width = width;
    this.canvas.height = height;
    this.metadata = {
      width,
      height,
      duration: durationMs,
      format: 'mp4',
      videoCodec: videoTrack.codec,
      audioCodec: audioTrack?.codec,
      bitrate: videoTrack.bitrate,
      frameRate,
      hasAudio: Boolean(audioTrack),
      hasVideo: true,
    };
    this.metadataReady$.next(this.metadata);

    const sampleEntry = this.getSampleEntry(mp4File, videoTrack.id);
    this.avcSampleConfig = this.getAvcSampleConfig(sampleEntry);
    const decoderConfig = await this.buildDecoderConfig(videoTrack, sampleEntry);

    this.decoderConfig = decoderConfig;
    await this.rebuildDecoder();

    this.samples = await this.extractSamples(mp4File, videoTrack.id, videoTrack.nb_samples);
    this.keyframeSampleIndexes = this.samples.filter((sample) => sample.isRap).map((sample) => sample.index);
    this.isLoaded = true;
    this.sampleCursor = 0;
    this.playbackBaseMs = 0;
    this.currentTime$.next(0);
    this.stateChanged$.next('paused');

    await this.seek(0);
    return this.metadata;
  }

  private async parseInfo(mp4File: any, buffer: ArrayBuffer): Promise<any> {
    return new Promise((resolve, reject) => {
      mp4File.onError = (error: unknown) => {
        reject(new Error(`MP4 parsing failed: ${String(error)}`));
      };
      mp4File.onReady = (info: any) => resolve(info);

      const sourceBuffer = buffer as ArrayBuffer & { fileStart?: number };
      sourceBuffer.fileStart = 0;
      mp4File.appendBuffer(sourceBuffer, true);
      mp4File.flush();
    });
  }

  private pickVideoTrack(info: any): TrackInfo {
    const track = (info.videoTracks?.[0] ?? info.tracks?.find((candidate: any) => candidate.video)) as TrackInfo | undefined;
    if (!track) {
      throw new Error('No decodable video track was found in this MP4 file.');
    }
    return track;
  }

  private getSampleEntry(mp4File: any, trackId: number): any {
    const track = mp4File.getTrackById(trackId);
    const entry = track?.mdia?.minf?.stbl?.stsd?.entries?.[0];
    if (!entry) {
      throw new Error('The MP4 file is missing a video sample description.');
    }
    return entry;
  }

  private async buildDecoderConfig(track: TrackInfo, sampleEntry: any): Promise<ExtendedVideoDecoderConfig> {
    if (!('VideoDecoder' in window)) {
      throw new Error('This browser runtime does not expose WebCodecs VideoDecoder.');
    }

    const codedWidth = track.video?.width ?? track.track_width ?? 1280;
    const codedHeight = track.video?.height ?? track.track_height ?? 720;
    const description = this.serializeCodecDescription(sampleEntry);
    const candidates = this.buildDecoderConfigCandidates(track.codec, codedWidth, codedHeight, description);

    for (const candidate of candidates) {
      if (await this.canUseDecoderConfig(candidate)) {
        return candidate;
      }
    }

    throw new Error(`Unsupported MP4 codec for WebCodecs: ${track.codec}`);
  }

  private buildDecoderConfigCandidates(
    codec: string,
    codedWidth: number,
    codedHeight: number,
    description?: Uint8Array,
  ): ExtendedVideoDecoderConfig[] {
    const codecCandidates = this.expandCodecCandidates(codec);
    const configs: ExtendedVideoDecoderConfig[] = [];
    const isAvcCodec = codec.startsWith('avc1') || codec.startsWith('avc3');

    for (const codecCandidate of codecCandidates) {
      if (isAvcCodec) {
        configs.push({
          codec: codecCandidate,
          codedWidth,
          codedHeight,
          avc: { format: 'annexb' },
          optimizeForLatency: true,
        });
      }

      if (description) {
        configs.push({
          codec: codecCandidate,
          codedWidth,
          codedHeight,
          description,
          ...(isAvcCodec ? { avc: { format: 'avc' as const } } : {}),
          optimizeForLatency: true,
        });
      }

      configs.push({
        codec: codecCandidate,
        codedWidth,
        codedHeight,
        optimizeForLatency: true,
      });
    }

    return configs;
  }

  private expandCodecCandidates(codec: string): string[] {
    const candidates = new Set<string>([codec]);
    const baseCodec = codec.split('.')[0];
    candidates.add(baseCodec);

    if (codec.startsWith('avc1.')) {
      candidates.add('avc1');
      candidates.add(codec.toLowerCase());
      candidates.add(codec.toUpperCase());
    }

    if (codec.startsWith('hvc1.') || codec.startsWith('hev1.')) {
      candidates.add('hvc1');
      candidates.add('hev1');
    }

    return Array.from(candidates);
  }

  private async canUseDecoderConfig(config: VideoDecoderConfig): Promise<boolean> {
    try {
      if (typeof VideoDecoder.isConfigSupported === 'function') {
        const support = await VideoDecoder.isConfigSupported(config);
        if (support.supported) {
          return true;
        }
      }
    } catch {
      // Some embedded runtimes throw or report false negatives; fall through to a configure probe.
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

  private serializeCodecDescription(sampleEntry: any): Uint8Array | undefined {
    if (sampleEntry.avcC) {
      return this.serializeAvcC(sampleEntry.avcC);
    }

    const box = sampleEntry.hvcC ?? sampleEntry.vpcC ?? sampleEntry.av1C;
    if (!box) {
      return undefined;
    }

    const stream = new MP4Box.DataStream(0);
    box.write(stream);
    return new Uint8Array(stream.buffer.slice(8, stream.position));
  }

  private getAvcSampleConfig(sampleEntry: any): AvcSampleConfig | undefined {
    const avcC = sampleEntry?.avcC;
    if (!avcC) {
      return undefined;
    }

    return {
      lengthSize: (avcC.lengthSizeMinusOne ?? 3) + 1,
      sps: (Array.isArray(avcC.SPS) ? avcC.SPS : [])
        .map((entry: any) => this.normalizeNaluBytes(entry))
        .filter((entry: Uint8Array) => entry.byteLength > 0),
      pps: (Array.isArray(avcC.PPS) ? avcC.PPS : [])
        .map((entry: any) => this.normalizeNaluBytes(entry))
        .filter((entry: Uint8Array) => entry.byteLength > 0),
    };
  }

  private serializeAvcC(avcC: any): Uint8Array {
    const spsList = Array.isArray(avcC.SPS) ? avcC.SPS : [];
    const ppsList = Array.isArray(avcC.PPS) ? avcC.PPS : [];
    const ext = avcC.ext instanceof Uint8Array ? avcC.ext : undefined;

    let size = 6 + 1;
    for (const sps of spsList) {
      const nalu = this.normalizeNaluBytes(sps);
      size += 2 + nalu.byteLength;
    }
    for (const pps of ppsList) {
      const nalu = this.normalizeNaluBytes(pps);
      size += 2 + nalu.byteLength;
    }
    if (ext) {
      size += ext.byteLength;
    }

    const bytes = new Uint8Array(size);
    let offset = 0;

    bytes[offset++] = avcC.configurationVersion ?? 1;
    bytes[offset++] = avcC.AVCProfileIndication ?? 0;
    bytes[offset++] = avcC.profile_compatibility ?? 0;
    bytes[offset++] = avcC.AVCLevelIndication ?? 0;
    bytes[offset++] = 0xfc | ((avcC.lengthSizeMinusOne ?? 3) & 0x03);
    bytes[offset++] = 0xe0 | (spsList.length & 0x1f);

    for (const sps of spsList) {
      const nalu = this.normalizeNaluBytes(sps);
      offset = this.writeLengthPrefixedNalu(bytes, offset, nalu);
    }

    bytes[offset++] = ppsList.length & 0xff;
    for (const pps of ppsList) {
      const nalu = this.normalizeNaluBytes(pps);
      offset = this.writeLengthPrefixedNalu(bytes, offset, nalu);
    }

    if (ext) {
      bytes.set(ext, offset);
    }

    return bytes;
  }

  private writeLengthPrefixedNalu(target: Uint8Array, offset: number, nalu: Uint8Array): number {
    const length = nalu.byteLength;
    target[offset++] = (length >> 8) & 0xff;
    target[offset++] = length & 0xff;
    target.set(nalu, offset);
    return offset + length;
  }

  private normalizeNaluBytes(entry: any): Uint8Array {
    if (entry?.nalu instanceof Uint8Array) {
      return entry.nalu;
    }
    if (entry instanceof Uint8Array) {
      return entry;
    }
    if (entry?.data instanceof Uint8Array) {
      return entry.data;
    }
    return new Uint8Array(0);
  }

  private convertAvcSampleToAnnexB(sample: Uint8Array, isKeyFrame: boolean): Uint8Array {
    if (!this.avcSampleConfig) {
      return sample;
    }

    const nalUnits: Uint8Array[] = [];
    if (isKeyFrame) {
      nalUnits.push(...this.avcSampleConfig.sps, ...this.avcSampleConfig.pps);
    }

    let offset = 0;
    while (offset + this.avcSampleConfig.lengthSize <= sample.byteLength) {
      let nalLength = 0;
      for (let index = 0; index < this.avcSampleConfig.lengthSize; index++) {
        nalLength = (nalLength << 8) | sample[offset + index];
      }
      offset += this.avcSampleConfig.lengthSize;

      if (nalLength <= 0 || offset + nalLength > sample.byteLength) {
        break;
      }

      nalUnits.push(sample.subarray(offset, offset + nalLength));
      offset += nalLength;
    }

    if (!nalUnits.length) {
      return sample;
    }

    const annexB = new Uint8Array(nalUnits.reduce((total, unit) => total + 4 + unit.byteLength, 0));
    let writeOffset = 0;
    for (const unit of nalUnits) {
      annexB[writeOffset++] = 0x00;
      annexB[writeOffset++] = 0x00;
      annexB[writeOffset++] = 0x00;
      annexB[writeOffset++] = 0x01;
      annexB.set(unit, writeOffset);
      writeOffset += unit.byteLength;
    }

    return annexB;
  }

  private async rebuildDecoder(): Promise<void> {
    if (!this.decoderConfig) {
      return;
    }

    this.decoder?.close();
    this.decoder = new VideoDecoder({
      output: (frame) => this.onDecodedFrame(frame),
      error: (error) => {
        const message = error instanceof Error ? error.message : String(error);
        this.error$.next(`Video decode error: ${message}`);
      },
    });
    this.decoder.configure(this.decoderConfig);
    this.awaitingKeyFrame = true;
  }

  private async extractSamples(mp4File: any, trackId: number, expectedCount: number): Promise<SampleInfo[]> {
    return new Promise((resolve, reject) => {
      const extracted: SampleInfo[] = [];

      mp4File.onError = (error: unknown) => {
        reject(new Error(`MP4 sample extraction failed: ${String(error)}`));
      };

      mp4File.onSamples = (_id: number, _user: unknown, samples: any[]) => {
        for (const sample of samples) {
          const sampleBytes = sample.data instanceof Uint8Array ? sample.data : new Uint8Array(sample.data);
          extracted.push({
            index: extracted.length,
            timestampMs: this.toMilliseconds(sample.cts, sample.timescale),
            durationMs: Math.max(1, this.toMilliseconds(sample.duration, sample.timescale)),
            isRap: Boolean(sample.is_rap || sample.is_sync),
            data: this.avcSampleConfig
              ? this.convertAvcSampleToAnnexB(sampleBytes, Boolean(sample.is_rap || sample.is_sync))
              : sampleBytes,
          });
        }

        if (extracted.length >= expectedCount) {
          mp4File.stop();
          mp4File.unsetExtractionOptions(trackId);
          resolve(extracted);
        }
      };

      mp4File.setExtractionOptions(trackId, null, {
        nbSamples: Math.min(250, Math.max(1, expectedCount)),
      });
      mp4File.start();
      mp4File.flush();
    });
  }

  private onDecodedFrame(frame: VideoFrame): void {
    const timestampMs = Math.round(frame.timestamp / 1000);
    this.decodedFrames.push({ timestampMs, frame });
    this.decodedFrames.sort((left, right) => left.timestampMs - right.timestampMs);

    while (this.decodedFrames.length > CanvasVideoPlayer.MAX_DECODED_FRAMES) {
      const stale = this.decodedFrames.shift();
      stale?.frame.close();
    }

    this.frameReady$.next(timestampMs);
  }

  private renderLoop(): void {
    if (this.stateChanged$.value !== 'playing') {
      return;
    }

    const currentTimeMs = this.getCurrentTimeMs();
    this.currentTime$.next(currentTimeMs / 1000);
    this.decodeAhead();
    this.drawForTime(currentTimeMs);

    if (currentTimeMs >= this.getDurationMs()) {
      this.playbackBaseMs = this.getDurationMs();
      this.cancelRenderLoop();
      this.stateChanged$.next('ended');
      return;
    }

    this.rafId = requestAnimationFrame(() => this.renderLoop());
  }

  private decodeAhead(): void {
    if (!this.decoder || !this.samples.length) {
      return;
    }

    while (
      this.sampleCursor < this.samples.length &&
      this.decoder.decodeQueueSize < CanvasVideoPlayer.MAX_DECODE_QUEUE &&
      this.decodedFrames.length + this.decoder.decodeQueueSize < CanvasVideoPlayer.MAX_DECODED_FRAMES
    ) {
      const sample = this.samples[this.sampleCursor];

      if (this.awaitingKeyFrame && !sample.isRap) {
        const keyframeIndex = this.findDecodeStartIndex(sample.timestampMs);
        if (keyframeIndex === this.sampleCursor) {
          this.error$.next(`Decoder requires a keyframe before sample ${sample.index}, but no prior keyframe was found.`);
          break;
        }
        this.sampleCursor = keyframeIndex;
        continue;
      }

      this.sampleCursor += 1;

      try {
        this.decoder.decode(new EncodedVideoChunk({
          type: sample.isRap ? 'key' : 'delta',
          timestamp: sample.timestampMs * 1000,
          duration: sample.durationMs * 1000,
          data: sample.data,
        }));
        if (sample.isRap) {
          this.awaitingKeyFrame = false;
        }
      } catch (error) {
        const message = error instanceof Error ? error.message : String(error);
        this.error$.next(`Failed to decode MP4 sample ${sample.index}: ${message}`);
        break;
      }
    }
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

    if (frame.timestampMs > currentTimeMs + 40) {
      return;
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

  private cancelRenderLoop(): void {
    if (this.rafId !== null) {
      cancelAnimationFrame(this.rafId);
      this.rafId = null;
    }
  }

  private findDecodeStartIndex(timeMs: number): number {
    if (!this.samples.length) {
      return 0;
    }

    let low = 0;
    let high = this.samples.length - 1;
    while (low < high) {
      const mid = Math.floor((low + high) / 2);
      if (this.samples[mid].timestampMs < timeMs) {
        low = mid + 1;
      } else {
        high = mid;
      }
    }

    const sampleIndex = this.samples[low].timestampMs > timeMs && low > 0 ? low - 1 : low;
    let rapIndex = 0;
    for (const keyframeIndex of this.keyframeSampleIndexes) {
      if (keyframeIndex > sampleIndex) {
        break;
      }
      rapIndex = keyframeIndex;
    }
    return rapIndex;
  }

  private getCurrentTimeMs(): number {
    if (this.stateChanged$.value !== 'playing') {
      return this.playbackBaseMs;
    }

    const elapsedMs = performance.now() - this.playbackStartPerfMs;
    return Math.min(this.getDurationMs(), this.playbackBaseMs + elapsedMs);
  }

  private getDurationMs(): number {
    return this.metadata?.duration ?? 0;
  }

  private inferFrameRate(track: TrackInfo, durationMs: number): number {
    if (!track.nb_samples || durationMs <= 0) {
      return 0;
    }

    return Math.max(1, Math.round((track.nb_samples * 1000) / durationMs));
  }

  private toMilliseconds(value: number, timescale: number): number {
    return Math.round((value * 1000) / Math.max(1, timescale));
  }

  private ensureMp4(buffer: ArrayBuffer): void {
    const bytes = new Uint8Array(buffer.slice(0, 12));
    const boxType = String.fromCharCode(bytes[4], bytes[5], bytes[6], bytes[7]);
    if (boxType !== 'ftyp' && boxType !== 'moov' && boxType !== 'mdat') {
      throw new Error('Canvas playback currently supports MP4/MOV sources only.');
    }
  }

  private ensureLoaded(): void {
    if (!this.isLoaded || !this.metadata) {
      throw new Error('Video player is not initialized.');
    }
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