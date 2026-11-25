# Smooth Lyric Sweep Implementation Plan

**Superseded:** The user subsequently reported vertical band artifacts and requested restoring the original native word renderer. `internal/app/ui_lyric_sweep.go`, its fallback timing logic and its dedicated tests have been removed. This document records the abandoned implementation; see `2026-11-25-native-lyrics-restoration.md` for the current direction.

**Goal:** Add timed-word lift and smooth highlighting, with a continuous line sweep for ordinary LRC, to the existing Apple Music lyrics view.

**Architecture:** Retain YRC/KRC/TTML's existing `Line.Timed` and `Word.Start/End` model. Cache active-paragraph shaping and canvas allocation; refresh its glyph commands per paint to respect the font atlas lifetime. Its Draw callback samples the audio position and paints the moving feathered edge at 60fps without rebuilding the list. Keep inactive rows as native text.

**Spec:** User request, 2026-11-25, in this conversation.

**Constraints:** Preserve the current uncommitted lyrics changes, wrapping, translations, click seeking, whole-line opt-out, main-thread ownership and reduced-motion preference. No dependencies or framework edits.

**Review focus:** Pausing and seeking; missing or zero word durations; wrapped English/CJK text; long LRC lines and final-line duration; idle/offscreen redraws.

### Task 1: Timing and paint budget

- [x] Add regressions for exact short-word timing and playback-only redraws for timed and LRC lines; verify failure.
- [x] Replace build timers with an active-line Draw callback, cached shaping/canvas allocation and real audio-clock sampling.
- [x] Use the next line's start for LRC duration and an existing/estimated end for the last line; preserve the whole-line setting.
- [x] Verify paused, offscreen and completed lines stop continuous redraws; run new UI tests five times.

### Task 2: Feathered sweep and lift

- [x] Paint the original shaped fragments through a soft horizontal edge; retain geometry across wrap boundaries.
- [x] Move the singing word up by at most 1.5 DIP and return smoothly by its end; disable lift for reduced motion.
- [x] Save and inspect synthetic YRC/LRC/wrapped previews with NM_FRAMES, including 150%/200% scaling.

### Task 3: Verification

- [x] Run focused lyrics regressions, Go vet, MyGo vet and `CGO_ENABLED=0 go test ./internal/...`.
- [x] Review the patch and record existing failures and native/GPU verification limits.

## Decisions

- Work in the supplied dirty checkout to extend the user's in-progress lyrics implementation.
- Reuse existing true word timestamps rather than duplicating the model/parser.
- Read the audio clock in the paint callback: its position already accounts for heard audio, seeking and playback speed, so a separate wall-clock estimate would introduce drift.
- Review fix: fade perspective rows individually, as flat rows do, to keep the 60fps Draw callback outside an ancestor effect canvas. Tradeoff: edge fading follows rows rather than a viewport-wide gradient.
- Review fix: the sweep supplies highlighting in word mode; retain the existing blurred aura for whole-line mode. This avoids a glow frozen at the previous build position or illuminating unsung LRC words. Tradeoff: word mode omits the decorative blurred aura.
- Review fix: shape the canvas source at its pixel size so 150%/200% display scaling uses native-resolution glyphs.
- Review fix: enable sweeping only in the music lyrics pane; shared speech captions keep their recorded end and native whole-line rendering.

## Verification record

- Initial regressions reproduced 10 list rebuilds and 10 still-neighbor paints in 20 display frames; ordinary LRC had no sweep. Short and zero-length word timing also failed before the fix.
- Independent source review found perspective ancestor-layer repainting, stale glow, HiDPI source rasterization and unintended caption sweeping. Perspective and caption regressions failed before their fixes; all new behavioral/performance tests passed with `-count=5` afterward (15.996s).
- `go test -mod=readonly ./internal/lyrics ./internal/netease ./internal/wordlyrics -count=1`: passed.
- Root package compilation with `CGO_ENABLED=0 go test -mod=readonly . -run '^$'`: passed.
- `go vet -mod=readonly ./internal/...`: passed; `git diff --check`: passed.
- `go tool mygo vet .`: reports the existing `settingsKit.c *ui.Context` field at `internal/app/ui_settings.go:250`; confirmed the same field is in HEAD.
- `TestDownloadModelFromCaptions`, `TestCaptionClickSeeks`, and `TestCaptionsOnTheirWay` fail in the current tree and also in a temporary Go overlay restoring every changed Go file to HEAD and excluding the new files. These failures are independent of this change.
- Synthetic full-pane, English/CJK, wrapped, LRC and HiDPI PNGs inspected in `%TEMP%/cloudy-lyric-sweep-frames`. No native-window/GPU frame-rate measurement performed.
- Final focused app run (`TestLyric|TestGlow|TestWordHeld|TestNowPlayingWordsSwitch|TestNowPlayingFrameBudget|TestCaptionLinesDoNotSweep`): passed (35.890s).
- Full `CGO_ENABLED=0 go test -mod=readonly ./internal/... -timeout=15m`: every non-app package passed. App reported the three baseline caption failures above, then reached the 15-minute suite timeout (the newly started `TestPlayerBarCompactActions` was listed with 0s elapsed, so this does not establish a hang in that test).
- A final-code full run excluding those three failures and the previously slow `TestNowPlayingCalmKeepsProgressInWideWindow` reached its 10-minute suite timeout while `TestPrivateStagedPresentation` was 6s old, with no other assertion failures reported. Every non-app package passed again. Neither timeout is claimed as a passing app suite or attributed to the test running when the alarm fired.
- Work remains uncommitted in the supplied checkout; no dependency/framework changes. Native GPU cadence remains unmeasured.
