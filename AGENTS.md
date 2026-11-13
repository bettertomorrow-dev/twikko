# Twikko

## Purpose

Twikko is a local CLI utility for editing a group of structured entities in
a browser. An entity is a composite object such as a Jira issue, a database row,
or another record with named fields.

The tool is designed for an agentic workflow. An agent prepares a YAML input
file, starts Twikko, then waits for the user to review and change the
entities in a temporary local web UI. Twikko writes the result to YAML
after the user saves or cancels. The agent then applies the saved changes to
the source system.

## Product scope

- Edit existing entities only. Do not add or delete entities.
- Accept a reusable YAML view and separate YAML, JSON, or CSV data.
- Support text, multiline text, number, boolean, enum, multi-select, and date
  fields.
- A field can be read-only regardless of its type.
- Validate required fields, field types, and enum values before saving.
- Choose cards or a table at launch with `view.mode` or the `--mode` override.
  Keep that view fixed during the session.
- Write complete resulting entities and a per-field change summary in YAML.

Out of scope:

- Remote hosting, authentication, or multi-user editing.
- Direct Jira, database, or other third-party integrations.
- Schema inference.
- Validation rules such as ranges, regular expressions, or string lengths.
- Persistence between sessions.

## Agent interaction protocol

1. The agent creates a view YAML file, a YAML/JSON/CSV data file, and chooses
   an output YAML path.
2. The agent launches `twikko --view <view> --data <data> --output <path>` as
   a background process. The legacy `--input <path>` single-YAML mode remains
   supported separately and cannot be combined with the new flags.
3. Twikko prints its localhost URL to stdout. The agent gives that URL to
   the user.
4. The user edits the entities and selects Save or Cancel.
5. Twikko writes the output, reports the final status and output path to
   stdout, then exits.
6. The agent reads the output and applies the changes to the source system.

The agent waits up to five minutes for Twikko to exit. If it does not exit
in that time, the agent stops its current loop and tells the user to say
`continue` after they finish editing. The web server remains available. On a
later `continue`, the agent checks the output file and process status before
resuming its work.

## Technical direction

- Go provides the CLI, schema parser, validation, output writer, and HTTP
  server.
- HTMX handles partial page updates for validation and edit state.
- `templ` provides typed HTML components.
- Tailwind CSS styles the component-based UI. Use the standalone Tailwind CLI,
  not a Node runtime.
- The theme is the shadcn/ui neutral token set in `internal/ui/input.css`,
  with a light and a dark variant that follows the operating-system setting.
  The `changed`, `info`, `success`, and `warning` tokens extend it.
  Components use semantic utilities (`bg-background`, `text-muted-foreground`,
  `border-input`, ...), never palette colours such as `slate-200`.
- Icons are Tabler icons. `make vendor` downloads the pinned `.svg` files into
  `internal/icons/svg/`. Templates draw them with the `Icon` component, which
  sets a CSS mask, so no icon path data lives in a template. To add an icon, add
  its name to `TABLER_ICONS` in the Makefile and run `make vendor`. The
  embedded files are also the list of valid icon names for header widgets.
- Header widget and body row classes live in `internal/ui/data.go` as full
  class strings, so Tailwind finds them. Never build a class name from a YAML
  value.
- Embed generated templates and static assets into one binary with `go:embed`.
- Bind the server to `127.0.0.1` only. It must run only for the editing
  session.
- Write output atomically: write a temporary file in the destination directory,
  then rename it to the requested output path.

## UI conventions

- Show one entity per card. In the new format, the card header shows
  `view.cards.header` widgets. Legacy `--input` uses `schema.header` as before,
  else its `title_field` value, else the identifier field. Widgets sit between
  the eye toggle and the ellipsis menu, left-aligned with an equal gap. A widget
  with no value is skipped. Widgets read the current values, so editing a field
  refreshes the widgets bound to it.
- Header and body hold the same widgets. A display widget shows a value. A
  control edits a field, with the control picked from the field's data type
  unless the widget names another that fits it. A field has one control across
  header and body.
- Lay the card body out in the rows and columns of `view.cards.body` (one to
  four widgets per row, up to ten rows, equal widths, one column on a narrow
  screen), else a two-column grid of every field that no header control edits.
  Fields a body does not place are hidden but kept in the output. Display
  widgets refresh on edits without re-rendering a control.
- Lines are widgets: `vline` and `hline`, 1px, coloured by an optional intent.
  They take no source and do not count in the body's row and column limits. A
  vline takes only its own width in the row grid (a `--cols` track of `auto`)
  and stretches to the row's height, so it never changes the layout. An hline is
  a row of its own. The header takes vlines only.
- Put an eye toggle at the header's top left. An open eye means the card is
  expanded, a crossed eye means it is collapsed. Pressing it flips the state.
- Put card actions, currently only Reset, in an ellipsis dropdown at the right
  of the header.
- Choose an input control from the field type.
- Date, time, and date-time fields are a masked text input (`YYYY-MM-DD`,
  `HH:MM`, `YYYY-MM-DD HH:MM`) with a button that opens a popover from
  `internal/ui/static/app.js` (calendar grid, hour and minute columns), not the
  browser's native picker, which CSS cannot style. The user can type digits or
  pick from the popover. A hidden input carries the value in the native
  input's format (`2026-10-20`, `14:30`, `2026-10-20T14:30`).
- Render read-only values as unavailable for editing.
- Show validation feedback next to the field that caused it.
- Mark changed fields.
- Keep Save and Cancel visible while the user scrolls.
- Tables use thin horizontal and vertical grid lines. Cell editors have no
  separate borders or repeated visible captions. Columns resize by dragging
  header boundaries, with keyboard resizing also supported.

## Example data

Never commit corporate or work-related content as example data. This covers
`examples/`, skill assets, test fixtures, docs, and code comments. Examples
copied from a real Jira, database, or other work system leak employer
information into git history, and a later edit does not remove it from there.

- Do not copy real ticket keys, project names, titles, descriptions, people's
  names or emails, company or product names, hostnames, or cloud IDs.
- When a skill pulls real data to test it, keep that data in a temporary
  directory. Do not paste it into a file in the repository.
- Use invented values: keys like `PROJ-101`, made-up names, `example.com`.
  Invented values must not resemble a real ticket the author works on.
- If a real value is needed to reproduce a bug, replace it with a made-up one of
  the same shape (type, length, characters) before committing.

## Documentation

Product requirements live in [docs/PRD.md](docs/PRD.md).

## Development commands

- `make tools` downloads the standalone Tailwind executable into `bin/`.
- `make vendor` downloads the pinned HTMX browser runtime.
- `make generate` runs `templ` and compiles Tailwind CSS.
- `make fmt` formats Go and templ files.
- `make test` runs the Go test suite.
- `make build` creates `bin/twikko`.
- `make run` starts the CSV example editor and writes its output to
  `/tmp/twikko-out.yaml`.

The first-version support target is 200 records × 20 fields, up to 2 MiB and
20 multienum options. These are support and measurement targets, not truncation
limits. Opening either view (2 s), server response p95 (300 ms), and Save
(2 s) are goals only; do not describe them as measured results until a run has
recorded measurements.

For the two-file value, identity, change-summary, and label semantics, follow
`docs/implementation-plan-data-views.md`'s accepted stage 0 decisions and
`docs/PRD.md`. In particular, no ID means `record_index` starting at zero;
missing `from` differs from `from: null`, and `to` is always present.
