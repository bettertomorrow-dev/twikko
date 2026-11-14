# Run Twikko with rdr

The root `dev.toml` defines `cards` using the task example and `table` using
500 invented orders. Each service gets its own port and binary. Build runs
`make generate`, so install the standalone Tailwind CLI with `make tools` on a
fresh checkout. Generated assets may appear in the Git diff after rebuilding.

From this checkout, with `rdr` on PATH:

```sh
rdr config check --json
rdr register . --json
rdr up cards table --dry-run --json
rdr up cards table --no-proxy --json
rdr logs cards --tail 20
rdr logs table --tail 20
```

Open the full `twikko: url=http://127.0.0.1:.../s/.../` link from the logs.
Twikko requires the random session token in this path. `rdr open` supplies only
the service root, which returns 404. Readiness uses TCP; opening the full session
URL verifies the editor. Use direct localhost access because Twikko validates
the Host header for writes.

Results are written to `.rdr-output/<mode>.<unique-id>/result.yaml` in this
worktree. The output path is printed when the session starts. Each launch gets a
new directory; runner cleanup leaves results intact. Git ignores this directory.

```sh
rdr status --json
rdr restart table --no-proxy --json
rdr down cards table --json
```

Restart rebuilds the editor. There is no source watcher. Stopping or restarting
an unfinished session cancels it; save edits before restarting if you need them.
Save and Cancel both end the Twikko process. The current runner treats service
exit as `failed`, even after a successful save. Check the final Twikko log and
result YAML, then use `rdr down` to clear the stopped run before another `up`.

## Developing rdr itself

If the machine-local development toggles are installed, enable them in the same
terminal before the commands above:

```zsh
source ~/.zshrc.local
rdr-dev-on
```

This selects the development binary and its separate project registry under
`RDR_HOME`. Register Twikko in that environment. `rdr-dev-off` restores the
previous environment; it does not stop services. Run `rdr down cards table`
before switching back when you want these sessions stopped.

The config also works with the installed rdr. Each environment needs its own
registration. Existing worktrees need this config and script in their checkout
before they can run these services. If other worktrees lack `dev.toml`,
`register` reports `partial` with exit code 5 while registering valid worktrees.
Inspect its results before continuing from the configured checkout.
