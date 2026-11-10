# multieditor

multieditor is a small CLI that opens a temporary local web page for editing a
group of structured records, such as Jira issues or database rows. It is built
for a hand-off between an AI agent and a person: the agent writes the records
to YAML, the person reviews and edits them as cards in a browser, and the agent
reads back a YAML result and applies it to the source system.

multieditor never talks to Jira, a database, or any other service. It reads one
YAML file, serves a page on `127.0.0.1` until the person saves or cancels, then
writes one YAML file and exits.

## How it works

1. The agent writes an input YAML file with a schema and the entities.
2. The agent starts `multieditor` in the background and gives the user the URL
   it prints.
3. The user edits the cards in the browser and selects **Save** or **Cancel**.
4. multieditor writes the output file, prints the final status, and exits.
5. The agent reads the output and applies the `changes` to the source system.

## Quick start

You need Go 1.25 or newer.

```sh
git clone git@github.com:bettertomorrow-dev/multieditor.git
cd multieditor
go build -o bin/multieditor .

bin/multieditor --input examples/jira.yaml --output /tmp/out.yaml --open
```

The command prints the editor URL and keeps running until you save or cancel:

```text
multieditor: url=http://127.0.0.1:54321/s/<token>/
multieditor: status=saved output=/tmp/out.yaml
```

The generated templ code and the compiled CSS are committed, so a plain
`go build` works without any other tool. See [Development](#development) for
changing the UI.

## Command line

```text
multieditor --input in.yaml --output out.yaml [--port 0] [--open]
```

| Flag       | Meaning                                                              |
| ---------- | -------------------------------------------------------------------- |
| `--input`  | Required. YAML file with the schema and entities.                    |
| `--output` | Required. YAML file written on Save or Cancel.                       |
| `--port`   | Port on `127.0.0.1`. The default `0` picks a free port.              |
| `--open`   | Open the URL in the default browser.                                 |

multieditor writes these lines to standard output:

| Line                                       | When                         |
| ------------------------------------------ | ---------------------------- |
| `multieditor: url=<url>`                   | The editor is ready.         |
| `multieditor: status=<saved\|cancelled> output=<path>` | The session ended. |
| `multieditor: error=<message>`             | Something failed.            |

Exit codes: `0` after a save, `2` after a cancel (including Ctrl+C), `1` for an
input, server, or output error.

## Input

```yaml
schema:
  id_field: key          # names the field that identifies each entity
  title_field: card_title  # optional: entity key shown as the card title
  fields:
    - name: key
      label: Ticket
      type: text
      readonly: true
    - name: summary
      type: text
      required: true
    - name: status
      type: enum
      options: [To Do, In Progress, Done]
    - name: labels
      type: multienum
      options: [backend, ui, bug]
entities:
  - key: PROJ-1
    card_title: PROJ-1 · Fix login
    summary: Fix login
    status: To Do
    labels: [bug]
```

Field types are `text`, `longtext`, `number`, `bool`, `enum`, `multienum`, and
`date`. Any field can be `required` or `readonly`. `enum` and `multienum` need
`options`; other types must not have them. Entity keys that the schema does not
declare are not shown, but they are kept unchanged in the output.

## Output

```yaml
status: saved
saved_at: 2026-10-13T08:15:30Z
changes:
  - entity_id: PROJ-1
    field: status
    from: To Do
    to: In Progress
entities:
  - key: PROJ-1
    summary: Fix login
    status: In Progress
    labels: [bug]
```

A cancelled session writes only `status: cancelled`, and the caller must not
apply anything from it. The file is written atomically.

## What the editor does

- One card per entity. The eye button at the top left collapses and expands it,
  and a collapsed card still shows how many fields changed and whether any has
  an error.
- The ellipsis menu at the top right holds card actions. It has **Reset** today.
- Changed fields are marked, and Save stays disabled while any field is invalid.
- The server rejects edits to read-only fields and values that break the schema,
  even if a client sends them directly.
- The look follows shadcn/ui's neutral theme, in light or dark to match the
  operating system.

The full requirements are in [docs/PRD.md](docs/PRD.md).

## Use it from an agent

[`skills/multieditor`](skills/multieditor/SKILL.md) is an agent skill that
explains how to write the input, start the tool, wait for the user, and read the
result. It is linked into `.claude/skills`, `.agents/skills`, and
`.codex/skills`, so Claude Code, Codex, and other agents pick it up inside this
repository.

To run the latest source from any directory, link the dev wrapper onto your
`PATH`:

```sh
ln -s "$PWD/scripts/multieditor-dev" ~/.local/bin/multieditor
```

The wrapper regenerates the UI, rebuilds, and then runs the binary with your
arguments.

## Development

Stack: Go, [templ](https://templ.guide), HTMX, and Tailwind CSS v4 through the
standalone CLI, so no Node runtime is needed. Templates and static files are
embedded into the binary with `go:embed`.

| Command         | What it does                                                        |
| --------------- | ------------------------------------------------------------------- |
| `make tools`    | Download the standalone Tailwind executable into `bin/`.            |
| `make vendor`   | Download the pinned HTMX build and the Tabler icons.                |
| `make generate` | Run templ and compile the CSS.                                      |
| `make fmt`      | Format Go and templ files.                                          |
| `make test`     | Run the Go tests.                                                   |
| `make build`    | Build `bin/multieditor`.                                            |
| `make run`      | Start the example editor, writing to `/tmp/multieditor-out.yaml`.   |

After you change a `.templ` file or `internal/ui/input.css`, run
`make generate` and commit the regenerated files.

Layout:

```text
main.go              CLI entry point and process lifecycle
internal/schema      Input parsing and validation
internal/session     In-memory edit state and change tracking
internal/output      Atomic YAML output writer
internal/server      HTTP and HTMX handlers, token check, asset allowlist
internal/ui          templ components, Tailwind input, embedded static files
skills/multieditor   Agent skill
examples/            Sample input files
docs/PRD.md          Product requirements
AGENTS.md            Instructions for coding agents working on this repo
```

### Theme and icons

The colour tokens in `internal/ui/input.css` come from shadcn/ui's neutral
theme. Components use semantic classes such as `bg-background` and
`text-muted-foreground`, never fixed palette colours.

Icons are [Tabler Icons](https://tabler.io/icons), vendored as `.svg` files in
`internal/ui/static/icons/` and drawn as CSS masks by the `Icon` component. To
add one, put its name in `TABLER_ICONS` in the `Makefile`, run `make vendor`,
and use `@Icon(data, "<name>")` in a template.

## Scope

This is a proof of concept. It edits existing entities only, with no create or
delete, no authentication or multi-user editing, no JSON or CSV input, and no
validation beyond required values, types, and enum options. State lives in
memory and ends with the process.

## License

multieditor is released under the MIT License; see [LICENSE](LICENSE). The
vendored Tabler icons are also MIT licensed; see
`internal/ui/static/icons/LICENSE`. HTMX is vendored as
`internal/ui/static/htmx.min.js`.
