import { Injectable } from '@angular/core';
import { BehaviorSubject, Subject } from 'rxjs';
import { MediaSessionProtocol, SessionInfo, TrackInfo, PacketBatch } from './media-session-protocol.service';
import { WebCodecsDecoderService } from './webcodecs-decoder.service';

/**
 * MediaPlaybackController
 * 
 * Orchestrates the complete media playback pipeline:
 * - Session lifecycle management (open → fetch → decode → render → close)
 * - Prefetch logic (buffers 1-2 seconds ahead)
 * - Seeking and resumption
 * - Audio/video synchronization
 * - Error recovery via keyframe seeking
 */

export interface PlaybackState {
  isPlaying: boolean;
  currentTimeMs: number;
  durationMs: number;
  bufferedMs: number;
  isBuffering: boolean;
  error?: string;
}

export interface TrackSelection {
  videoTrackId?: number;
  audioTrackId?: number;
}

@Injectable({
  providedIn: 'root'
})
export class MediaPlaybackController {
  private sessionId: string | null = null;
  private sessionInfo: SessionInfo | null = null;

  private canvas: HTMLCanvasElement | null = null;
  private playbackState$ = new BehaviorSubject<PlaybackState>({
    isPlaying: false,
    currentTimeMs: 0,
    durationMs: 0,
    bufferedMs: 0,
    isBuffering: false,
  });

  private error$ = new Subject<Error>();
  private videoTrackId: number | null = null;
  private audioTrackId: number | null = null;

  // Prefetch management
  private prefetchBuffer: Map<number, PacketBuffer> = new Map();
  private prefetchCursor: Map<number, string> = new Map();
  private PREFETCH_DURATION_MS = 2000; // Keep 2 seconds buffered
  private PACKET_BATCH_SIZE = 50;

  private playbackTimer: any = null;
  private lastFrameTimestampUs = 0;

  constructor(
    private protocol: MediaSessionProtocol,
    private decoder: WebCodecsDecoderService
  ) {}

  /**
   * Open a media file for playback
   */
  async openMedia(filePath: string, canvas: HTMLCanvasElement): Promise<void> {
    try {
      // Close any existing session
      if (this.sessionId) {
        await this.closeMedia();
      }

      this.canvas = canvas;
      this.updatePlaybackState({ isBuffering: true });

      // Open session on backend
      this.sessionInfo = await this.protocol.open(filePath);
      this.sessionId = this.sessionInfo.session_id;

      // Find first video and audio tracks
      const videoTrack = this.sessionInfo.tracks.find((t: TrackInfo) => t.kind === 'video');
      const audioTrack = this.sessionInfo.tracks.find((t: TrackInfo) => t.kind === 'audio');

      if (!videoTrack && !audioTrack) {
        throw new Error('No video or audio tracks found in media file');
      }

      // Initialize decoders
      if (videoTrack) {
        this.videoTrackId = videoTrack.id;
        await this.decoder.initializeVideoDecoder(
          {
            codec: videoTrack.codec,
            width: videoTrack.width || 1920,
            height: videoTrack.height || 1080,
          },
          canvas
        );
      }

      if (audioTrack) {
        this.audioTrackId = audioTrack.id;
        await this.decoder.initializeAudioDecoder({
          codec: audioTrack.codec,
          sampleRate: audioTrack.sample_rate || 44100,
          channels: audioTrack.channels || 2,
        });
      }

      // Initialize prefetch buffers
      if (this.videoTrackId !== null) {
        this.prefetchBuffer.set(this.videoTrackId, new PacketBuffer());
        this.prefetchCursor.set(this.videoTrackId, '0');
      }
      if (this.audioTrackId !== null) {
        this.prefetchBuffer.set(this.audioTrackId, new PacketBuffer());
        this.prefetchCursor.set(this.audioTrackId, '0');
      }

      // Start prefetch
      this.startPrefetching();

      this.updatePlaybackState({
        durationMs: this.sessionInfo.duration_ms,
        isBuffering: false,
      });
    } catch (error) {
      this.error$.next(error as Error);
      this.updatePlaybackState({ error: (error as Error).message });
      throw error;
    }
  }

