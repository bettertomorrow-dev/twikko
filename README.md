# Twikko

Twikko opens a temporary local editor for structured records. Give it a YAML
view definition and a separate YAML, JSON, or CSV data file. The user edits the
records in cards or a table, then saves or cancels. Twikko writes a YAML result;
it does not connect to the source system.

## Quick start

```sh
go build -o bin/twikko .
bin/twikko --view examples/tasks.view.yaml --data examples/tasks.csv \
  --output /tmp/tasks-result.yaml --open
```

The command prints a localhost URL and stays open until the user saves or
cancels. Equivalent example data is in `examples/tasks.yaml` and
`examples/tasks.json`.

The legacy `--input` mode remains available for existing single-file YAML
inputs, including their built-in header/body templates. Do not combine it with
`--view` or `--data`.

## Command line

```text
twikko --view view.yaml --data records.csv --output result.yaml [--port 0] [--open]
twikko --input legacy.yaml --output result.yaml [--port 0] [--open]
```

`--view` and `--data` are required together. The data extension selects YAML
(`.yaml`/`.yml`), JSON (`.json`), or CSV (`.csv`). Output is always YAML.
`--output` must not alias either input file. The server binds to `127.0.0.1`.
The existing `--port`, `--open`, and `--version` options remain available.

Twikko prints `twikko: url=<url>` when ready and
`twikko: status=<saved|cancelled> output=<path>` when the session ends. Exit
codes are 0 for save, 2 for cancel (including Ctrl+C), and 1 for errors.

The caller should wait up to five minutes. If the process is still running,
leave it running and ask the user to say `continue` when done. On continuation,
check the process and result file before applying any changes.

## View files

A view file is a complete, versioned YAML document. Its `schema` describes
field types and validation; `view` describes card and table layouts. It can be
reused with each supported data format.

```yaml
version: 1
title: Tasks
description: Invented task records.
schema:
  id_field: id
  fields:
    - { name: id, label: ID, type: text, readonly: true }
    - { name: title, label: Title, type: text, required: true }
    - name: status
      label: Status
      type: enum
      options: [Backlog, In Progress, Done]
view:
  mode: cards
  show_labels: true
  table:
    columns: [{ source: id }, { source: title }, { source: status }]
  cards:
    header: [{ type: tag, source: id }, { source: status }]
    body:
      - [{ source: title }]
```

The mode defaults to `cards` and is fixed for the session. `--mode table` or
`--mode cards` overrides `view.mode` at launch. There is no browser view switch.
Both layouts are optional. A table without
`columns` uses schema field order; cards without a layout use the standard
header and body grid. Unknown keys and unsupported versions are errors. The
legacy format keeps its old `schema.header` and `schema.body` widget syntax,
including built-in templates; the new view format uses complete files and does
not accept template references.

Field types are `text`, `longtext`, `number`, `bool`, `enum`, `multienum`,
`date`, `time`, and `datetime`. Enum types require `options`; other types
forbid them. Fields may be `required` or `readonly`. YAML and JSON data are
lists of objects. CSV uses the first row as unique, non-empty field names;
`import.csv.delimiter` sets its one-character delimiter and defaults to comma.
Quoted cells may contain delimiters and line breaks. Multi-enum CSV cells hold
a JSON array of strings. CSV values are interpreted using the schema, while
undeclared columns remain strings. Unknown fields and nested values are kept.

### Labels

`view.show_labels` controls widget labels and table headings globally.
`show_label` on a card widget or table column overrides it. An omitted setting
keeps the normal label behavior for that location. Explicit `false` hides only
the visible caption; values, errors, and changed markers remain visible, and
controls retain accessible names. Table headings keep visually hidden text.
Explicit `true` shows a caption even in a card header. `label` chooses the
caption text; it does not turn the caption on.

### Values and changes

