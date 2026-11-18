# Run Twikko with rundevrun

The root `rundevrun.toml` defines `cards` using the task example and `table` using
500 invented orders. Each service gets its own port and binary. Build runs
`make generate`, so install the standalone Tailwind CLI with `make tools` on a
fresh checkout. Generated assets may appear in the Git diff after rebuilding.

From this checkout, with `rundevrun` on PATH:

```sh
rundevrun config check --json
rundevrun register . --json
rundevrun up cards table --dry-run --json
rundevrun up cards table --no-proxy --json
rundevrun logs cards --tail 20
rundevrun logs table --tail 20
```

Open the full `twikko: url=http://127.0.0.1:.../s/.../` link from the logs.
Twikko requires the random session token in this path. `rundevrun open` supplies only
the service root, which returns 404. Readiness uses TCP; opening the full session
URL verifies the editor. Use direct localhost access because Twikko validates
the Host header for writes.

Results are written to `.rdr-output/<mode>.<unique-id>/result.yaml` in this
worktree. The output path is printed when the session starts. Each launch gets a
new directory; runner cleanup leaves results intact. Git ignores this directory.

```sh
rundevrun status --json
rundevrun restart table --no-proxy --json
rundevrun down cards table --json
```

Restart rebuilds the editor. There is no source watcher. Stopping or restarting
an unfinished session cancels it; save edits before restarting if you need them.
Save and Cancel both end the Twikko process. The current runner treats service
exit as `failed`, even after a successful save. Check the final Twikko log and
result YAML, then use `rundevrun down` to clear the stopped run before another `up`.

## Installed runner

Use the normal installed `rundevrun`, available at `~/.local/bin/rundevrun` on this machine.
It uses the standard runtime state at `~/.local/state/rundevrun`; no shell toggle
or project-specific state override is needed. Confirm the selected executable
with `command -v rundevrun` and `rundevrun version --json`.

The local installation is maintained from the rundevrun checkout:

```sh
cd ~/dev/rundevrun
rtk proxy ./scripts/rdr-local update
```

Update builds the current checkout, including uncommitted edits. Use
`update --committed` to build only its committed HEAD. Local hooks update the
installation from committed `main` after merges and rebase pulls when enabled.
To restore the saved previous binary:

```sh
rtk proxy ./scripts/rdr-local rollback
rtk proxy rundevrun version --json
```

Update and rollback preserve runtime state and registrations. Existing
supervisors keep their running code. To use a replacement supervisor, stop and
start the affected services with the same project and worktree selectors.

Existing worktrees need `rundevrun.toml` and the launch script before they can run
these services. If other worktrees lack `rundevrun.toml`, `register` reports `partial`
with exit code 5 while registering valid worktrees. Inspect its results before
continuing from the configured checkout.
