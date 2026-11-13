---
name: twikko
description: "Hand a group of structured records (Jira issues, database rows, config entries) to the user for review and bulk editing in a local browser form, then apply what they saved. Use when several records with multiple fields need the user's edits or sign-off and chat is a poor place for it, or when the user asks to edit a set of tickets or rows in a UI. Do NOT use for a single record, a yes/no confirmation, or adding or deleting records."
---

# Twikko

Twikko is a local CLI for editing records in a browser. Prepare a reusable
YAML view and a separate YAML, JSON, or CSV data file. The user edits the
records in cards or a table. Twikko writes a YAML result; it never updates the
source system itself.

Use it when several records have enough fields that the user needs to compare
them or edit them outside chat. It cannot add or delete records.

If `twikko` is not on `PATH`, stop and tell the user. Do not build it yourself.

## Procedure

1. Create a fresh directory with `mktemp -d`. Write `view.yaml` and a data
   file there. Choose `data.yaml`, `data.json`, or `data.csv`; read
   `assets/example-input.yaml` for the legacy format only. Put `out.yaml` in
   the same directory so an old result cannot be mistaken for this run.
2. Start Twikko in the background:

   ```sh
   twikko --view <dir>/view.yaml --data <dir>/data.yaml --output <dir>/out.yaml
   ```

   Use the matching data extension. Add `--open` only if the user asks for
   the browser to open automatically. The process prints `twikko: url=<url>`.
3. Give the user the URL and ask them to save or cancel when finished.
4. Wait up to five minutes for the process to exit. If it is still running,
   leave it running and tell the user to say `continue` when finished.
5. On `continue`, check process status and whether `out.yaml` exists. If the
   process is still running, report that and repeat the URL. Otherwise read
   the result.

## Writing the view and data

The view is a complete YAML document with `version: 1`, a `schema`, and
optional `view` settings. Keep it separate from the records so the same view
can load YAML, JSON, or CSV. See `assets/example-input.yaml` only for field
type syntax; it is a legacy single-file input. Examples in the repository use
invented data.

Declare fields with `name`, `type`, optional `label`, `required`, and
`readonly`. Types are `text`, `longtext`, `number`, `bool`, `enum`,
`multienum`, `date`, `time`, and `datetime`. Enum and multienum need `options`.
Use actual booleans and arrays in YAML/JSON. Dates are `YYYY-MM-DD`, times are
`HH:MM`, and datetimes are `YYYY-MM-DDTHH:MM` without a time zone. Preserve
values from the source; never invent a missing business value. Mark values the
user cannot change as read-only.

YAML and JSON files contain a top-level list of objects. CSV's first row has
unique, non-empty field names. The delimiter defaults to comma and can be set
at `import.csv.delimiter` in the view. CSV multienum cells contain a JSON
array, for example `["docs","ui"]`. Unknown fields and nested values stay in
the output.

If `schema.id_field` is declared, it must be a text or number field with
non-empty, unique values. IDs cannot be edited. Without an ID field, records
are still addressable in the result by their zero-based source position.

## Reading the result

Read stdout for `twikko: status=<saved|cancelled> output=<path>`. Exit code 0
means saved, 2 means cancelled, and 1 means an error.

- **Saved (exit 0):** `entities` contains every record in final form and
  `changes` contains changed fields. Apply only those changes to the source.
  With an ID, each change has `entity_id` as the string form of the original
  ID. Without an ID, it has `record_index`, zero-based including `0`, and no
  `entity_id`. Do not treat that index as a user ID. `field` names the field.
  `to` is always present. `from` is omitted only when the original key was
  missing; `from: null` means the key existed with null. A cleared text value
  is `""`, nullable scalar values are `null`, and a cleared multienum is `[]`.
  Map these to the source system's corresponding empty state. If `changes` is
  empty, the user saved without edits, so apply nothing.
- **Cancelled (exit 2):** `status` is `cancelled`. Apply nothing and tell the
  user.
- **Error (exit 1):** report the error. Correct an input error before starting
  another session.

Untouched missing, null, empty string, and empty array remain distinct.
Required rejects all four but accepts `false` and `0`. Text whitespace is
preserved. Numbers remain exact decimal values; do not convert them through a
binary float when applying changes. JSON numeric input is likewise read
without floating-point conversion.

## Legacy transition

The older `twikko --input <path> --output <path>` mode remains supported for
single-file YAML. Its old `schema.header` and `schema.body` settings, including
built-in template references, remain available there. Do not mix `--input`
with `--view` or `--data`. New sessions should use the two-file contract.


The view is fixed for a session. Set `view.mode` in the YAML or pass
`--mode cards` / `--mode table` at launch. There is no browser view switch.
Any invalid editable value must be reachable in that selected layout.
