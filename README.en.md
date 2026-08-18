<p align="right"><a href="./README.md">简体中文</a> | English</p>

# ArticleToMotion

ArticleToMotion is a macOS and Linux CLI for producing vertical motion-graphics videos.

The canvas is chosen once at `am init` — interactively, or with `--canvas vertical-3x4` / `--canvas vertical-9x16` (1080×1440 and 1080×1920, both 30fps). Non-interactive environments must pass `--canvas` explicitly; there is no silent default. It can split a final SRT into concurrently rendered scenes or drive a complete script, TTS, subtitle, cover, audio, and publishing workflow.

▶ [Watch the ArticleToMotion promo video (MP4, 4m 47s)](https://github.com/chouheiwa/articale-to-motion/blob/main/docs/article-to-motion-tutorial.mp4)

The Go release ships as one `am` binary with embedded project templates. It does not require Python. `am init` installs a pinned official HyperFrames skill release instead of redistributing local skill files.

## Install and initialize

Download the appropriate archive from GitHub Releases, verify it with `checksums.txt`, and place `am` on `PATH`. Requirements:

- macOS or Linux (amd64/arm64)
- Node.js 22+
- Git, FFmpeg, FFprobe
- At least one authenticated CLI: Codex, Claude Code, Qoder, CodeBuddy, or OpenCode

```bash
am init my-video                          # interactive canvas picker
am init my-video --canvas vertical-9x16   # or specify explicitly
cd my-video
```

Canvas is chosen once at init and written to `frame.md`. Built-in presets:

| Preset | Canvas | Use case |
|---|---|---|
| `vertical-3x4` | 1080×1440, 30fps | Default, knowledge/tech vertical |
| `vertical-9x16` | 1080×1920, 30fps | Full-screen vertical (TikTok, etc.) |

Non-interactive environments (CI, pipes) must pass `--canvas` explicitly; there is no silent default.

`am init` refuses to overwrite existing files with different content. By default it runs `npx --yes hyperframes@0.8.1 skills`; use `--skip-hyperframes` for offline or CI environments.

Initialization also writes the binary's built-in skill tree to the project's `.agents/skills/`, where the renderer discovers it automatically:

| Skill | Purpose |
|---|---|
| `text-to-lottie` | Lottie layers inside a scene: logo reveals, icons, loaders, SVG stroke-on, vector effects |
| `algorithmic-art` | Algorithmic and generative visuals |

Both ship inside the binary. They need no network install, and `--skip-hyperframes` does not affect them.

Edit `article-to-motion.conf` to choose the orchestrator and renderer (they may be the same CLI):

```text
ORCHESTRATOR=codex
RENDERER=claude
TTS_PROVIDER=minimax
SCENE_JOBS=3
```

Priority: environment variables > project `.env` > config file > built-in defaults. The project `.env` must not store API Keys, Tokens, Secrets, or Passwords.

## Run

For an existing final `transcription.srt`:

```bash
am run
```

For complete production from an article or spoken script:

```bash
am run PROMPT-PRODUCTION.md
```

`am run` prepends the directory of the current `am` executable to the orchestrator process's `PATH` and exposes its absolute path as `AM_EXECUTABLE`. Commands such as `am scene ...` in the prompt therefore resolve to the same CLI that started the workflow, without copying the binary into each project.

Scene operations are available directly:

```bash
am scene run scenes/scene-001
am scene run-all scenes/ --jobs 3 --retries 2 --report-json production/run-report.json
```

Existing outputs are skipped; inputs newer than the output mark it as stale — use `--force` to re-render. Interruptions stop new tasks, terminate in-flight process groups, and write a partial report.

AI CLIs run in project-scoped safe mode by default. `am --unsafe run` explicitly restores bypass/auto-approval behavior and should only be used for trusted projects. Extra environment variables in safe mode must be named in `AM_PASSTHROUGH_ENV`.

## Inspect frames and concatenate

A successful render is not a correct picture: text can be unreadable against the artwork, CJK glyphs can silently fall back to boxes, an animation can freeze halfway — all of that still exits 0. Extracting frames and looking at them is the only way to catch it early.

```bash
# Extract frames for visual review; --at takes seconds, percentages, and end
am scene frames scenes/scene-001 --at 0,50%,end
am scene frames scenes/scene-001 --at 0 --check-blank   # a blank cover frame fails

# Concatenate scenes in order into a silent master, stream-copied when specs match
am concat scenes/ --out production/silent-master.mp4 --dry-run
am concat scenes/ --out production/silent-master.mp4
```

`am concat` deliberately offers no retiming. A resolution or frame-rate mismatch fails and asks you to re-render the scene rather than scaling or resampling — the former loses quality, the latter moves the timeline.

## Validate, archive, and develop

```bash
am validate publish publish.md --project-root .
am validate style --project-root .
am validate style --project-root . --regenerate-examples  # requires rsvg-convert and magick

# Machine acceptance: canvas, frame rate, codec, pixel format, frame count, audio
# — all checked at once, with every mismatch reported in a single pass
am validate video production/silent-master.mp4 --silent --decode
am validate video final.mp4 --audio --expect-frames 8340 --check-frame-zero \
  --report-json production/final-check.json

am archive --dry-run
am archive
```

`--expect-frames` and `--decode` decode the whole file, so they cost time proportional to its length and are off by default. Read the file written by `--report-json` to decide what happened; do not parse the terminal output.

## Development

```bash
go build ./cmd/am
go test ./...
go test -race ./...
go vet ./...
```

ArticleToMotion is licensed under the [Apache License 2.0](./LICENSE). HyperFrames is an independent Apache-2.0 project installed at its pinned release by `am init`.

The `references/` of the built-in `text-to-lottie` skill are trimmed from [diffusionstudio/lottie](https://github.com/diffusionstudio/lottie) (MIT). The copyright notice and the record of what was kept, dropped, and changed live in `LICENSE` and `ATTRIBUTION.md` under `assets/shared/.agents/skills/text-to-lottie/`.
