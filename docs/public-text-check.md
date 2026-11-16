# The public-text check

This repository is public. Everything that is written in it or about it is public too: tracked files, issue and pull request titles and
bodies, comments, reviews, releases, branch and tag names, and commit messages. One rule covers all of it (audit 2026-11-14 F1, #653):

- No reserved word. The list is kept only as SHA-256 digests, so the code and this page do not spell what the list keeps out, and a failing
  report never repeats it. Another private tool or project is never named; say "a related project" or leave the cross-reference out.
- No path under a personal profile. Write `%USERPROFILE%`, `%LOCALAPPDATA%` or `<path>`, never a folder that names an account.
- No AI attribution line or co-author trailer in a commit message, or in the title and body of a pull request (the squash message on `main`
  is made from them).

The rule engine is the package `internal/hygiene`; the program that reads event text is `cmd/publictextcheck`. Both have tests that run in CI.

## Where it runs

| What is read | Workflow | Check name | Required |
|---|---|---|---|
| The tracked files | `ci.yml` (the `go` job runs the package test) | the CI jobs | yes, with CI |
| A pull request's title, body, branch name and commit messages (opened, edited, synchronize, reopened) | `public-text.yml` | `public-text` | the operator adds it by hand (below) |
| An issue's title and body (opened, edited, reopened), comments on issues and pull requests, reviews and review comments, releases, created branches and tags | `public-text-discussion.yml` | `public-text-discussion` | no |
| The commit messages of a push to `main` | `public-text-push.yml` | `public-text-push` | no |

The required check is `public-text` and no other workflow may use that name: two runs with one name on one commit let the newest result decide,
so a run that checks less could hide a failed one. A test keeps it that way. A comment or an issue is text of other people, so its check is a
warning for a maintainer, not a gate on a pull request.

A failing run names the place (for example `issue #12 body, line 3`) and the rule, never the text, because the log of a public repository is public.
When the text is edited, the check runs again.

## How the workflows are built

- The text is read from the event payload file the runner writes (`GITHUB_EVENT_PATH`) by the Go program, never from an expression in a shell
  step. A `${{ }}` expression in these workflows holds a secret, a number, an id or a commit id and nothing else, and a test fails if one
  holds more. Text written by a stranger is data and never code.
- The token is read-only (`permissions: contents: read`). The workflows use the plain `pull_request`, `issues` and comment events, not
  `pull_request_target`, so no write token runs over text from a fork.
- The actions are pinned to a commit id and no third-party action is used (checkout and setup-go only).
- The optional repository secret `PUBLIC_TEXT_EXTRA_DIGESTS` adds digests to the built-in list (hex digests, separated by commas, spaces or new
  lines). It is empty for a pull request from a fork, which then gets the built-in list only.

## Checking the text that is already there

Fetch the items and run the program on the file. Write the file outside the repository and delete it afterwards:

```
gh issue list --state all --limit 1000 --json number,title,body,comments > issues.json
gh pr list --state all --limit 1000 --json number,title,body,headRefName,comments,reviews > prs.json
go run ./cmd/publictextcheck -items issues.json
go run ./cmd/publictextcheck -items prs.json
```

The report lists place and rule. On a local machine `-show` also prints the text; it is refused in CI. A pull request carrying an AI attribution line
in a body that was merged before the rule existed is reported too: that is the same rule, applied to the text that became a squash message.

## Adding a word to the list

`go run ./cmd/publictextcheck -digest WORD` prints the digest of a word, or of up to four words in a row. Put the digest in the repository secret
`PUBLIC_TEXT_EXTRA_DIGESTS` rather than in the source, so the list does not grow in public. The check finds a word however it is written: with
hyphens or spaces between its parts, over a line break, in other case, or with hidden characters inside it.

## When a check fails

1. Edit the title, body or comment to say what is needed in neutral words. The check runs again on the edit.
2. An edit leaves the earlier text in the page's edit history. If the text must go, delete that revision by hand in the "edited" menu of the item
   (where GitHub offers it). Old commits are not rewritten: a history rewrite breaks clones and links, so a word that was once in a commit
   stays in the history as a residual risk.

## The operator's step: the required check

Adding `public-text` to the required status checks of the branch rule for `main` is a setting of the repository, done by hand in the repository
settings. Nothing in the repository changes that rule. Until it is done, a failing `public-text` run is visible on the pull request but does not
block a merge.

## What is not covered

- Comments on a commit: GitHub has no workflow trigger for them (a trigger it does not know makes it refuse the whole file).
- Discussions: the repository has them turned off. If that changes, add the discussion events and a payload test.
- The names and descriptions of labels and milestones: short, maintainer-written, and no event this check reads.
- A release or a created ref runs the workflow file as it is at that tag or branch, so the check reaches only a ref made after the file was merged
  into it, and GitHub starts no run when more than three tags are pushed at once.
