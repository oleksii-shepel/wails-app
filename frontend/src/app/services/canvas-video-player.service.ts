import { Injectable } from '@angular/core';
import { BehaviorSubject, Subject } from 'rxjs';
import { BufferSource, EncodedPacketSink, Input, MATROSKA, MP4, MPEG_TS, QTFF, UrlSource, WEBM } from 'mediabunny';
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

type ContainerFormat = 'mp4' | 'mkv' | 'ts' | 'unknown';

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

interface DemuxedPacketCursor {
  token: unknown;
  packet: SampleInfo;
}

interface DemuxedVideoSource {
  metadata: VideoMetadata;
  decoderConfig: ExtendedVideoDecoderConfig;
  avcSampleConfig?: AvcSampleConfig;
  getFirstPacket(): Promise<DemuxedPacketCursor | null>;
  getPacketForTime(timeMs: number, options?: { keyframe?: boolean }): Promise<DemuxedPacketCursor | null>;
  getNextPacket(cursor: DemuxedPacketCursor): Promise<DemuxedPacketCursor | null>;
  destroy?(): void;
}

class Mp4DemuxedSource {
  static async load(buffer: ArrayBuffer): Promise<DemuxedVideoSource> {
    this.ensureMp4(buffer);

    const mp4File = MP4Box.createFile();
    const info = await this.parseInfo(mp4File, buffer);
    const videoTrack = this.pickVideoTrack(info);
    const audioTrack = info.audioTracks?.[0] ?? null;
    const width = videoTrack.video?.width ?? videoTrack.track_width ?? 1280;
    const height = videoTrack.video?.height ?? videoTrack.track_height ?? 720;
    const durationMs = this.resolveTrackDurationMs(videoTrack);
    const frameRate = this.inferFrameRate(videoTrack, durationMs);

    const metadata: VideoMetadata = {
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

    const sampleEntry = this.getSampleEntry(mp4File, videoTrack.id);
    const avcSampleConfig = this.getAvcSampleConfig(sampleEntry);
    const decoderConfig = await this.buildDecoderConfig(videoTrack, sampleEntry);
    const samples = await this.extractSamples(mp4File, videoTrack.id, videoTrack.nb_samples, avcSampleConfig, decoderConfig);
    const keyframeSampleIndexes = samples.filter((sample) => sample.isRap).map((sample) => sample.index);

    return ArrayBackedDemuxedSource.create({
      metadata,
      decoderConfig,
      avcSampleConfig,
      samples,
      destroy: () => {
        mp4File.stop?.();
        mp4File.unsetExtractionOptions?.(videoTrack.id);
      },
    });
  }

  static async loadFromUrl(url: string): Promise<DemuxedVideoSource> {
    return MediaBunnyDemuxedSource.loadFromUrl(url, [MP4, QTFF], (inputFormat) => inputFormat === QTFF ? 'mov' : 'mp4');
  }

  private static async parseInfo(mp4File: any, buffer: ArrayBuffer): Promise<any> {
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

  private static pickVideoTrack(info: any): TrackInfo {
    const track = (info.videoTracks?.[0] ?? info.tracks?.find((candidate: any) => candidate.video)) as TrackInfo | undefined;
    if (!track) {
      throw new Error('No decodable video track was found in this MP4 file.');
    }
    return track;
  }

  private static getSampleEntry(mp4File: any, trackId: number): any {
    const track = mp4File.getTrackById(trackId);
    const entry = track?.mdia?.minf?.stbl?.stsd?.entries?.[0];
    if (!entry) {
      throw new Error('The MP4 file is missing a video sample description.');
    }
    return entry;
  }

  private static async buildDecoderConfig(track: TrackInfo, sampleEntry: any): Promise<ExtendedVideoDecoderConfig> {
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

  private static buildDecoderConfigCandidates(
    codec: string,
    codedWidth: number,
    codedHeight: number,
    description?: Uint8Array,
  ): ExtendedVideoDecoderConfig[] {
    const codecCandidates = this.expandCodecCandidates(codec);
    const configs: ExtendedVideoDecoderConfig[] = [];
    const isAvcCodec = codec.startsWith('avc1') || codec.startsWith('avc3');

    for (const codecCandidate of codecCandidates) {
      if (description) {
        configs.push({
          codec: codecCandidate,
          codedWidth,
          codedHeight,
          description,
          ...(isAvcCodec ? { avc: { format: 'avc' as const } } : {}),
          optimizeForLatency: true,
        });

        if (isAvcCodec) {
          configs.push({
            codec: codecCandidate,
            codedWidth,
            codedHeight,
            description,
            avc: { format: 'annexb' as const },
            optimizeForLatency: true,
          });
        }
      }

      if (isAvcCodec) {
        configs.push({
          codec: codecCandidate,
          codedWidth,
          codedHeight,
          avc: { format: 'annexb' as const },
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

  private static expandCodecCandidates(codec: string): string[] {
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

  private static serializeCodecDescription(sampleEntry: any): Uint8Array | undefined {
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

  private static getAvcSampleConfig(sampleEntry: any): AvcSampleConfig | undefined {
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

  private static serializeAvcC(avcC: any): Uint8Array {
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

  private static writeLengthPrefixedNalu(target: Uint8Array, offset: number, nalu: Uint8Array): number {
    const length = nalu.byteLength;
    target[offset++] = (length >> 8) & 0xff;
    target[offset++] = length & 0xff;
    target.set(nalu, offset);
    return offset + length;
  }

  private static normalizeNaluBytes(entry: any): Uint8Array {
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

  private static convertAvcSampleToAnnexB(sample: Uint8Array, isKeyFrame: boolean, avcSampleConfig?: AvcSampleConfig): Uint8Array {
    if (!avcSampleConfig) {
      return sample;
    }

    const nalUnits: Uint8Array[] = [];
    if (isKeyFrame) {
      nalUnits.push(...avcSampleConfig.sps, ...avcSampleConfig.pps);
    }

    let offset = 0;
    while (offset + avcSampleConfig.lengthSize <= sample.byteLength) {
      let nalLength = 0;
      for (let index = 0; index < avcSampleConfig.lengthSize; index++) {
        nalLength = (nalLength << 8) | sample[offset + index];
      }
      offset += avcSampleConfig.lengthSize;

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

  private static async extractSamples(
    mp4File: any,
    trackId: number,
    expectedCount: number,
    avcSampleConfig: AvcSampleConfig | undefined,
    decoderConfig: ExtendedVideoDecoderConfig,
  ): Promise<SampleInfo[]> {
    return new Promise((resolve, reject) => {
      const batchSize = 1000;
      const extracted: SampleInfo[] = [];
      let resolved = false;

      const finish = () => {
        if (resolved) {
          return;
        }
        resolved = true;
        mp4File.stop();
        mp4File.unsetExtractionOptions(trackId);
        resolve(extracted);
      };

      mp4File.onError = (error: unknown) => {
        if (resolved) {
          return;
        }
        reject(new Error(`MP4 sample extraction failed: ${String(error)}`));
      };

      mp4File.onSamples = (_id: number, _user: unknown, samples: any[]) => {
        const expectsAnnexB = decoderConfig.avc?.format === 'annexb';
        for (const sample of samples) {
          const sampleBytes = sample.data instanceof Uint8Array ? sample.data : new Uint8Array(sample.data);
          extracted.push({
            index: extracted.length,
            timestampMs: this.toMilliseconds(sample.cts, sample.timescale),
            durationMs: Math.max(1, this.toMilliseconds(sample.duration, sample.timescale)),
            isRap: Boolean(sample.is_rap || sample.is_sync),
            data: (avcSampleConfig && expectsAnnexB)
              ? this.convertAvcSampleToAnnexB(sampleBytes, Boolean(sample.is_rap || sample.is_sync), avcSampleConfig)
              : sampleBytes,
          });
        }

        if (extracted.length >= expectedCount || samples.length < batchSize) {
          finish();
        }
      };

      mp4File.setExtractionOptions(trackId, null, {
        nbSamples: batchSize,
      });
      mp4File.start();
      mp4File.flush();
    });
  }

  private static resolveTrackDurationMs(track: TrackInfo): number {
    const candidates = [
      this.toMilliseconds(track.duration, track.timescale),
      this.toMilliseconds(track.samples_duration ?? 0, track.timescale),
      this.toMilliseconds(track.movie_duration ?? 0, track.movie_timescale ?? 0),
      this.toMilliseconds((track.sample_duration ?? 0) * (track.nb_samples ?? 0), track.timescale),
    ];

    return candidates.find((value) => value > 0) ?? 0;
  }

  private static inferFrameRate(track: TrackInfo, durationMs: number): number {
    if (!track.nb_samples || durationMs <= 0) {
      return 0;
    }

    return Math.max(1, Math.round((track.nb_samples * 1000) / durationMs));
  }

  private static toMilliseconds(value: number, timescale: number): number {
    return Math.round((value * 1000) / Math.max(1, timescale));
  }

  private static ensureMp4(buffer: ArrayBuffer): void {
    const bytes = new Uint8Array(buffer.slice(0, 12));
    const boxType = String.fromCharCode(bytes[4], bytes[5], bytes[6], bytes[7]);
    if (boxType !== 'ftyp' && boxType !== 'moov' && boxType !== 'mdat') {
      throw new Error('Canvas playback currently supports MP4/MOV sources only.');
    }
  }
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
  ): Promise<DemuxedVideoSource> {
    return this.loadInput(new UrlSource(url, {
      maxCacheSize: 64 * 1024 * 1024,
      parallelism: 2,
    }), formats, resolveFormatName);
  }

  static async loadFromBuffer(
    buffer: ArrayBuffer,
    formats: unknown[],
    resolveFormatName: (inputFormat: unknown) => string,
  ): Promise<DemuxedVideoSource> {
    return this.loadInput(new BufferSource(buffer), formats, resolveFormatName);
  }

  private static async loadInput(
    source: BufferSource | UrlSource,
    formats: unknown[],
    resolveFormatName: (inputFormat: unknown) => string,
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
        input.computeDuration(),
      ]);

      if (!videoTrack) {
        throw new Error(`No decodable video track was found in this ${normalizeFormatName(inputFormat)} file.`);
      }

      const packetSink = new EncodedPacketSink(videoTrack);

      const [decoderConfig, videoCodec, audioCodec, stats, trackDurationSec, firstPacket] = await Promise.all([
        videoTrack.getDecoderConfig(),
        videoTrack.getCodecParameterString(),
        audioTrack?.getCodecParameterString() ?? Promise.resolve(null),
        videoTrack.computePacketStats(120),
        videoTrack.computeDuration(),
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
        this.toMilliseconds(inputDurationSec) - timestampOffsetMs,
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

      return {
        metadata,
        decoderConfig: normalizedDecoderConfig,
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

  private static toMilliseconds(valueSec: number): number {
    return Math.round(valueSec * 1000);
  }
}

class MkvDemuxedSource {
  static async load(buffer: ArrayBuffer): Promise<DemuxedVideoSource> {
    this.ensureMatroska(buffer);

    return MediaBunnyDemuxedSource.loadFromBuffer(buffer, [MATROSKA, WEBM], (inputFormat) => inputFormat === WEBM ? 'webm' : 'mkv');
  }

  static async loadFromUrl(url: string): Promise<DemuxedVideoSource> {
    return MediaBunnyDemuxedSource.loadFromUrl(url, [MATROSKA, WEBM], (inputFormat) => inputFormat === WEBM ? 'webm' : 'mkv');
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

  static async loadFromUrl(url: string): Promise<DemuxedVideoSource> {
    return MediaBunnyDemuxedSource.loadFromUrl(url, [MPEG_TS], () => 'ts');
  }

  private static ensureTsFormat(buffer: ArrayBuffer): void {
    const bytes = new Uint8Array(buffer.slice(0, 1));
    // TS/M2TS streams typically start with sync byte 0x47
    if (bytes.byteLength < 1 || bytes[0] !== 0x47) {
      throw new Error('Canvas playback expected an MPEG-TS source, but the file does not start with a TS sync byte.');
    }
  }
}

@Injectable({ providedIn: 'root' })
export class CanvasVideoPlayerService {
  private static readonly DEMUXER_BACKED_EXTENSIONS = new Set(['mp4', 'm4v', 'mov', 'mkv', 'webm', 'ts', 'mts', 'm2ts']);

  async createPlayer(container: HTMLElement): Promise<CanvasVideoPlayer> {
    return new CanvasVideoPlayer(container);
  }

  supportsCanvasVideoFormat(ext: string): boolean {
    return CanvasVideoPlayerService.DEMUXER_BACKED_EXTENSIONS.has(ext.toLowerCase());
  }

  getUnsupportedVideoMessage(ext: string): string {
    const normalizedExt = ext.toLowerCase();
    const upperExt = normalizedExt.toUpperCase();

    return `${upperExt} is detected as a video file, but the current canvas player only has demux backends for MP4/MOV/MKV/WebM/TS right now. ${upperExt} support still needs a container-specific demuxer before WebCodecs can decode it.`;
  }
}

export class CanvasVideoPlayer {
  private readonly canvas: HTMLCanvasElement;
  private readonly ctx: CanvasRenderingContext2D;

  private source?: DemuxedVideoSource;
  private decoder?: VideoDecoder;
  private decoderConfig?: ExtendedVideoDecoderConfig;
  private metadata?: VideoMetadata;
  
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
  private maxDecodedFrames = 60; // Updated when video loads
  private decodedFrames: DecodedFrame[] = [];
  private nextPacketCursor: DemuxedPacketCursor | null = null;
  private reachedEndOfStream = false;
  private decodeAheadInFlight = false;
  private decodeAheadQueued = false;
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
      const hintedFormat = this.detectContainerFormatFromUrl(url);
      if (hintedFormat === 'mp4' || hintedFormat === 'mkv' || hintedFormat === 'ts') {
        return await this.loadFromStreamUrl(url, hintedFormat);
      }

      const buffer = await this.fetchFile(url);
      const containerFormat = this.detectContainerFormat(buffer, url);
      const metadata = await this.loadFromBuffer(buffer, containerFormat);
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
    this.clearDecodedFrames();
    this.nextPacketCursor = await this.getSeekCursor(clampedTimeMs);
    this.reachedEndOfStream = this.nextPacketCursor === null;
    await this.decodeAhead();
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
    this.source?.destroy?.();
    this.source = undefined;
    this.decoder?.close();
    this.decoder = undefined;
    this.stateChanged$.complete();
    this.frameReady$.complete();
    this.error$.complete();
    this.metadataReady$.complete();
    this.currentTime$.complete();
    this.canvas.remove();
  }

  private async loadFromBuffer(buffer: ArrayBuffer, containerFormat: ContainerFormat): Promise<VideoMetadata> {
    const source = await this.demuxBuffer(buffer, containerFormat);
    return this.initializeDemuxedSource(source);
  }

  private async loadFromStreamUrl(url: string, containerFormat: ContainerFormat): Promise<VideoMetadata> {
    const source = await this.demuxUrl(url, containerFormat);
    return this.initializeDemuxedSource(source);
  }

  private async demuxBuffer(buffer: ArrayBuffer, containerFormat: ContainerFormat): Promise<DemuxedVideoSource> {
    switch (containerFormat) {
      case 'mp4':
        return Mp4DemuxedSource.load(buffer);
      case 'mkv':
        return MkvDemuxedSource.load(buffer);
      case 'ts':
        return TsDemuxedSource.load(buffer);
      default:
        throw new Error('Canvas playback currently supports MP4/MOV/MKV/WebM/TS sources only. Other containers need a demuxer implementation before WebCodecs can decode them.');
    }
  }

  private async demuxUrl(url: string, containerFormat: ContainerFormat): Promise<DemuxedVideoSource> {
    switch (containerFormat) {
      case 'mp4':
        return Mp4DemuxedSource.loadFromUrl(url);
      case 'mkv':
        return MkvDemuxedSource.loadFromUrl(url);
      case 'ts':
        return TsDemuxedSource.loadFromUrl(url);
      default:
        throw new Error('Streaming URL demux is currently implemented for MP4/MOV/MKV/WebM/TS only. Other containers still use the buffered load path.');
    }
  }

  private async initializeDemuxedSource(source: DemuxedVideoSource): Promise<VideoMetadata> {
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

    await this.rebuildDecoder();
    this.isLoaded = true;
    this.nextPacketCursor = null;
    this.reachedEndOfStream = false;
    this.playbackBaseMs = 0;
    this.currentTime$.next(0);
    this.stateChanged$.next('paused');

    await this.seek(0);
    return this.metadata;
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
        console.error('[WebCodecs] VideoDecoder error:', error);
        const message = error instanceof Error ? error.message : String(error);
        this.error$.next(`Video decode error: ${message}`);
      },
    });
    this.decoder.configure(this.decoderConfig);
    this.awaitingKeyFrame = true;
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
    // or the canvas will remain empty black.
    if (this.stateChanged$.value !== 'playing') {
      this.drawForTime(this.getCurrentTimeMs());
      this.requestDecodeAhead();
    }
  }

  private renderLoop(): void {
    if (this.stateChanged$.value !== 'playing') {
      return;
    }

    const currentTimeMs = this.getPlaybackTimeMs();
    this.currentTime$.next(currentTimeMs / 1000);
    this.requestDecodeAhead();
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

  private async decodeAhead(): Promise<void> {
    if (this.decodeAheadInFlight || !this.decoder || !this.source || !this.nextPacketCursor) {
      return;
    }

    this.decodeAheadInFlight = true;

    try {
      while (
        this.nextPacketCursor &&
        this.decoder &&
        this.decoder.decodeQueueSize < CanvasVideoPlayer.MAX_DECODE_QUEUE &&
        this.decodedFrames.length + this.decoder.decodeQueueSize < this.maxDecodedFrames
      ) {
        let cursor = this.nextPacketCursor;
        let packet = cursor.packet;

        if (this.awaitingKeyFrame && !packet.isRap) {
          const keyframeCursor = await this.findNextKeyframeCursor(cursor);
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

  private async getSeekCursor(timeMs: number): Promise<DemuxedPacketCursor | null> {
    if (!this.source) {
      return null;
    }

    const keyframeCursor = await this.source.getPacketForTime(timeMs, { keyframe: true });
    return keyframeCursor ?? this.source.getFirstPacket();
  }

  private async findNextKeyframeCursor(cursor: DemuxedPacketCursor): Promise<DemuxedPacketCursor | null> {
    if (cursor.packet.isRap) {
      return cursor;
    }

    if (!this.source) {
      return null;
    }

    let nextCursor = await this.source.getNextPacket(cursor);
    while (nextCursor) {
      if (nextCursor.packet.isRap) {
        return nextCursor;
      }
      nextCursor = await this.source.getNextPacket(nextCursor);
    }

    return null;
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

    const frameIntervalMs = this.metadata ? 1000 / (this.metadata.frameRate || 30) : 16.67;
    if (frame.timestampMs > currentTimeMs + frameIntervalMs * 1.5) {
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

    const elapsedMs = performance.now() - this.playbackStartPerfMs;
    return Math.min(this.getDurationMs(), this.playbackBaseMs + elapsedMs);
  }

  private getPlaybackTimeMs(): number {
    const targetTimeMs = this.getCurrentTimeMs();
    const bufferedUntilMs = this.getBufferedUntilMs();
    const frameIntervalMs = this.metadata ? 1000 / (this.metadata.frameRate || 30) : 16.67;

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

  private getDurationMs(): number {
    return this.metadata?.duration ?? 0;
  }

  private toMilliseconds(value: number, timescale: number): number {
    return Math.round((value * 1000) / Math.max(1, timescale));
  }

  private detectContainerFormat(buffer: ArrayBuffer, url?: string): ContainerFormat {
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