  /**
   * Start playback
   */
  async play(): Promise<void> {
    if (!this.sessionId) {
      throw new Error('No media session open');
    }

    const state = this.playbackState$.value;
    if (!state.isPlaying) {
      this.updatePlaybackState({ isPlaying: true });
      this.startPlaybackTimer();
    }
  }

  /**
   * Pause playback
   */
  pause(): void {
    const state = this.playbackState$.value;
    if (state.isPlaying) {
      this.updatePlaybackState({ isPlaying: false });
      this.stopPlaybackTimer();
    }
  }

  /**
   * Seek to a specific time
   */
  async seek(timeMs: number): Promise<void> {
    const state = this.playbackState$.value;

    try {
      this.updatePlaybackState({ isBuffering: true, currentTimeMs: timeMs });
      this.stopPlaybackTimer();

      // For each active track, seek and rebuffer
      if (this.videoTrackId !== null) {
        const seekResult = await this.protocol.seek(this.sessionId!, this.videoTrackId, timeMs);
        this.prefetchCursor.set(this.videoTrackId, seekResult.cursor);
        const buffer = this.prefetchBuffer.get(this.videoTrackId);
        if (buffer) {
          buffer.clear();
        }
      }

      if (this.audioTrackId !== null) {
        const seekResult = await this.protocol.seek(this.sessionId!, this.audioTrackId, timeMs);
        this.prefetchCursor.set(this.audioTrackId, seekResult.cursor);
        const buffer = this.prefetchBuffer.get(this.audioTrackId);
        if (buffer) {
          buffer.clear();
        }
      }

      this.lastFrameTimestampUs = timeMs * 1000;

      // Resume playback if it was playing before seek
      if (state.isPlaying) {
        await this.play();
      }

      this.updatePlaybackState({ isBuffering: false });
    } catch (error) {
      this.error$.next(error as Error);
      this.updatePlaybackState({ error: (error as Error).message });
      throw error;
    }
  }

  /**
   * Close media and clean up resources
   */
  async closeMedia(): Promise<void> {
    try {
      this.stopPlaybackTimer();
      this.stopPrefetching();

      await this.decoder.closeVideoDecoder();
      await this.decoder.closeAudioDecoder();

      if (this.sessionId) {
        await this.protocol.close(this.sessionId);
        this.sessionId = null;
      }

      this.sessionInfo = null;
      this.prefetchBuffer.clear();
      this.prefetchCursor.clear();

      this.updatePlaybackState({
        isPlaying: false,
        currentTimeMs: 0,
        durationMs: 0,
        bufferedMs: 0,
      });
    } catch (error) {
      console.error('Error closing media:', error);
    }
  }

  /**
   * Get playback state observable
   */
  getPlaybackState() {
    return this.playbackState$.asObservable();
  }

  /**
   * Get error observable
   */
  getErrors() {
    return this.error$.asObservable();
  }

  /**
   * Get session info
   */
  getSessionInfo(): SessionInfo | null {
    return this.sessionInfo;
  }

  // ============ PRIVATE IMPLEMENTATION ============

  private startPlaybackTimer(): void {
    if (this.playbackTimer) {
      return;
    }

    const startTime = performance.now();
    const playbackStartMs = this.playbackState$.value.currentTimeMs;

    this.playbackTimer = setInterval(() => {
      const state = this.playbackState$.value;

      if (!state.isPlaying) {
        return;
      }

      const elapsed = performance.now() - startTime;
      const newTimeMs = playbackStartMs + elapsed;

      if (newTimeMs >= state.durationMs) {
        // Playback finished
        this.pause();
        this.updatePlaybackState({ currentTimeMs: state.durationMs });
      } else {
        this.updatePlaybackState({ currentTimeMs: newTimeMs });
      }
    }, 30); // Update every 30ms (≈33 FPS)
  }

