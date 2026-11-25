# Releasing

Cloudy is released from `nya-owo/cloudy`, the remote called `fork` in the local clone. `origin` is the original project, egoist/mygo-cloudmusic, which was taken down in November 2026: never push there. Windows is the primary target; macOS and Linux (arm64 and amd64 each, experimental) are built in workflows of their own, so that their failures or runner delays never block Windows releases.

1. Start from the last released main, in a clean worktree, not from an old local branch:

   ```bash
   git fetch fork
   git worktree add -b claude/<work> ../wt-<work> fork/main
   ```

   Commit the work there, and run `go vet ./internal/...` and `go test ./internal/...`. The Test workflow runs the same on every push to main.

2. Bump `version` in `mygo.json`, and add a `## 1.2.7` section at the top of `CHANGELOG.md` saying, in Chinese, what changed for people using the app since the last release (`git log v1.2.6..HEAD`). Commit both in a commit titled just the version, e.g. `1.2.7`. MyGo's updater shows that section in the app's update window, and the release page takes it as its notes; the release fails without one.

3. Push main first, and tag only once that push has gone through. Pushing both at once still pushes the tag when main is rejected, and the tag starts a release build of a commit that isn't on main:

   ```bash
   git push fork HEAD:main
   git tag v1.2.7
   git push fork v1.2.7
   ```

   If main is rejected, someone pushed meanwhile: rebase onto `fork/main`, add what came in to the changelog, amend the version commit, test, and push main again before tagging. If a tag went up on the wrong commit anyway, cancel its run, then delete the tag and the draft it made before tagging again:

   ```bash
   git push fork :refs/tags/v1.2.7
   gh release delete v1.2.7 --repo nya-owo/cloudy --yes
   ```

4. `.github/workflows/release.yml` builds the Windows amd64 and arm64 apps, signs their updates with the `MYGO_UPDATER_PRIVATE_KEY` secret and uploads them to a draft release. Wait for it to pass:

   ```bash
   gh run watch "$(gh run list --repo nya-owo/cloudy --workflow release.yml -L1 --json databaseId -q '.[0].databaseId')" --repo nya-owo/cloudy --exit-status
   ```

   If it fails, don't publish; fix it first. The release should hold the two Windows installers (`cloudy-setup-*-amd64.exe`, `-arm64.exe`) and what installed apps update from (`update-windows-*.json` and the `.tar.gz` archives). `.github/workflows/release-macos.yml` runs independently on the same `v*` tag (and can be re-run via `workflow_dispatch` with a `tag` input) to upload the experimental macOS disk images (`cloudy-*-arm64.dmg`, `-amd64.dmg`), `.tar.gz` archives, and `update-darwin-*.json` manifests to the same release without blocking Windows. `.github/workflows/release-linux.yml` does the same for Linux: the `.deb` packages, the `linux-*.tar.gz` archives, `install.sh` (which installs on Linux and macOS) and `update-linux-*.json`. Check that both ran; a failed one can be run again with its `tag` input after the release is published, since the manifests go up last.

5. Publish the draft and mark it latest. Installed apps update to it from then on:

   ```bash
   gh release edit v1.2.7 --repo nya-owo/cloudy --draft=false --latest
   ```

6. Check the update feed shows the new version:

   ```bash
   curl -sL https://github.com/nya-owo/cloudy/releases/latest/download/update-windows-amd64.json
   curl -sL https://github.com/nya-owo/cloudy/releases/latest/download/update-darwin-arm64.json
   curl -sL https://github.com/nya-owo/cloudy/releases/latest/download/update-darwin-amd64.json
   curl -sL https://github.com/nya-owo/cloudy/releases/latest/download/update-linux-amd64.json
   curl -sL https://github.com/nya-owo/cloudy/releases/latest/download/update-linux-arm64.json
   ```

The `website/` directory is the original project's site and is not deployed.

## The speech recognition libraries

Podcast captions run sherpa-onnx, whose libraries the app downloads with the model from the `sherpa-onnx-<version>` release, not from the app's own. They only change with sherpa-onnx's version, in `scripts/sherpa`. Then pack them again, commit the `internal/speech/natives.go` it writes, and upload them as a prerelease, which never becomes the latest that updates look for:

```bash
go run ./scripts/sherpa
gh release create sherpa-onnx-2.0.1 --repo nya-owo/cloudy --prerelease --latest=false build/sherpa/*
```
