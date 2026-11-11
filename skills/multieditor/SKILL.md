---
name: multieditor
description: "Hand a group of structured records (Jira issues, database rows, config entries) to the user for review and bulk editing in a local browser form, then apply what they saved. Use when several records with multiple fields need the user's edits or sign-off and chat is a poor place for it, or when the user asks to edit a set of tickets or rows in a UI. Do NOT use for a single record, a yes/no confirmation, or adding or deleting records."
---

# multieditor

`multieditor` is a local CLI. You write records and a field schema to YAML and start it. It serves a temporary web form on `127.0.0.1`, one card per record. The user edits and presses Save or Cancel. It writes a result YAML, prints the status and exits. You then apply the saved changes to the source system. The tool never touches the source system itself.

Use it when you hold several records with many fields and the user must compare values, edit long text, or choose from fixed options. Chat makes that slow and error-prone.

It edits existing records only. It cannot add or delete a record, and it has no JSON or CSV input.

If `multieditor` is not on `PATH`, stop and tell the user. Do not build it yourself.

## Procedure

1. **Write the input YAML** into a fresh directory from `mktemp -d`. The output path goes in the same directory. A reused path leaves an old result that looks like the new one. Read `assets/example-input.yaml` first.
2. **Start it** as a background process: `multieditor --input <dir>/in.yaml --output <dir>/out.yaml`. It prints `multieditor: url=<url>` on stdout. Add `--open` only when the user asks for the browser to be opened.
3. **Give the user the URL** and tell them to press Save or Cancel when done.
4. **Wait up to five minutes** for the process to exit. If it is still running, stop your loop and tell the user to say `continue` after they finish. Leave the process running: killing it destroys their edits.
5. **On `continue`**, check whether the process exited and whether the output file exists, then go to step 6. If it is still running, say so and repeat the URL.
6. **Read the result** and act on its status.

## Writing the input

The file has `schema` and `entities`. `schema.id_field` names the field that identifies each record. `schema.fields` lists the fields to show, each with `name`, optional `label`, `type`, optional `required` and `readonly`.

- `type` is one of `text`, `longtext`, `number`, `bool`, `enum`, `multienum`, `date`, `time`, `datetime`.
- `enum` and `multienum` need `options`. No other type may have them. Take the options from the source system, not from guesses. An entity value outside its options makes the whole load fail.
- `bool` values must be real YAML booleans. `multienum` values must be a list. `date` values are `YYYY-MM-DD`, `time` values `HH:MM`, and `datetime` values `YYYY-MM-DDTHH:MM`, both without a time zone. `number` values must be numeric.
- Mark `readonly: true` on fields the user should see but not change. Show the identifier and any context field that way.
- Omit a field that has no value. Do not write `null`: a number field rejects it at load.
- **Choose the card header.** By default it shows the identifier. Use at most one of these three, because combining them fails the load:
  - `schema.title_field`: the name of an entity key that holds the title text. Keep it out of `schema.fields`: a declared field becomes an input, and the user could edit the title. Combine a short identifier with the title (`PROJ-1 · Fix login`); use a short descriptive title alone for a long or opaque identifier such as a UUID. It is fixed text and does not follow edits. An entity without the key shows its identifier.
  - `schema.header`, a list of widgets drawn side by side between the eye toggle and the menu (see Widgets below). A display widget's `source` is an entity key, or a dotted path such as `reporter.name`, holding a scalar. An entity with no value there skips the widget. `icon` and `tag` take `variants`, which map a value (matched exactly) to `{icon, intent}`. `intent` is `neutral`, `info`, `success`, `warning` or `danger`; only an icon variant takes `icon`. An icon whose value matches no variant draws a neutral diamond. `text` takes `style` with `bold`, `italic` and `muted`. Icon names are vendored Tabler icons, and an unknown name fails the load with the list of valid ones. Widgets read the current values, so one bound to an editable field follows the user's edits.
  - `schema.header: { template: <name> }`, a built-in widget list shipped with `multieditor`. Use `gojira` for tickets pulled by the gojira skill. An unknown name fails the load with the list of available ones. A template cannot be extended: write the widgets out to change one.
- **Choose the card body.** By default every field shows in a two-column grid. `schema.body` is a list of rows, each a list of one to four widgets (at most ten rows) that share the row width equally. Use it when the data needs a shape: a long text on its own full-width row, related fields side by side. `schema.body: { template: <name> }` selects a built-in layout, such as `gojira`. Do not write rows by hand when a template fits.
  - **A field no control places is hidden** once a body is set, but it is kept in the output. The load fails if a field has two controls, a control names an undeclared field, a required editable field is left out, or a limit is broken.
- **Widgets.** Header and body take the same widgets, `{type, source, label, options, variants, style}`. A widget either shows a value or edits a field:
  - **Lines** (`vline`, `hline`) draw a 1px separator and take no `source`, only an optional `intent`. A `vline` sits between two widgets of a row or the header and takes only its own width. An `hline` is a row of its own in the body. Lines do not count toward the row and column limits.
  - **Display widgets** (`icon`, `tag`, `text`) are read-only, with a `source` that is an entity key or dotted path. In the body, `label` is a caption above the value. They follow edits made elsewhere on the card.
  - **Controls** edit the schema field `source` names. Leave `type` out (it means `control`) and the field's type picks the control: `text` gets an input, `longtext` a textarea, `number` a number box, `bool` an on/off switch, `enum` a select, `multienum` checkboxes, `date`/`time`/`datetime` their pickers.
  - **Override the control** only for a tailored case, by naming it as `type`: `textarea` or `select` for a `text` field, `input` for a `longtext` field. `select` over a `text` field needs `options` on the widget, and the field then accepts only those values. A control that does not fit the field fails the load and the error lists the ones that do. Do not use `options` anywhere else.
  - `label` replaces the field label. A `toggle` shows its label to the right of the switch, and no label means no text. A control in the header shows no label and keeps it as a tooltip, so put a control there only when its meaning is obvious, such as a status select next to a status tag.
  - A control in the header can be edited while the card is collapsed.
- Keep entity keys that are not in the schema. They are not shown, and they come back unchanged in the output. Use them to carry what you need to apply the result, such as an internal id, a URL or a version. A title key and header sources such as `type` are often of this kind.

## Reading the result

Stdout ends with `multieditor: status=<saved|cancelled> output=<path>`. The exit code is 0 for saved, 2 for cancelled, and 1 for an error.

- **Saved (exit 0).** `changes` lists one entry per changed field: `entity_id`, `field`, `from`, `to`. `entities` holds every record in its final form. The user pressing Save is their approval of those changes, so apply `changes` to the source system and nothing beyond them. A cleared optional field comes back as `""` or `[]`: map that to the source system's way of unsetting a value. If `changes` is empty, the user saved without editing anything: apply nothing.
- **Cancelled (exit 2).** The output holds only `status: cancelled`. Apply nothing and tell the user.
- **Error (exit 1).** Stdout has `multieditor: error=<message>`. If the message names the input, fix the input and start again. Otherwise report it to the user.
