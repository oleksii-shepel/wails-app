# AVI Playback Settings

This file is the current source of truth for AVI-family preview behavior in the app.

Scope:

- `.avi`
- `.divx`
- `.xvid`

Status note:

- The rules below are exact code-configured behavior from the current tree.
- They are not a claim that every combination was manually verified at runtime in this session.

## Current Baseline

As of the current code, AVI handling was restored to the older fallback path associated with commit `a16b456` (`added scripts`) for transcoded preview serving.

That means:

- AVI is allowed to attempt segmented playback again.
- AVI-family segmented playback now uses keyframe-aligned segment boundaries instead of fixed 2-second windows.
- When the app falls back to streamed MP4 preview, it uses the legacy `startAstiavTranscode` / `startAstiavTranscodeReader` path.
- The `mode` query parameter is still attached to preview URLs, but the restored legacy astiav path does not currently branch on it internally.

## Effective Behavior Matrix

| Source | First Attempt | Fallback Plan | Seekable | Notes |
| --- | --- | --- | --- | --- |
| Disk AVI | `segmented` | `remux` or `transcode`, depending on codec probe | Yes | Segmented AVI now uses keyframe-aligned windows; if segmented build fails, the fallback URL includes `mode=<plan-mode>` and a 90-second preview window. |
| Archive AVI | `segmented` | `transcode` | Segmented: yes. Fallback: no | Segmented AVI archive entries also use keyframe-aligned windows. Archive fallback preview rejects `start>0`, so streamed archive fallback is not seekable. |

## Exact Backend Rules

### 1. Segmented playback is allowed for AVI again

AVI-family files are included in segmented playback attempts:

- `shouldAttemptSegmentedDiskPlayback()` includes `.avi`, `.divx`, `.xvid`
- `buildDiskVideoPlaybackPlan()` tries segmented playback first
- `buildArchiveVideoPlaybackPlan()` tries segmented playback first

For AVI-family segmented playback, the cache builder now uses keyframe-aligned segment ranges instead of fixed 2-second ranges:

- `shouldUseKeyframeAlignedSegments()` includes `.avi`, `.divx`, `.xvid`
- segment manifests preserve real per-segment start/duration values derived from keyframes

Practical consequence:

- segmented AVI playback avoids duplicate-scene overlap caused by backward keyframe seek into a fixed-size segment window
- segment lengths are no longer uniform; they follow GOP spacing

### 2. Disk fallback mode depends on codecs

If segmented playback is unavailable for a disk AVI file, the fallback mode is chosen like this:

1. If video codec is remuxable and audio codec is remuxable, plan mode is `remux`
2. Otherwise, if the codec is on the transcode-preferred list or the extension is AVI-family, plan mode is `transcode`
3. Otherwise, plan mode falls back to `remux`

Remuxable codecs:

- Video: `h264`, `hevc`, `h265`, `av1`
- Audio: empty, `aac`

Transcode-preferred video codecs:

- `mpeg2video`
- `vc1`
- `mjpeg`
- `mpeg4`
- `msmpeg4v3`
- `wmv3`
- `vp6`
- `vp6f`

Practical consequence:

- Typical legacy AVI content such as MPEG-4/Xvid/DivX lands on a `transcode` plan
- AVI containing modern H.264 plus AAC can land on a `remux` plan

### 3. Archive fallback is stricter

If segmented playback is unavailable for an archive AVI entry, the fallback plan is effectively `transcode`.

Reason:

- `decidePlaybackMode()` only returns `remux` for `.mov`, `.qt`, `.flv`, `.ts`, `.mts`, `.m2ts`
- AVI-family extensions are not in that remux extension set
- AVI-family extensions are in `shouldTranscodeVideoPreview()`

### 4. Restored fallback serving path

The current transcoded AVI fallback is served by:

- `serveTranscodedPath()` -> `startAstiavTranscode(...)`
- `serveTranscodedArchiveEntry()` -> `startAstiavTranscodeReader(...)`

This is the restored older path.

Important detail:

- `startAstiavTranscode` and `startAstiavTranscodeReader` currently call `remuxLoop(...)`
- The `playbackMode` argument is passed through, but is not used inside those functions today

So the exact current situation is:

- plan selection still distinguishes `remux` vs `transcode`
- the restored fallback implementation currently uses the same legacy astiav pipeline for both labels

## Seek Behavior

### Disk AVI

- Segmented path: seekable
- Fallback streamed MP4 path: seekable through the preview URL and `start` parameter

### Archive AVI

- Segmented path: seekable
- Fallback streamed MP4 path: not seekable
- `serveTranscodedArchiveEntry()` returns `400` if `startSeconds > 0`

## Files That Define This Behavior

- `internal/app/stream_handler.go`
- `internal/app/astiav_transcoder.go`
- `frontend/src/app/surfaces/viewer/viewer-surface.component.ts`

## Manual Verification Checklist

Use this section to record what was actually confirmed in the running app.

| Scenario | Expected Result | Status |
| --- | --- | --- |
| Disk AVI opens through segmented path | Canvas preview starts and scrub works | Not verified in this session |
| Disk AVI falls back cleanly when segmented build fails | MP4 preview starts from legacy astiav path | Not verified in this session |
| Disk AVI seek after fallback | Seeking works with restarted preview URL | Not verified in this session |
| Archive AVI opens through segmented path | Canvas preview starts and scrub works | Not verified in this session |
| Archive AVI fallback preview | Preview plays, but seek is unavailable | Not verified in this session |
| AVI with H.264/AAC on disk | Plan may report `remux` | Not verified in this session |
| Legacy Xvid/DivX AVI on disk | Plan is expected to report `transcode` | Not verified in this session |
