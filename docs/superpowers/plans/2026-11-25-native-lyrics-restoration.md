# Native Lyrics Restoration

**Request:** Work on `feat/apple-music-lyrics`; remove handwritten gradient masking and LRC fallback sweeping, restore the original word parser/renderer, and retain Apple Music outer motion.

## Changes

- Created the requested branch with every existing uncommitted change carried over; `main`'s commit is unchanged.
- Restored HEAD's original `ui.RichText` word spans, `wordLit`, `lineGlow` and 40ms word-update timer. Existing LRC/YRC/KRC/TTML parsing was already unchanged.
- Removed the custom canvas, per-band image slicing, per-word bitmap lift, line-duration override and sweep-specific tests.
- Kept the 38% focus anchor, spring following, 35ms row staggering (200ms cap), left-anchored 1.06 focus scaling, non-active visible-row depth blur, and manual scrolling/dragging takeover.
- The active row now resets depth to zero immediately, including the first frame after a line switch, so the native word renderer never inherits a neighbor's animated blur. The original glow remains a separate decorative sibling; it does not filter the foreground text.

## Verification

- Native RichText rendering block compared byte-for-byte (normalized newlines) with HEAD: identical. `git diff --exit-code` for `ui_light.go` and all lyric/API/parser packages: no changes.
- `go vet -mod=readonly ./internal/...`: passed.
- `CGO_ENABLED=0 go test -mod=readonly ./... -run '^$' -timeout=2m`: every package and its tests compile successfully.
- Seven spring, manual-scroll, drag, seek, stagger and idle regressions passed with `-count=5` (11.063s).
- Focused lyrics, glow, timing, whole-line setting and frame-budget tests passed together (32.709s).
- Native lyric, newly active row, stagger, and manual-reading PNG previews inspected in `%TEMP%/cloudy-native-lyrics-frames`. No custom band-mask rendering remains.
- Independent review found no actionable issues; confirmed native restoration, preserved outer effects and immediate active-row depth reset.
- `go tool mygo vet .` still reports the pre-existing `settingsKit.c *ui.Context` at `internal/app/ui_settings.go:250`. This file was not modified.
- Prepared on `feat/apple-music-lyrics`; `main` remains at `5fd77dd3126e2f3141b9c7d95ee39ac32213e1d7`.
- Windows x64 Release rebuilt with `go tool mygo build -platform windows/amd64 -o build/release` (41.1s), including the installer. Refreshed the portable ZIP and its SHA-256 file; all five archived application/resource files were checked against their source hashes.

## Backup verification

- User requested committing all current changes and pushing this feature branch to `https://github.com/longlongshang/cloudy.git`, with the original repository retained as `upstream`.
- Fresh `go vet -mod=readonly ./internal/...` passed; the focused lyrics/glow/word-timing/whole-line/frame-budget suite passed (29.263s).
- Fresh `CGO_ENABLED=0 go test -mod=readonly -failfast -timeout=5m ./internal/...` passed every non-app package and stopped in `TestPoolLeavesComeAndGo` (`backdrop_test.go:255`, "1 leaves in the picture at 651.7 s"). That backdrop test is unchanged. The full suite is not claimed green; the requested backup proceeds with this result recorded.
- Build outputs remain excluded by `.gitignore`; only source, tests and implementation records are included in the backup commit.
