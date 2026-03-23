import { Injectable } from '@angular/core';

/**
 * WebCodecsDecoderService
 * 
 * Manages WebCodecs-based media decoding for video and audio streams.
 * - Consumes PacketBatch from media session protocol
 * - Creates EncodedVideoChunk/EncodedAudioChunk objects
 * - Feeds to VideoDecoder/AudioDecoder
 * - Renders frames to Canvas or outputs audio samples
 * - Handles keyframe-based error recovery
 */

export interface VideoDecoderConfig {
  codec: string;           // WebCodecs codec string (e.g., "avc1.640028")
  width: number;
  height: number;
  description?: Uint8Array; // Initialization data (SPS/PPS)
}

export interface AudioDecoderConfig {
  codec: string;           // WebCodecs codec string (e.g., "mp4a.40.2")
  sampleRate: number;
  channels: number;
  description?: Uint8Array; // Initialization data (AudioSpecificConfig)
}

export interface DecodedVideoFrameInfo {
  timestamp: number;       // Microseconds
  codedWidth: number;
  codedHeight: number;
  duration?: number;
}

export interface DecodedAudioDataInfo {
  timestamp: number;       // Microseconds
  sampleCount: number;
  format: AudioSampleFormat;
}

export type AudioSampleFormat = 'u8' | 's16' | 's32' | 'f32';

@Injectable({
  providedIn: 'root'
})
export class WebCodecsDecoderService {
  private videoDecoder: VideoDecoder | null = null;
  private audioDecoder: AudioDecoder | null = null;

  private videoCanvas: HTMLCanvasElement | null = null;
  private videoCtx: CanvasRenderingContext2D | null = null;

  private audioContext: AudioContext | null = null;
  private audioBuffers: AudioBuffer[] = [];

  private isVideoReady = false;
  private isAudioReady = false;

  private pendingVideoFrames: DecodedVideoFrameInfo[] = [];
  private pendingAudioData: DecodedAudioDataInfo[] = [];

  constructor() {}

  /**
   * Initialize video decoder and rendering pipeline
   */
  async initializeVideoDecoder(
    config: VideoDecoderConfig,
    canvas: HTMLCanvasElement
  ): Promise<void> {
    this.videoCanvas = canvas;
    this.videoCtx = canvas.getContext('2d');

    if (!this.videoCtx) {
      throw new Error('Cannot get 2D context from canvas');
    }

    // Set canvas size to video dimensions
    canvas.width = config.width;
    canvas.height = config.height;

    // Create VideoDecoder
    const videoConfig: any = {
      codec: config.codec,
      codedWidth: config.width,
      codedHeight: config.height,
      description: config.description,
    };

    try {
      const support = await VideoDecoder.isConfigSupported(videoConfig);
      if (!support.supported) {
        throw new Error(`Video codec not supported: ${config.codec}`);
      }

      this.videoDecoder = new VideoDecoder({
        output: (frame: VideoFrame) => this.handleDecodedVideoFrame(frame),
        error: (error: DOMException) => this.handleVideoDecoderError(error),
      });

      this.videoDecoder.configure(videoConfig);
      this.isVideoReady = true;
    } catch (error) {
      throw new Error(`Video decoder initialization failed: ${error}`);
    }
  }

  /**
   * Initialize audio decoder and playback pipeline
   */
  async initializeAudioDecoder(config: AudioDecoderConfig): Promise<void> {
    // Create AudioContext (global singleton pattern)
    if (!this.audioContext) {
      this.audioContext = new AudioContext();
    }

    const audioConfig: any = {
      codec: config.codec,
      numberOfChannels: config.channels,
      sampleRate: config.sampleRate,
      description: config.description,
    };

    try {
      const support = await AudioDecoder.isConfigSupported(audioConfig);
      if (!support.supported) {
        throw new Error(`Audio codec not supported: ${config.codec}`);
      }

      this.audioDecoder = new AudioDecoder({
        output: (data: AudioData) => this.handleDecodedAudioData(data),
        error: (error: DOMException) => this.handleAudioDecoderError(error),
      });

      this.audioDecoder.configure(audioConfig);
      this.isAudioReady = true;
    } catch (error) {
      throw new Error(`Audio decoder initialization failed: ${error}`);
    }
  }