Untouched missing keys, `null`, empty strings, and empty arrays stay distinct.
Clearing text writes `""`; clearing number, date, time, datetime, enum, or bool
writes `null`; clearing multienum writes `[]`. Blank CSV cells use those same
type-specific empty values. A nullable bool has true, false, and null states.
Required rejects missing, null, empty string, and empty array, but accepts
`false` and `0`. Text whitespace is preserved. A request that omits a field
does not clear it.

Numbers are exact decimal values, including exponent notation. JSON numbers are
read without conversion through binary floating point. Unsupported number
syntax or values beyond the documented parser bounds produce errors instead
of rounding. Numerically equal edits retain the original value.

`schema.id_field`, when present, must name a text or number field whose values
are non-empty and unique. IDs are immutable and checked on the server. A change
entry uses `entity_id` as the string form of that original ID. Without an ID
field, it uses zero-based `record_index` instead and omits `entity_id`.
Missing `from` means the original field was absent; `from: null` means it was
present with null. `to` is always present. A saved result always contains
`entities` and `changes`, including empty lists. A cancelled result has
`status: cancelled` and is not applied.

## Examples and agent skills

### Table editor

Run `make run-table` to open 500 invented orders in a table, or use:

```sh
bin/twikko --view examples/orders.view.yaml --data examples/orders.csv \
  --mode table --output /tmp/twikko-table-out.yaml --open
```

The example includes payment status and type, purchase and delivery prices,
UTC timestamps, account IDs, and regions. All amounts are in USD. Only 5% of
orders have a discount, expressed as a fixed amount. Accounts appear one to seven
times, scattered throughout the table. Edit cells, resize columns
by dragging their heading edges, and select Save to write the complete records
and field changes to `/tmp/twikko-table-out.yaml`.

### Other examples

- `examples/tasks.view.yaml` works with `tasks.yaml`, `tasks.json`, and
  `tasks.csv`. They contain the same invented records.
- `examples/jira.yaml` and `examples/manual-test.yaml` exercise the legacy
  `--input` route.
- [`skills/twikko`](skills/twikko/SKILL.md) describes preparing inputs,
  launching Twikko, waiting, and applying saved changes.
- [`skills/gojira`](skills/gojira/SKILL.md) describes preparing read-only
  issue data and uses its reusable view at
  `skills/gojira/assets/gojira.view.yaml`.

## Development

Go, templ, HTMX, and Tailwind CSS v4 through its standalone CLI. Generated
templates and static assets are embedded in the binary.

| Command | Purpose |
| --- | --- |
| `make tools` | Download the standalone Tailwind executable. |
| `make vendor` | Download HTMX and Tabler icons. |
| `make generate` | Generate templ code and compile CSS. |
| `make fmt` | Format Go and templ files. |
| `make test` | Run Go tests. |
| `make build` | Build `bin/twikko`. |
| `make run` | Open the CSV example and write `/tmp/twikko-out.yaml`. |
| `make run-table` | Open the 500-order CSV example in a table and write `/tmp/twikko-table-out.yaml`. |

The first-version support target is 200 records × 20 fields, up to 2 MiB and
20 multienum options. It is measurable, not a truncation limit. Opening and
opening either view (2 s), server p95 response (300 ms), and Save (2 s) are
measurement goals. These timings have not yet been measured.

See [docs/PRD.md](docs/PRD.md) for product requirements.


The optional browser regression check in `scripts/test-data-views.cjs` needs
an externally installed Playwright package and its Chromium browser. After
`make build`, run `NODE_PATH=/path/to/node_modules node scripts/test-data-views.cjs`.
This checks both launch modes, immediate Save with queued edits, invalid drafts,
focus, Reset, nullable values, labels, read-only controls, table grid lines,
borderless cell editors, and pointer/keyboard column resizing.


In table mode, drag the right edge of a column heading to resize that column.
A focused resize handle also accepts Left/Right, Shift for larger steps, and
Home for the minimum width. Widths stay in the current page and survive row
Reset; reloading restores the defaults. Table cells provide the grid borders,
so their editors have no separate borders or repeated visible captions.