  private stopPlaybackTimer(): void {
    if (this.playbackTimer) {
      clearInterval(this.playbackTimer);
      this.playbackTimer = null;
    }
  }

  private prefetchTimer: any = null;

  private startPrefetching(): void {
    if (this.prefetchTimer) {
      return;
    }

    this.prefetchTimer = setInterval(async () => {
      await this.doPrefetch();
    }, 100); // Check prefetch buffer every 100ms
  }

  private stopPrefetching(): void {
    if (this.prefetchTimer) {
      clearInterval(this.prefetchTimer);
      this.prefetchTimer = null;
    }
  }

  private async doPrefetch(): Promise<void> {
    if (!this.sessionId || !this.sessionInfo) {
      return;
    }

    const state = this.playbackState$.value;

    try {
      // Prefetch video packets
      if (this.videoTrackId !== null) {
        const buffer = this.prefetchBuffer.get(this.videoTrackId);
        if (!buffer) return;

        const bufferedMs = buffer.getDurationMs();

        if (bufferedMs < this.PREFETCH_DURATION_MS) {
          const cursor = this.prefetchCursor.get(this.videoTrackId) || '0';
          const batch = await this.protocol.readPackets(
            this.sessionId,
            this.videoTrackId,
            cursor,
            this.PACKET_BATCH_SIZE
          );

          // Add packets to buffer
          for (const packet of batch.packets) {
            this.decoder.feedVideoChunk(
              batch.payload.slice(packet.offset, packet.offset + packet.size),
              packet.timestamp_us,
              packet.is_keyframe,
              packet.duration_us
            );
          }

          buffer.addPackets(batch.packets);

          // Update cursor for next fetch
          if (batch.next_cursor) {
            this.prefetchCursor.set(this.videoTrackId, batch.next_cursor);
          }
        }
      }

      // Prefetch audio packets
      if (this.audioTrackId !== null) {
        const buffer = this.prefetchBuffer.get(this.audioTrackId);
        if (!buffer) return;

        const bufferedMs = buffer.getDurationMs();

        if (bufferedMs < this.PREFETCH_DURATION_MS) {
          const cursor = this.prefetchCursor.get(this.audioTrackId) || '0';
          const batch = await this.protocol.readPackets(
            this.sessionId,
            this.audioTrackId,
            cursor,
            this.PACKET_BATCH_SIZE
          );

          // Add packets to buffer
          for (const packet of batch.packets) {
            this.decoder.feedAudioChunk(
              batch.payload.slice(packet.offset, packet.offset + packet.size),
              packet.timestamp_us,
              packet.duration_us
            );
          }

          buffer.addPackets(batch.packets);

          // Update cursor for next fetch
          if (batch.next_cursor) {
            this.prefetchCursor.set(this.audioTrackId, batch.next_cursor);
          }
        }
      }

      // Update buffered duration in state
      const videoBuffer = this.prefetchBuffer.get(this.videoTrackId || 0);
      const bufferedMs = videoBuffer?.getDurationMs() || 0;
      this.updatePlaybackState({ bufferedMs });
    } catch (error) {
      console.error('Prefetch error:', error);
      this.error$.next(error as Error);
    }
  }

  private updatePlaybackState(partial: Partial<PlaybackState>): void {
    const current = this.playbackState$.value;
    this.playbackState$.next({ ...current, ...partial });
  }
}

/**
 * Helper: Buffer to track prefetched packets and their total duration
 */
class PacketBuffer {
  private packets: any[] = [];

  addPackets(packets: any[]): void {
    this.packets.push(...packets);
  }

  clear(): void {
    this.packets = [];
  }

  getDurationMs(): number {
    if (this.packets.length === 0) {
      return 0;
    }

    const totalDurationUs = this.packets.reduce((sum, p) => sum + (p.durationUs || 0), 0);
    return totalDurationUs / 1000;
  }

  getPackets(): any[] {
    return [...this.packets];
  }
}
