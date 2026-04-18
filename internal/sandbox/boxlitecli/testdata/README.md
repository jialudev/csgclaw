# BoxLite CLI spike fixtures

These fixtures capture the first implementation step from
`docs/boxlite-cli-sandbox.md`: run the real `boxlite` binary and preserve the
observable stdout, stderr, exit code, and inspect JSON shape that the future
`boxlite-cli` sandbox provider must handle.

Captured with:

- boxlite: `0.7.5`
- os: `macos`
- arch: `aarch64`
- date: `2026-04-18`
- home: `/tmp/csgclaw-boxlite-cli-spike-*`

Confirmed behavior:

- `boxlite create --name csgclaw-spike alpine` exits `0` and writes the box ID
  only to stdout.
- `boxlite inspect --format json csgclaw-spike` exits `0` for a configured box
  and writes a JSON array of box objects, not a single object.
- `inspect` for a missing box exits `1`, writes `[]` to stdout, and writes
  `Error: no such box: <name>` to stderr.
- `exec` for a missing box exits `1` and writes `Error: No such box: <name>` to
  stderr.
- `rm -f` for an existing configured box exits `0` and writes the box name to
  stdout.
- A process terminated by SIGTERM exits `143` in the shell-level spike.
- Rerunning `start` with `https_proxy=http://127.0.0.1:7890` progressed past
  the previous `alpine` config failure, but failed while pulling BoxLite's
  bootstrap `debian:bookworm-slim` manifest with `Not authorized`.
- Rerunning the same proxy-enabled flow after `docker login docker.io` still
  failed on the same `debian:bookworm-slim` manifest authorization error, so
  Docker CLI login alone did not unblock BoxLite's bootstrap image pull in this
  environment.
- A later manual rerun of `boxlite --home "$home" start csgclaw-spike` succeeded
  after Docker Hub retries. The provided terminal output was combined
  stdout/stderr, so it is stored separately as a manual combined log rather than
  as stream-split runner output.
- A subsequent stream-split rerun succeeded for both non-detached and detached
  boxes. The earlier Docker Hub authorization errors should be treated as
  transient failure fixtures, not as the normal expected behavior.
- `create --detach` is the important provider path: it leaves the box running
  after `start`, which allows `inspect` to report `running` and `exec` to attach
  to the existing VM.
- Non-detached `start` succeeds but then auto-stops the box after the image
  command exits. A later `exec` against that stopped box can implicitly restart
  it for the command and then shut it down again.

Open spike gaps:

- Full start stderr is intentionally not copied into individual fixtures because
  it is large and mostly image-pull progress. The important stream-split stdout,
  inspect JSON, command stdout/stderr, and exit codes are captured.
- The `list` command printed only table headers for a configured box in this
  run; the provider should rely on `inspect --format json` rather than table
  output.