  /**
   * Feed an encoded video chunk to the decoder
   */
  feedVideoChunk(
    data: Uint8Array,
    timestamp: number,
    isKeyframe: boolean,
    duration?: number
  ): void {
    if (!this.videoDecoder || !this.isVideoReady) {
      throw new Error('Video decoder not initialized');
    }

    const chunk = new EncodedVideoChunk({
      type: isKeyframe ? 'key' : 'delta',
      timestamp: timestamp,
      duration: duration || 0,
      data: data,
    });

    try {
      this.videoDecoder.decode(chunk);
    } catch (error) {
      console.error(`Failed to decode video chunk: ${error}`);
      // Will trigger error handler with recovery via keyframe
    }
  }

  /**
   * Feed an encoded audio chunk to the decoder
   */
  feedAudioChunk(
    data: Uint8Array,
    timestamp: number,
    duration?: number
  ): void {
    if (!this.audioDecoder || !this.isAudioReady) {
      throw new Error('Audio decoder not initialized');
    }

    const chunk = new EncodedAudioChunk({
      type: 'key', // Audio doesn't distinguish key/delta
      timestamp: timestamp,
      duration: duration || 0,
      data: data,
    });

    try {
      this.audioDecoder.decode(chunk);
    } catch (error) {
      console.error(`Failed to decode audio chunk: ${error}`);
    }
  }

  /**
   * Flush and close video decoder
   */
  async closeVideoDecoder(): Promise<void> {
    if (this.videoDecoder) {
      try {
        await this.videoDecoder.flush();
        this.videoDecoder.close();
      } catch (error) {
        console.error(`Error closing video decoder: ${error}`);
      }
      this.videoDecoder = null;
      this.isVideoReady = false;
    }
  }

  /**
   * Flush and close audio decoder
   */
  async closeAudioDecoder(): Promise<void> {
    if (this.audioDecoder) {
      try {
        await this.audioDecoder.flush();
        this.audioDecoder.close();
      } catch (error) {
        console.error(`Error closing audio decoder: ${error}`);
      }
      this.audioDecoder = null;
      this.isAudioReady = false;
    }
  }

  /**
   * Get current decode state
   */
  getDecoderState(): {
    videoReady: boolean;
    audioReady: boolean;
    videoDecoded: number;
    audioDecoded: number;
  } {
    return {
      videoReady: this.isVideoReady,
      audioReady: this.isAudioReady,
      videoDecoded: this.videoDecoder?.decodeQueueSize || 0,
      audioDecoded: this.audioDecoder?.decodeQueueSize || 0,
    };
  }

  /**
   * Handle decoded video frame (render to canvas)
   */
  private handleDecodedVideoFrame(frame: VideoFrame): void {
    if (!this.videoCtx) {
      console.warn('Canvas context not available');
      frame.close();
      return;
    }

    try {
      // Draw frame to canvas
      this.videoCtx.drawImage(frame as any, 0, 0);

      this.pendingVideoFrames.push({
        timestamp: frame.timestamp,
        codedWidth: frame.codedWidth,
        codedHeight: frame.codedHeight,
        duration: frame.duration ?? undefined,
      });
    } catch (error) {
      console.error(`Error rendering frame: ${error}`);
    } finally {
      // Always close the frame to free resources
      frame.close();
    }
  }

  /**
   * Handle decoded audio data (queue for playback or analysis)
   */
  private handleDecodedAudioData(data: AudioData): void {
    this.pendingAudioData.push({
      timestamp: data.timestamp,
      sampleCount: data.numberOfFrames,
      format: (data.format as AudioSampleFormat) ?? 'f32',
    });
    data.close();

    // Note: AudioData is opaque in WebCodecs; for playback, integrate with
    // AudioContext or MediaElement. Full implementation would require:
    // - Getting samples from AudioData (if API allows)
    // - Writing to AudioContext buffer
    // - Scheduling playback via AudioContext.currentTime
  }

  /**
   * Handle video decoder errors (request keyframe for recovery)
   */
  private handleVideoDecoderError(error: DOMException): void {
    console.error(`Video decoder error: ${error.message}`);
    // Signal to playback controller: need to seek to nearest keyframe
    // This is typically handled at a higher level via seek() operation
  }

  /**
   * Handle audio decoder errors
   */
  private handleAudioDecoderError(error: DOMException): void {
    console.error(`Audio decoder error: ${error.message}`);
  }

  /**
   * Get pending video frames (for external consumption like recording)
   */
  getPendingVideoFrames(): DecodedVideoFrameInfo[] {
    const frames = [...this.pendingVideoFrames];
    this.pendingVideoFrames = [];
    return frames;
  }

  /**
   * Get pending audio data
   */
  getPendingAudioData(): DecodedAudioDataInfo[] {
    const data = [...this.pendingAudioData];
    this.pendingAudioData = [];
    return data;
  }
}
