# Song workflow contract (schema 1)

`--delivery song` is independent of narration; speech remains the default. Only
`song.yaml` activates song mode. `cast.yaml` and song mode are mutually exclusive.
All user production state lives outside this CLI repository.

## Configuration and provenance

`song.yaml` accepts only `schema`, `provider`, `model`, `endpoint`, `workspace`, `lyrics`,
`style`, `language`, `target_seconds`, `bpm`, and `python`. Unknown fields are an
error, including credential fields. Providers are `minimax`, `acestep`, `bailian`, `elevenlabs`, `fal`, `mureka`, `lyria`, and `manual`.
Use `am song providers [PLATFORM]` for signup links and key setup, and
`am song configure --provider PLATFORM` to update only platform fields.
Hosted endpoints are fixed; only native ACE-Step accepts `endpoint`.
Bailian requires a Beijing `workspace`. Keys are environment-only and nested calls
require explicit `AM_PASSTHROUGH_ENV`; no global whitelist expansion.
Endpoints cannot include URL credentials or query strings. Lyric paths must be
relative to the project without traversal or symlinks. Target duration is 10–600
seconds and BPM is 30–300; neither overrides measured audio timing.

Candidates live in `production/song/candidates/ID/`. Audio and lyric files are
copied into the candidate; `candidate.json` records SHA-256 digests, provider,
model, measured duration, task ID where available, and request digest. Generated
candidates also retain a credential-free `request.json`, start time and elapsed
execution time. MiniMax's synchronous CLI does not expose an asynchronous task ID.
Raw provider output is not copied into logs because it may contain authentication
or signed download URLs. MiniMax structured failures are redacted and retained in
`provider-error.json`, with the CLI exit code and a bounded diagnostic message.

Generation slots are keyed by configuration, lyrics, batch and index. A persisted
submission marker precedes the network call. A ready slot is reused; a known
ACE-Step, fal or Mureka task resumes polling. An interrupted or failed synchronous MiniMax/Bailian/ElevenLabs/Lyria call, or an
ACE-Step submission without a recorded ID, never resubmits automatically. Use a
new batch only after checking the provider. Downloads remain temporary until
complete and are limited to 512 MiB. Redirects are rejected. ACE-Step downloads must be same-origin. Hosted audio
downloads use allowlisted TLS storage domains and never receive API credentials.
Signed URLs and raw response bodies are never saved. fal outputs use `.wav`;
other platforms request MP3. `production/song/.operation-lock` prevents concurrent candidate,
selection and preparation mutations; after a process crash, confirm no song
command is running before removing a leftover lock.

## Timing and manual corrections

The authoritative `production/song/timeline.json` contains:

- `schema: 1`, `candidate_id`, `audio_sha256`, `lyrics_sha256`,
  `duration_seconds`, `source`, and `reviewed`.
- `lines`: each has unique `id`, `source_line`, `text`, optional `alignment_text`,
  `start` and `end` in global seconds, and optional `words` with `text/start/end`.
- `beats`: strictly increasing global seconds. These are ordinary beats, not
  bar/downbeat annotations.
- `instrumentals`: computed `intro`, `interlude`, and `outro` ranges.
- `warnings`: preserved alignment and beat-analysis warnings.

Automatic alignment is pinned to Xingyu 0.6.1. Raw outputs, an exact original /
display / alignment-input line mapping, an issue report and an editable
`timeline-draft.json` are stored in a distinct alignment-run directory. Repeated
lyric lines are never deduplicated. Null timings and non-aligned statuses block
publication. Estimated or missing word timings are omitted and reported; malformed
remaining word timings also block publication. Sentence timing alone remains usable.

To correct a draft, preserve the selected candidate and digest fields, keep one
line per sung lyric in original order, and adjust timing. Remove unreliable words
rather than inventing exact timestamps. Import it using
`am song prepare --timeline FILE --reviewed` only after reviewing the actual sung
facts, pronunciation and timing. Instrumental ranges are recomputed on import.
Changing the selection or candidate bytes invalidates the timeline. A successful
forced alignment does not establish content accuracy or the ≤200 ms onset target.

The CLI exports `production/song/lyrics.srt`,
`production/song/phrase-timeline.json`, and `production/phrase-timeline.json`.
Phrases carry subtitle indices, semantic order, millisecond times, global frame
ranges, alignment source and review status. Confidence is explicitly null rather
than an invented score; `visible_event_ids` start empty and must be connected
when constructing the existing visible-event inventory.

## Scene and media boundary

`am song cues SCENES` uses the same directory order as `scene run-all`. Every scene
must last an integer number of project frames; total duration is exactly
`ceil(audio_duration * fps) / fps`. It clips intersecting lyric lines and words at
scene boundaries, preserving IDs, and assigns boundary beats to the next scene.
It writes `song-cues.json` and this optional `scene.json` block:

