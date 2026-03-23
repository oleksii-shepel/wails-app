import { Injectable } from '@angular/core';

/**
 * MediaSessionProtocol handles the binary session-based media protocol.
 * 
 * Protocol:
 * - POST /wails-stream/session/open → SessionInfo
 * - GET /wails-stream/session/packets → binary framed PacketBatch
 * - POST /wails-stream/session/seek → SeekResult
 * - POST /wails-stream/session/close → status
 */

export interface TrackInfo {
  id: number;
  kind: 'video' | 'audio' | 'subtitle';
  codec: string; // WebCodecs format: "avc1.640028", "mp4a.40.2", etc.
  
  // Video
  width?: number;
  height?: number;
  fps?: number;
  
  // Audio
  sample_rate?: number;
  channels?: number;
  
  // Bitstream format for frontend decoder
  bitstream_format: string; // "annexb", "avcc", "raw", etc.
  description_b64?: string; // Base64-encoded init data (SPS/PPS, AudioSpecificConfig, etc.)
  has_b_frames?: boolean;
  keyframe_count?: number;
}

export interface SessionInfo {
  session_id: string;
  title: string;
  duration_ms: number;
  seekable: boolean;
  tracks: TrackInfo[];
  seek_points_ms: number[];
}

export interface PacketMeta {
  cursor: string;
  offset: number;
  size: number;
  timestamp_us: number;
  duration_us: number;
  is_keyframe: boolean;
}

export interface PacketBatch {
  session_id: string;
  track_id: number;
  next_cursor: string;
  packets: PacketMeta[];
  payload: Uint8Array;
}

export interface SeekResult {
  session_id: string;
  resolved_time_ms: number;
  cursor: string;
  is_keyframe: boolean;
}

@Injectable({
  providedIn: 'root',
})
export class MediaSessionProtocol {
  private baseURL = '/wails-stream/session';

  /**
   * Open a media session from a disk path or archive entry.
   */
  async open(path: string): Promise<SessionInfo> {
    const resp = await fetch(`${this.baseURL}/open`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ path }),
    });

    if (!resp.ok) {
      throw new Error(`Failed to open session: ${resp.statusText}`);
    }

    return resp.json();
  }

  /**
   * Read a batch of codec packets from a session.
   * 
   * Returns binary-framed response: [4-byte LE header length][JSON header][binary payload]
   */
  async readPackets(
    sessionId: string,
    trackId: number,
    cursor: string,
    limit: number = 64
  ): Promise<PacketBatch> {
    const url = new URL(`${this.baseURL}/packets`, window.location.origin);
    url.searchParams.set('session_id', sessionId);
    url.searchParams.set('track_id', String(trackId));
    url.searchParams.set('cursor', cursor);
    url.searchParams.set('limit', String(limit));

    const resp = await fetch(url.toString());
    if (!resp.ok) {
      throw new Error(`Failed to read packets: ${resp.statusText}`);
    }

    // Parse binary-framed response
    const buffer = await resp.arrayBuffer();
    return this.parseFramedPacketBatch(buffer);
  }

  /**
   * Seek to a time position in the media.
   */
  async seek(
    sessionId: string,
    trackId: number,
    timeMs: number
  ): Promise<SeekResult> {
    const resp = await fetch(`${this.baseURL}/seek`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        session_id: sessionId,
        track_id: trackId,
        time_ms: timeMs,
      }),
    });

    if (!resp.ok) {
      throw new Error(`Failed to seek: ${resp.statusText}`);
    }

    return resp.json();
  }

  /**
   * Close a media session.
   */
  async close(sessionId: string): Promise<void> {
    const resp = await fetch(`${this.baseURL}/close`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ session_id: sessionId }),
    });

    if (!resp.ok) {
      throw new Error(`Failed to close session: ${resp.statusText}`);
    }
  }

  /**
   * Parse binary-framed packet batch response.
   * Format: [4-byte LE header length][JSON header][binary payload]
   */
  private parseFramedPacketBatch(buffer: ArrayBuffer): PacketBatch {
    const view = new DataView(buffer);
    
    // Read header length (4 bytes, little-endian)
    if (buffer.byteLength < 4) {
      throw new Error('Packet batch response too short');
    }
    
    const headerLen = view.getUint32(0, true);
    if (buffer.byteLength < 4 + headerLen) {
      throw new Error('Packet batch response: header overrun');
    }

    // Extract and parse JSON header
    const headerBytes = new Uint8Array(buffer, 4, headerLen);
    const headerText = new TextDecoder().decode(headerBytes);
    const header = JSON.parse(headerText);

    // Extract binary payload
    const payloadStart = 4 + headerLen;
    const payload = new Uint8Array(buffer, payloadStart);

    return {
      session_id: header.session_id,
      track_id: header.track_id,
      next_cursor: header.next_cursor,
      packets: header.packets,
      payload,
    };
  }
}
