---
name: gojira
description: "Pull Jira tickets through the Atlassian MCP and open them in multieditor for review. Runs only when the user invokes it by name (/gojira or $gojira)."
disable-model-invocation: true
argument-hint: "[search criteria: JQL, or plain words like 'my bugs in PROJ']"
---

# gojira

Fetch Jira tickets, write them to a YAML file that `multieditor` accepts as input, and open that file in `multieditor` so the user can review the tickets. This skill never changes anything in Jira.

Search criteria from the user: $ARGUMENTS

If that line is empty, the user set none.

## Procedure

1. **Check the Atlassian MCP.** You need `searchJiraIssuesUsingJql` and `getAccessibleAtlassianResources`, which the Atlassian MCP server provides. If they are missing or the server is not authenticated, stop and tell the user. Do not fall back to `curl` or the Jira REST API.
2. **Get the `cloudId`.** Call `getAccessibleAtlassianResources`. With one site, use it. With several, ask the user which one.
3. **Build the JQL.**
   - Criteria given as JQL: use it verbatim.
   - Criteria given in words: translate to JQL. Keep only what the user said. Do not add a status or assignee filter they did not ask for.
   - No criteria: `assignee = currentUser() AND statusCategory != Done ORDER BY created DESC`. `statusCategory != Done` excludes every status the project puts in the Done category, which in a standard workflow includes "Won't Do" and "Rejected". `currentUser()` makes an `accountId` lookup unnecessary.
4. **Search.** Call `searchJiraIssuesUsingJql` with `fields: ["summary", "issuetype", "creator", "created", "duedate", "assignee", "status", "description"]`, `maxResults: 100` and `view: "full"`. The `compact` view drops the description and `creator`, and `evidence` still drops `creator`, even when `fields` lists them. A full page is tens of kilobytes, so the harness usually saves the result to a file and returns only its path and a preview. Read that file with a script such as `python3 -I`, and print only the fields you need. Do not load the whole file into context. If Jira rejects the JQL, fix it once and retry. If it fails again, report the error to the user.
5. **Check completeness.** If `isLast` is false, the result is cut off at 100 tickets. Write what you have and tell the user the result is partial, showing the JQL so they can narrow it. Do not page on silently: a long card list is hard to review.
6. **Write the YAML** into a fresh directory from `mktemp -d`, as `in.yaml`. Follow `assets/example-input.yaml`.
7. **Stop if there are no tickets.** If the search returned none, write no file, say so, and end here.
8. **Start multieditor.** This needs the tested procedure for starting the tool, handing the user its URL, waiting for Save or Cancel, and handling `continue`: **activate the multieditor skill** and follow it. The input file already exists from step 6, so start from launching the process, with `out.yaml` in the same directory as the output path. When you give the user the URL, add the ticket count and the exact JQL used. If the multieditor skill is not installed, stop and report the path of `in.yaml` instead. Do not improvise the launch.
9. **Report the outcome** once multieditor exits: the status and the path of `out.yaml`. Every field is read-only, so Save and Cancel both leave nothing to apply. Do not write anything back to Jira.

## Mapping Jira to entities

One entity per ticket, one key per row:

- `key` is the issue key, for example `PROJ-123`.
- `title` is Jira's `summary` field.
- `type` is the issue type name from `issuetype`, for example `Epic` or `Task`.
- `creator` and `assignee` are the person's display name. Omit `assignee` when the ticket is unassigned. Do not write `null`, which multieditor rejects for some types.
- `created` is `YYYY-MM-DD`, cut from the `created` timestamp.
- `due` is Jira's `duedate`, already `YYYY-MM-DD`. Omit the key when the ticket has no due date: Jira leaves `duedate` out of the response then.
- `status` is the status name as Jira returns it.
- `summary` is your own digest of the ticket's `description`: one to three sentences on what is wanted and why. It is not Jira's `summary` field, which is the title. When the description is empty, write `No description.` Do not invent detail from the title.

The card header comes from `header: { template: gojira }` under `schema`, a template built into `multieditor`. It draws an icon for `type`, a tag for `key`, the `title` in bold, and a tag for `status`. It reads exactly those four keys, so keep their names. Do not write `title_field` or a `card_title` key, and do not write widgets yourself. A type or status the template does not know gets a fallback icon or a neutral tag.

Every field is `readonly: true`. The user reviews tickets here and edits none of them, and no edited value could be written back to Jira anyway. Field type is `text`, except `created` and `due` as `date` and `summary` as `longtext`.

Text values that contain `: `, a leading `#`, or other YAML syntax must be quoted. Ticket titles trip this often. When a script writes the file, emit every string with `json.dumps(value, ensure_ascii=False)`: a JSON string is a valid double-quoted YAML scalar, so no PyYAML install is needed.
