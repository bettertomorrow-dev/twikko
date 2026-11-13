# Twikko product requirements

## Summary

Twikko is a temporary local editor for structured records. A caller supplies a
reusable YAML view definition and records in YAML, JSON, or CSV. A user edits
the records in cards or a table. Twikko writes a YAML result for the caller to
apply to the source system. Twikko itself has no source-system integration.

## Goals

- Edit existing records in cards and a table using one session and shared
  validation and change tracking.
- Define fields and layouts in a versioned YAML view file.
- Read YAML, JSON, and CSV records while preserving values that are not being
  edited.
- Support the field types `text`, `longtext`, `number`, `bool`, `enum`,
  `multienum`, `date`, `time`, and `datetime`.
- Keep read-only fields immutable and report field-level validation errors.
- Return complete entities and a per-field change list in YAML.
- Bind only to localhost and keep session state in memory.

## Out of scope

- Creating or deleting records.
- Remote hosting, authentication, or concurrent editing.
- Calls to Jira, databases, or other third-party APIs.
- Editing nested paths, schema inference, sorting, filtering, formulas, bulk
  operations, or range selection.
- Persisting sessions after the process exits.
- Virtualizing the table before measurements show it is needed.

## CLI and lifecycle

New sessions use two input files:

```sh
twikko --view tasks.view.yaml --data tasks.csv --output result.yaml --open
```

`--view` and `--data` are required together. Data extension selects YAML
(`.yaml`/`.yml`), JSON, or CSV; unknown formats fail. `--output` is YAML and
must not alias either input, including detectable symlink or hardlink aliases.
The server binds to `127.0.0.1`. Existing `--port`, `--open`, `--version`,
stdout protocol, and exit codes remain supported. `--mode cards|table` overrides
`view.mode` at launch. No browser control or endpoint changes it afterward.

The legacy `--input` mode remains a separate compatibility route. It accepts
the old single YAML document and its `schema.header` / `schema.body` template
references. It cannot be combined with `--view` or `--data`. Both modes feed
the same editing session. Keep legacy template loaders until that route is
retired.

The caller starts Twikko in the background and gives the printed URL to the
user. It waits up to five minutes. If the process remains open, the caller
leaves it running and resumes only when asked to continue, checking the output
and process first.

## View and data contract

The view is a complete YAML document with `version: 1`, optional `title` and
`description`, a required `schema`, optional `import.csv.delimiter`, and
optional `view` settings. `schema` declares field names, types, labels,
required/read-only flags, and enum options. `view.mode` is `cards` by default
or `table`. `view.cards` defines a card header and body; `view.table.columns`
defines columns. Both layouts are optional. Default table order is schema
order. Default cards use the identifier/title and a two-column grid.
Unsupported versions and unknown keys are errors with a setting path.

YAML and JSON data are top-level lists of objects. Only top-level fields are
editable; nested data is preserved. CSV's first row contains unique, non-empty
column names. Fields bind by name. The delimiter defaults to comma and can be
set in the view file. Standard CSV quoting handles delimiters and line breaks
inside cells. A multienum cell contains a JSON array of strings. Data order is
preserved. Unknown fields and nested structures are retained; undeclared CSV
columns remain strings.

Card and table layouts can each place one editor per field. A field can have
one control in each layout. Options remain a schema-wide constraint. The launch
mode stays fixed for the session; --mode overrides view.mode. Reset clears
drafts. Invalid editable values must be fixable in the selected layout.

## Labels and accessibility

`view.show_labels` defaults to the ordinary label behavior for the location.
An explicit `false` hides visible field captions and column headings.
Widget/column `show_label` overrides the global value. An omitted local value
inherits the global setting. `label` supplies caption text but does not enable
it. Hiding a label leaves values, validation errors, and changed markers in
place. Inputs retain accessible names; table header text remains available to
assistive technology. `title`, `description`, card titles, and separators are
not field labels.

## Value and identity rules

Untouched missing keys, nulls, empty strings, and empty arrays remain distinct.
Clearing text or longtext produces an empty string. Clearing number, date,
time, datetime, enum, or bool produces null. Clearing multienum produces an
empty array. Blank CSV cells use the same type-specific empty value. Nullable
bool has true, false, and null states. Required rejects missing, null, empty
string, and empty array, but accepts false and zero. Text whitespace is
preserved. Missing fields in an edit request are not cleared.

Numbers are exact decimal YAML scalars. JSON decoding uses number tokens rather
than binary floating point. Numeric comparison uses exact rational arithmetic.
Decimal and exponent notation are supported, but fractions such as `1/3`,
NaN, and infinity are rejected. Inputs longer than 1024 characters or with an
absolute exponent over 10000 are rejected before comparison. Numerically equal
edits preserve the original node and change summaries retain exact values.

An optional `schema.id_field` must identify a declared text or number field.
IDs must be non-empty and unique and cannot be edited. Number IDs compare
exactly. With an ID, changes use `entity_id`, the string form of the original
value. Without one, changes use zero-based `record_index` (including index 0)
and omit `entity_id`. A missing original field omits `from`; an original null
is written as `from: null`. `to` is always present. Save always writes
`status`, `entities`, and `changes`, including empty lists. Cancel writes
`status: cancelled`; callers do not apply it.

Structural errors, duplicate keys or IDs, invalid IDs, and invalid read-only
values block startup. An invalid editable value may open only when a layout
provides a way to fix it. Twikko shows the original invalid value and blocks
Save until it is corrected.

## UI

- Keep Save and Cancel visible while scrolling.
- Show field errors and changed markers next to their controls.
- Validate on the server, including read-only enforcement.
- Reuse field controls and session state across cards and table.
- Provide row Reset, sticky table headings, horizontal scrolling, and keyboard
  navigation in table view.
- Use an expanded editor for long text and multienum when an inline cell harms
  table readability.
- Keep active editors intact across partial updates.
- Use a native table with thin horizontal and vertical grid lines and cell
  editors without separate borders or repeated captions.
- Let users resize columns by dragging header boundaries or using the
  keyboard. Width changes survive row Reset and are local to the page.

## Data volume and measurements

The first-version support target is 200 records × 20 fields, at most 2 MiB,
with at most 20 multienum options. These numbers define a measurable target,
not a row truncation limit. Measure browser opening separately from
server response time. Goals are 2 seconds for opening, p95 server
response at or below 300 ms, and 2 seconds for Save. No speed result is claimed
until measured.

## Result format

```yaml
status: saved
saved_at: 2026-11-13T10:00:00Z
changes:
  - entity_id: TASK-201
    field: status
    from: Backlog
    to: In Progress
entities:
  - id: TASK-201
    status: In Progress
```

Every saved result contains all entities, including hidden and undeclared
values. Cancel produces no applicable changes. Output is written atomically by
renaming a temporary file in the destination directory.

## Example files and skills

`examples/tasks.view.yaml` can be reused with the matching YAML, JSON, and CSV
examples. All records are invented. The older `examples/jira.yaml` and
`examples/manual-test.yaml` remain for the legacy `--input` path. The Twikko
and gojira skills document the two-file contract, result handling, and legacy
transition. Gojira's complete reusable view is
`skills/gojira/assets/gojira.view.yaml`.


## Earlier measurements

The earlier version with a browser view switch took between 2.7 and 3.3 seconds
to return to 200 cards with 20 text fields in headless Chromium 158 on Linux.
That switch has been removed. These historical numbers do not measure the
current fixed-view launch or column resizing.
