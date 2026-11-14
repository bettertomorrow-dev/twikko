# Table interactions and row conditions

The table starts in input-file order. Its first column shows a permanent,
one-based number for each source record. Sort this column ascending to return
to file order, or descending to reverse it. Click a data column's sort button
to alternate ascending and descending order. Only one column sorts at a time.

Numbers compare exactly, including decimals beyond JavaScript's safe integer
range. Dates and times compare chronologically, booleans put false before
true, and text compares lexicographically with case preserved. Enum values
follow their schema `options` order. Multienum values compare by those option
positions, independent of the order in which options were selected. Equal
values retain source order. Missing, null, and invalid values come last in
both directions. A display column without a declared field has no sort button.

Sorting happens when requested. Editing or resetting a row does not move it;
request sorting again to reflect the new values. Sorting preserves the
existing editor elements. Row numbers, source identity, result entity order,
and change indices do not depend on the display order.

Drag the right boundary of a heading to resize its column. Double-click that
boundary to fit the heading and cell contents, with a maximum automatic width
to keep long text manageable. Manual resizing can make it wider. A focused
resize handle accepts Left/Right, Shift for larger steps, Home for the minimum
width, and Enter for auto-fit.

Drag the separate move handle to arrange data columns. With the handle
focused, press Space to begin, Left/Right to move, Enter or Space to confirm,
or Escape to restore the previous arrangement. The number and actions columns
stay at the edges. Widths follow their columns. Reset preserves widths and
column order; reloading restores the view's layout.

Hovering a cell strengthens its border without changing the grid dimensions.
Hold Shift to tint that cell's row and column. Their intersection has the same
intensity as the rest of the crosshair. The tint follows the operating-system
theme and clears when the pointer leaves or the window loses focus.

## Conditional row backgrounds

Set `view.table.row_highlights` to an ordered list of conditions. The first
matching rule supplies the row's background intent: `neutral`, `info`,
`success`, `warning`, or `danger`. Conditions affect presentation. They do not
reject records or add validation constraints.

This fragment marks unfinished records whose due date is before today:

```yaml
view:
  mode: table
  table:
    timezone: Europe/Helsinki
    row_highlights:
      - intent: warning
        when:
          all:
            - field: due
              measure:
                distance_from: today
                unit: days
              op: lt
              value: 0
            - field: completed
              op: eq
              value: false
      - intent: success
        when:
          field: completed
          op: eq
          value: true
```

The named fields must be declared in `schema.fields`. A condition is either a
leaf with `field` and `op`, or an `all` or `any` list of conditions. Lists must
be nonempty. Unknown keys, fields, intents, incompatible operators, and invalid
operands are configuration errors with a path to the setting.
Condition trees are limited to depth 8 and 256 nodes in total.

| Field or measure | Operators | Operand |
| --- | --- | --- |
| Number | `gt`, `lt`, `eq`, `ne` | Exact YAML number |
| Number | `even`, `odd` | Omit `value`; fractional values match neither |
| Text or longtext | `eq`, `ne`, `contains`, `not_contains` | String, case-sensitive; whitespace preserved |
| Text with `measure: length` | `gt`, `lt`, `eq`, `ne`, `even`, `odd` | Unicode code-point count, rather than byte count |
| Boolean | `eq`, `ne` | YAML `true` or `false`, not a quoted string |
| Enum | `eq`, `ne` | One declared option |
| Enum or multienum | `contains_all`, `contains_any` | Nonempty list of declared options |
| Temporal distance | `gt`, `lt`, `eq`, `ne` | Signed numeric distance in the selected unit |

An enum is a set containing its single value for `contains_all` and
`contains_any`. Repeated options and option order in a condition do not change
the result. An empty multienum is an empty set. An empty string has length
zero. Missing, null, and invalid values match no leaf condition, including
`ne` and `not_contains`. An invalid draft does not reuse the last valid value
for highlighting.

Examples of string and set conditions:

```yaml
when:
  any:
    - { field: title, measure: length, op: gt, value: 60 }
    - { field: title, op: contains, value: review }
    - { field: labels, op: contains_all, value: [external, urgent] }
```

## Dates and time

Distance means `field value − reference value`. Use `today` for a date,
`now` for time or datetime, or another field of the same temporal type:

```yaml
when:
  field: finished_at
  measure:
    distance_from: { field: started_at }
    unit: hours
  op: gt
  value: 24
```

Dates use `days` and count calendar days. Time and datetime accept `minutes`,
`hours`, or `days`; a datetime day is 24 elapsed hours. Time compares within
one day without wrapping across midnight. For example, `01:00 − 23:00` is
negative 22 hours.

`view.table.timezone` accepts a Go time-zone name such as `UTC` or
`Europe/Helsinki`. When omitted, it uses the local zone of the Twikko process.
Datetime fields have no embedded offset and are interpreted in this zone.
Missing or ambiguous local times during daylight-saving transitions do not
match temporal conditions. Missing, null, or invalid reference fields also
make the condition nonmatching.

`now` uses minute precision. Highlighting updates after edits and Reset, and
relative-time rules refresh each minute. Focus, validation errors, and changed
markers remain visible over row backgrounds and the Shift crosshair.
