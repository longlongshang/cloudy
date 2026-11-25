# Apple Music Lyrics Implementation Plan

**Current direction (2026-11-25):** The user requested removing custom gradient masking and LRC sweeping. Restore HEAD's native RichText word rendering and timing helpers unchanged, inside the existing spring/stagger/depth/left-aligned scale containers. Clear depth blur immediately on the active row. The original implementation and verification notes below are historical; see `2026-11-25-native-lyrics-restoration.md` for the current work.

> **For agentic workers:** Use superpowers:executing-plans to implement the tasks inline. Steps use checkbox syntax for tracking.

**Goal:** Give the existing lyrics pane spring following, staggered rows, visible-only depth blur, left-anchored focus scaling and timed-word gradient highlighting.

**Architecture:** Extend the current scroll-offset-plus-spring-lag implementation. Keep layout stable while each row follows a delayed spring; rendering uses MyGo transforms and native bitmap effects. Preserve the optional perspective arc and shared caption renderer.

**Tech Stack:** Go 1.27, pinned github.com/nya-owo/mygo fork, existing LRC/YRC/KRC/TTML parsers.

**Spec:** User request in this conversation, 2026-11-24.

## Global Constraints

- No new dependencies or parser changes.
- Focus at 38% of the lyrics viewport, 35ms stagger per row capped at 200ms, 1.06x left-center focus scaling.
- User scroll clears depth blur immediately; resume 1.5 seconds after the last scroll/drag.
- Only genuinely timed words receive a horizontal gradient sweep; preserve whole-line and translation settings.
- Preserve click seeking including lyric offset, track reset, perspective and reduced-motion behavior.
- All state stays on the main thread; use synthetic Tester data for previews.

## Review Focus

- Scrolling while a spring is moving must not jump to its hidden target.
- Scroll/drag at a boundary must still suspend following and clear blur.
- Reflow and translation/font changes must recalculate focus without appearing as manual scrolling.
- Delayed rows must remain continuous on rapid line changes and long seeks.
- Wrapped timed words must keep the same layout before, during and after their sweep.

### Task 1: Following and user takeover

**Files:** internal/app/ui_nowplaying.go, internal/app/lyric_perspective_test.go

- [x] Add behavioral tests for manual scroll holding and returning after 1.5s even when the active line has not changed; shifted lyric click seeking; track reset.
- [x] Run focused tests and confirm missing behavior fails.
- [x] Extend lyricGlide for per-row delayed movement, preserve velocity when retargeted, and stop all row motion during manual takeover.
- [x] Use a 38% focus band with enough top/bottom padding; retain the perspective arc and bounce slider.
- [x] Run focused UI tests with -count=5.

### Task 2: Depth and timed-word rendering

**Files:** internal/app/ui_nowplaying.go, internal/app/ui_light.go, internal/app/perf_test.go

- [x] Gate blur and glow to visible row bounds, including animated lag; bypass depth effects during manual reading.
- [x] Use 1.06 focus scale with left-center anchor and reserve width for scaling.
- [x] Render timed words through a native horizontal alpha gradient while keeping shaping/wrapping stable; leave LRC as whole-line highlighting.
- [x] Add an idle-frame regression and synthetic NM_FRAMES previews; inspect stills of rest, transition, manual reading and wrapped bilingual text.

### Task 3: Verification

- [x] Run gofmt, go vet ./internal/..., go tool mygo vet ., and CGO_ENABLED=0 go test ./internal/....
- [x] Review the diff and verify no parser/dependency/private-message changes.
- [x] Report verification results and any remaining native/GPU limitations.

## Verification record

- `gofmt` and `go vet -mod=readonly ./internal/...`: passed; changed-file editor diagnostics clean.
- `go test -mod=readonly ./internal/app -run 'TestLyric|TestGlow|TestNpWhole|TestNowPlayingFrameBudget' -count=1 -timeout=5m`: passed (44.655s).
- Seven new interaction/frame-budget tests, `-count=5`: passed (10.633s). The existing perspective reflow test also passed five runs in the earlier combined validation (90.034s).
- Synthetic `NM_FRAMES` previews inspected: bilingual wrapping, focus scaling, partially sung English/CJK words, stagger, clear manual reading and resumed focus. No native application launch or real GPU frame-rate measurement.
- Independent review identified discontinuities when dragging during the spring and when cancelling staggered row offsets. Both fixed by reading input before styling and preserving each row's residual position during manual reading. Regression tests reproduce the former jumps and now pass.
- Ruling: preserve the existing blur opt-out and perspective zero-tilt behavior using `st.tilt > 0 || LyricBlur`; the new flat focus anchor must not enable blur by itself.
- Full `go test -mod=readonly ./internal/...` was run: all non-app packages passed. App tests reported `TestDownloadModelFromCaptions`, `TestCaptionClickSeeks`, and `TestCaptionsOnTheirWay` failures; all three were independently reproduced with the original HEAD versions of every changed Go file through a temporary Go overlay. The full run then hit its default 10-minute timeout during `TestNowPlayingCalmKeepsProgressInWideWindow/1180x580`; the stack was in the headless CPU blur renderer. Full-suite success is not claimed.
- `go tool mygo vet .` ran and reported the pre-existing `settingsKit.c *ui.Context` field in `internal/app/ui_settings.go:250` as a native UI lifetime issue. The original HEAD file contains the same field. No new lifetime issues were reported.
- No parser, module dependency, private-message or framework changes. Work remains uncommitted for review.
