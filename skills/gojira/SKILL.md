---
name: gojira
description: "Pull Jira tickets through the Atlassian MCP and open them in Twikko for review. Runs only when the user invokes it by name (/gojira or $gojira)."
disable-model-invocation: true
argument-hint: "[search criteria: JQL, or plain words like 'my bugs in PROJ']"
---

# gojira

Fetch Jira issues, write them as data for Twikko, and open them for review.
This skill never updates Jira.

Search criteria from the user: $ARGUMENTS

If that line is empty, the user set none.

## Procedure

1. Check that the Atlassian MCP has `searchJiraIssuesUsingJql` and
   `getAccessibleAtlassianResources`, and is authenticated. If not, stop and
   tell the user. Do not use `curl` or Jira REST as a fallback.
2. Call `getAccessibleAtlassianResources`. Use the single available site. If
   there are several, ask which one to use.
3. Build JQL. Use supplied JQL verbatim. Translate plain-language criteria
   without adding filters. If no criteria were supplied, use
   `assignee = currentUser() AND statusCategory != Done ORDER BY created DESC`.
4. Call `searchJiraIssuesUsingJql` with fields `summary`, `issuetype`,
   `creator`, `created`, `duedate`, `assignee`, `status`, and `description`,
   `maxResults: 100`, and `view: "full"`. If the result is cut off, use the
   returned records and tell the user it is partial, with the JQL. If Jira
   rejects the JQL, correct it once and retry; report a second error.
5. If there are no issues, say so and stop.
6. Create a fresh directory. Write the fetched records to `data.yaml` and copy
   `assets/gojira.view.yaml` to `view.yaml`. Describe the actual count and JQL
   in the view's description. Keep all fetched data in this temporary
   directory, never in the repository. Quote strings safely when serializing
   YAML. All repository examples must remain invented.
7. Activate the `twikko` skill and follow its launch, wait, continue, and
   result procedure using the two files. Do not improvise a separate launch
   flow. When handing over the URL, include issue count and exact JQL.
8. Report the final status and output path. All displayed issue fields are
   read-only, so there are no Jira changes to apply.

## Mapping Jira to data

Use one object per issue with these keys, matching
`assets/gojira.view.yaml`:

- `key`: Jira issue key, used as the immutable `id_field`.
- `title`: Jira summary.
- `type`: issue type name.
- `creator` and `assignee`: display names. Omit `assignee` when unassigned.
- `created`: date portion of the creation timestamp (`YYYY-MM-DD`).
- `due`: Jira due date; omit it when Jira omits the value.
- `status`: status name as returned.
- `summary`: a one to three sentence digest of Jira's description. Say
  `No description.` when it is empty. Do not infer details from the title.

All declared fields are read-only. The reusable view preserves the complete
gojira header and body arrangements, including every icon and status variant.
The header uses `type`, `key`, `title`, and `status`. The body shows creator,
assignee, created date, due date, and the description digest. Other record
keys remain in output unchanged but are not displayed.

## Applying results and compatibility

This view has `id_field: key`, so a saved change uses `entity_id` with the
original issue key. In general, a result for data without an ID uses
zero-based `record_index`, including index 0, and omits `entity_id`. Missing
`from` means the source key was absent; `from: null` means it existed with a
null value. `to` is always present. `entities` and `changes` are present on
every saved result. Cancelled results have `status: cancelled` and must not be
applied.

The old single-file `--input` protocol and built-in `schema.header` /
`schema.body` templates remain available for compatibility. New gojira runs
use `--view` and `--data`; do not combine the new flags with `--input`.