```json
{
  "song": {
    "fps": 30,
    "timeline": "../../production/song/timeline.json",
    "sha256": "<global timeline file digest>",
    "start_seconds": 0,
    "data": "song-cues.json",
    "data_sha256": "<local cues file digest>"
  }
}
```

Rendering checks the current selection, timeline, local content and their digests.
`--song-timeline` and `--srt` are mutually exclusive. The song skill is injected
only for song scenes. Song rendering never generates audio or edits global timing.
After timing changes, recompute cues and explicitly rerender stale outputs with
`--force`. Concat rejects stale song renders; mux also rejects a master older than
its scene outputs. Mux uses the complete selected track, with only sub-frame tail
padding, no retiming, extra BGM or speech ducking. Final media is decoded and checked.

## Verification boundaries

Automated tests exercise mock MiniMax/ACE-Step/Bailian/ElevenLabs/fal/Mureka/Lyria calls, failures, resumable polling,
interrupted downloads, redirect/credential protection, repeated lyrics, partial
alignment, selection invalidation, both canvas presets, stale rendering, and real
ffmpeg tail-padding. They do not establish singing quality or visual acceptance.
Real samples, alignment models and service credentials are maintained outside this
repository. No Python dependencies or model weights are installed by `am init`.

## Hosted API references (checked 2026-09-18)

- [Bailian Fun-Music](https://help.aliyun.com/zh/model-studio/fun-music-api):
  non-streaming fixed lyrics, Beijing invitation and ordinary billing; prompt is
  ignored when lyrics are provided. No target duration/BPM controls are sent.
- [ElevenLabs compose](https://elevenlabs.io/docs/api-reference/music/compose):
  v2/v2.5 composition chunks preserve lyric order and split at line boundaries,
  at most 30 lines / 120 seconds per chunk, 200 characters per line.
- [fal ACE-Step](https://fal.ai/models/fal-ai/ace-step/api) and
  [queue protocol](https://fal.ai/docs/documentation/model-apis/inference/queue):
  persist request ID before polling; resume status/result/download without POST.

Local key presence does not establish model access, credit balance or singing
quality. Configure/providers never perform paid calls. Hosted real-audio acceptance
requires separately provisioned accounts and remains separate from mock tests.

- [Mureka lyrics-to-song](https://platform.mureka.ai/docs/api/operations/post-v1-song-generate.html)
  and [task query](https://platform.mureka.ai/docs/api/operations/get-v1-song-query-%7Btask_id%7D.html):
  default pinned model `mureka-9.5`, `n=1`, separate API billing. Metadata-only polling
  resumes from the persisted ID. Completed results must contain exactly one song;
  unexpected multiple outputs are rejected rather than silently selected. Download
  requests use TLS Mureka subdomains without authorization headers. Provider lyric
  timestamps do not bypass independent alignment and manual review.
- [Google Lyria](https://ai.google.dev/gemini-api/docs/music-generation) and
  [Interactions schema](https://ai.google.dev/api/interactions-api): `lyria-3.5`,
  synchronous POST `/v1beta/interactions` with `x-goog-api-key`, `store=false`.
  Decode one inline MP3 audio block from `model_output` steps. Missing/blocked,
  malformed base64, non-MP3 or multiple audio blocks fail without replacing an
  existing audio file. JSON including base64 is capped at 96 MiB. Text blocks never
  overwrite source lyrics. As with other synchronous providers, a failed attempt
  is not automatically resubmitted, even when a response ID was returned.

## Browser handoff and human wait

`provider: manual` needs no API credentials. `song handoff` (or manual `generate`)
reads the authored lyrics and style, displays both, and stores content-addressed
snapshots under `production/song/handoffs/`. `web-handoff.json` records the snapshot
ID, hashes and `waiting_for_audio` / `imported` status, plus the received candidate.
This command returns normally; the production prompt must pause for the user,
never infer audio exists from exit code zero, poll for files or continue rendering.
The prompt instructs the user to generate on their website and attach audio or
provide a local path; another session can resume via the durable wait record.

`song receive AUDIO` verifies the snapshot and imports under the same operation
lock, binding the original lyrics even if project lyrics changed. Same-audio
receipt reuses the recorded candidate; different audio creates a new candidate.
No selection or timing is performed. Failed receipt leaves the wait record intact.
While waiting, changed lyrics/style require an explicit `handoff --refresh` and
another website generation. Provider-modified lyrics require explicit ordinary
`import --lyrics FINAL_LYRICS` after reviewing the changes. Existing valid user
selection takes precedence on continuation. No webpage credential is requested.
