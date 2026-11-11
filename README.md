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
title: Sprint triage                         # optional: page heading and tab title
description: Open bugs of the current sprint. # optional: text under the heading
schema:
  id_field: key          # names the field that identifies each entity
  header:                # optional: widgets drawn in the card header
    - { type: tag, source: key }
    - { type: text, source: summary, style: { bold: true } }
  body:                  # optional: rows of up to four widgets
    - - { source: status }               # no type: a control, picked from the field type
      - { source: labels }
    - - { type: textarea, source: summary }   # override: show a text field as a textarea
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
    summary: Fix login
    status: To Do
    labels: [bug]
```

Field types are `text`, `longtext`, `number`, `bool`, `enum`, `multienum`,
`date`, `time` (`14:30`), and `datetime` (`2026-10-20T14:30`, no zone). Any field
can be `required` or `readonly`. `enum` and `multienum` need
`options`; other types must not have them. Entity keys that the schema does not
declare are not shown, but they are kept unchanged in the output.

### Widgets

The card header and the card body are built from the same widgets. A widget has
a `type`, a `source`, and optionally a `label`, `options`, `variants`, and
`style`, and a line takes `intent`. It either shows a value, edits a field, or
draws a line.

Display widgets show a value, read-only. `source` is an entity key or a dotted
path to a scalar, and may point at keys the schema does not declare.

- `icon`: a vendored Tabler icon. `variants` map a value to an icon and an
  `intent` (`neutral`, `info`, `success`, `warning`, `danger`). A value with no
  variant gets a neutral diamond.
- `tag`: the value in a pill. `variants` map a value to an `intent`.
- `text`: the value as text, with `style` `bold`, `italic`, and `muted`.

Lines separate widgets. They show no value and take no `source`:

- `vline`: a vertical 1px line. In a body row it takes only its own width, and
  its height is the height of the row. In the header it is as tall as the
  header's line. It must sit between two widgets.
- `hline`: a horizontal 1px line. It is a row of its own in the body, with no
  other widget beside it, and it is not allowed in the header.

A line takes an optional `intent` (`neutral` by default, `info`, `success`,
`warning`, `danger`) that sets its colour.

Controls edit a field. `source` is the name of a field declared in
`schema.fields`. `type: control` picks the control from the field's type. Leaving
`type` out means `control`. Naming a control type instead overrides the pick:

| Field type | `control` draws | Other types it accepts |
| --- | --- | --- |
| `text` | `input` | `textarea`, `select` (needs `options`) |
| `longtext` | `textarea` | `input` |
| `number` | `number` | |
| `bool` | `toggle` | |
| `enum` | `select` | |
| `multienum` | `multiselect` (checkboxes) | |
| `date`, `time`, `datetime` | `date`, `time`, `datetime` | |

An override only changes how the same kind of value is drawn. `select` over a
`text` field takes `options` on the widget, and then the field accepts only
those values, including in a request sent straight to the server.

`label` replaces the field label above a control. A `toggle` shows it to the
right of the switch, and it is optional. In the header a control shows no label
and uses it as a tooltip, except a toggle. A display widget shows `label` as a
caption in the body and as a muted prefix in the header.

A field has one control in the header and body together. Display widgets can
repeat a source. Widgets that show values follow edits, and the control being
typed in is never redrawn.

### Card header

By default a card header shows the entity identifier. `schema.title_field`
names an entity key to show instead. `schema.header` replaces the title with a
list of widgets placed side by side between the eye toggle and the menu. A
display widget without a value is skipped. A control in the header works while
the card is collapsed. A field the header edits is left out of the default body
grid.

`header: { template: gojira }` selects a built-in widget list instead of
writing one. It is the header for tickets pulled by the `gojira` skill. An
unknown template name or icon name fails the load and lists the valid ones.
`title_field` and `header` cannot be used together.

### Card body

By default the body shows every field in a two-column grid. `schema.body`
shapes it as rows of one to four widgets, up to ten rows. A row's widgets share
its width equally, and every row stacks to one column on a narrow screen. Lines
do not count in those limits, and a vline does not take an equal share: the
row's columns are equal, with a one-pixel gap for each vline. A row with vlines
stays stacked, without them, until `md` (`lg` for four widgets). A
display widget with no value leaves an empty slot, so columns keep their place
across cards, and a text widget wraps long values. **A field that no control in
the header or body places is hidden** once a body is set, but it stays unchanged
in the output. The load fails when a field has two controls, a control names an
undeclared field or does not fit it, a limit is broken, or a required editable
field would be hidden.

`body: { template: gojira }` selects a built-in layout, as for the header.

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
result. [`skills/gojira`](skills/gojira/SKILL.md) pulls Jira tickets through the
Atlassian MCP and opens them in multieditor. It runs only when you invoke it by
name. Both are linked into `.claude/skills`, `.agents/skills`, and
`.codex/skills`, so Claude Code, Codex, and other agents pick them up inside
this repository.

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
internal/schema      Input parsing and validation, header widgets, body layout, templates
internal/icons       Vendored Tabler icons, the list of valid icon names
internal/session     In-memory edit state and change tracking
internal/output      Atomic YAML output writer
internal/server      HTTP and HTMX handlers, token check, asset allowlist
internal/ui          templ components, Tailwind input, embedded static files
skills/multieditor   Agent skill for running the tool
skills/gojira        Agent skill that pulls Jira tickets into multieditor
examples/            Sample input files
docs/PRD.md          Product requirements
AGENTS.md            Instructions for coding agents working on this repo
```

### Theme and icons

The colour tokens in `internal/ui/input.css` come from shadcn/ui's neutral
theme. Components use semantic classes such as `bg-background` and
`text-muted-foreground`, never fixed palette colours.

Icons are [Tabler Icons](https://tabler.io/icons), vendored as `.svg` files in
`internal/icons/svg/` and drawn as CSS masks by the `Icon` component. To add
one, put its name in `TABLER_ICONS` in the `Makefile`, run `make vendor`, and
use `@Icon(data, "<name>")` in a template or the name in a header widget.

## Scope

This is a proof of concept. It edits existing entities only, with no create or
delete, no authentication or multi-user editing, no JSON or CSV input, and no
validation beyond required values, types, and enum options. State lives in
memory and ends with the process.

## License

multieditor is released under the MIT License; see [LICENSE](LICENSE). The
vendored Tabler icons are also MIT licensed; see
`internal/icons/svg/LICENSE`. HTMX is vendored as
`internal/ui/static/htmx.min.js`.
