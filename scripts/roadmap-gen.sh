#!/usr/bin/env bash
# Regenerate docs/roadmap.md from the open issues of this repository.
#
# Source: `gh issue list` (EVERY open issue), docs/roadmap-status.tsv (one line per issue that has a
# status tag or a reason: number <TAB> status <TAB> reason) and docs/rank.tsv (the ranked top of the
# queue: number <TAB> rank <TAB> reason). Nothing is downloaded except what gh reads from GitHub.
#
# Usage:  bash scripts/roadmap-gen.sh [output-file]      (default docs/roadmap.md)
#
# An open issue is placed by its priority label: p0-critical, p1-now, p2-next, p3-later or p4-parked.
# One without a priority label that is a roadmap item, a bug or has `inbox` is listed under Inbox;
# every other one (audit trackers, design documents) under Reference (no priority). The category
# (the cat:* labels) is shown per row.
# A bug also carries the priority it was reported with (rp1-critical, rp2-high, rp3-normal, rp4-low,
# or rp-unset when the operator has not said); the page shows it in the Reported column. Within a
# working priority the bugs come first, ordered by reported priority (rp1 first, rp-unset last) and
# then by the date they were reported (oldest first); the rest follow, each by number.
# Ranked issues (docs/rank.tsv) come first within their priority, ordered by rank; the Rank column
# shows `-` for an issue that is not ranked. An unranked issue is not an error.
# Closed issues are never listed. The operator decides priority and rank changes; this script only
# writes down what the labels and the two tsv files say. It warns on stderr (never fails) about an
# open issue with no priority label and no cat:audit, and about a row of either tsv file whose
# issue is not open.
set -euo pipefail

root="$(git rev-parse --show-toplevel)"
cd "$root"

repo="$(gh repo view --json nameWithOwner --jq .nameWithOwner | tr -d '\r')"
name="${repo#*/}"
status_file="docs/roadmap-status.tsv"
rank_file="docs/rank.tsv"
out="${1:-docs/roadmap.md}"
tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT

# The label list is wrapped in commas so that an issue with no label still has a non-empty field
# (read with a tab IFS would otherwise swallow the empty field).
issues="$(gh issue list --repo "$repo" --state open --limit 1000 \
  --json number,title,labels,createdAt \
  --jq '[.[] | ([.labels[].name]) as $l
         | {n: .number, l: $l, t: .title,
            b: (if ($l | index("bug")) then 0 else 1 end),
            r: (if ($l | index("bug") | not) then 0
                elif ($l | index("rp1-critical")) then 1
                elif ($l | index("rp2-high")) then 2
                elif ($l | index("rp3-normal")) then 3
                elif ($l | index("rp4-low")) then 4
                else 5 end),
            c: (if ($l | index("bug")) then .createdAt else "" end)}]
        | sort_by([.b, .r, .c, .n]) | .[] | [.n, ("," + (.l | join(",")) + ","), .t] | @tsv' \
  | tr -d '\r')"

# status_of NUMBER -> "status<TAB>reason" (both may be empty)
status_of() {
  if [ -f "$status_file" ]; then
    awk -F'\t' -v n="$1" '$0 !~ /^#/ && $1 == n { print $2 "\t" $3; exit }' "$status_file" | tr -d '\r'
  fi
}

# rank_of NUMBER -> "rank<TAB>reason" from docs/rank.tsv (nothing when the issue is not ranked)
rank_of() {
  if [ -f "$rank_file" ]; then
    awk -F'\t' -v n="$1" '$0 !~ /^#/ && $1 == n { print $2 "\t" $3; exit }' "$rank_file" | tr -d '\r'
  fi
}

# priority_of LABELS -> p0-critical | p1-now | p2-next | p3-later | p4-parked | inbox | reference
priority_of() {
  local labels=",$1," p
  for p in p0-critical p1-now p2-next p3-later p4-parked; do
    case "$labels" in
      *",$p,"*) echo "$p"; return ;;
    esac
  done
  case "$labels" in
    *",roadmap,"* | *",bug,"* | *",inbox,"*) echo inbox ;;
    *) echo reference ;;
  esac
}

# category_of LABELS -> the cat:* labels without the prefix, comma separated; "-" when there is none
category_of() {
  local out="" l
  local IFS=,
  for l in $1; do
    case "$l" in cat:*) out="${out:+${out}, }${l#cat:}" ;; esac
  done
  echo "${out:--}"
}

# reported_of LABELS -> the reported priority of a bug as shown on the page; "-" for anything that is not a bug
reported_of() {
  local labels=",$1," p
  case "$labels" in *",bug,"*) ;; *) echo "-"; return ;; esac
  for p in rp1-critical rp2-high rp3-normal rp4-low; do
    case "$labels" in
      *",$p,"*) echo "${p%%-*} ${p#*-}"; return ;;
    esac
  done
  echo "unset"
}

# cell_text TEXT -> TEXT as plain text inside a table cell of docs/roadmap.md. An issue title is written by anyone who can open an issue and the page is committed, so
# nothing in it may act as markup or HTML: the backslash, the markdown characters and the pipe get a backslash, the angle brackets and the ampersand become entities, and
# the start of a web address is broken so that the page does not turn a bare address into a link either (a title that is `[text](url)` or an img tag shows as the text it is).
cell_text() {
  printf '%s' "$1" | sed \
    -e 's/\\/\\\\/g' \
    -e 's/&/\&amp;/g' \
    -e 's/</\&lt;/g' \
    -e 's/>/\&gt;/g' \
    -e 's/[][`*_~|!]/\\&/g' \
    -e 's,://,\&#58;//,g' \
    -e 's/\([wW][wW][wW]\)\./\1\&#46;/g'
}

# warnings: stderr only, never a failure
open_numbers="$(awk -F'\t' '{ print $1 }' <<< "$issues")"
while IFS=$'\t' read -r num labels _; do
  [ -n "$num" ] || continue
  case "$(priority_of "$labels")" in
    inbox | reference)
      case ",${labels}," in
        *",cat:audit,"*) ;;
        *) echo "warning: #${num} has no priority label (p0-critical to p4-parked) and is not cat:audit" >&2 ;;
      esac
      ;;
  esac
done <<< "$issues"
for f in "$status_file" "$rank_file"; do
  [ -f "$f" ] || continue
  while IFS=$'\t' read -r num _; do
    num="${num%$'\r'}"
    case "$num" in '' | '#'* | *[!0-9]*) continue ;; esac
    case $'\n'"${open_numbers}"$'\n' in
      *$'\n'"${num}"$'\n'*) ;;
      *) echo "warning: ${f} has a row for #${num}, which is not an open issue" >&2 ;;
    esac
  done < "$f"
done

# section PRIORITY HEADING DESCRIPTION
section() {
  local want="$1" heading="$2" blurb="$3" count=0 ranked="" others="" lines="" num labels title prio st status reason cell line rk rank why
  while IFS=$'\t' read -r num labels title; do
    [ -n "$num" ] || continue
    prio="$(priority_of "$labels")"
    [ "$prio" = "$want" ] || continue
    st="$(status_of "$num")"
    status="${st%%$'\t'*}"
    reason=""
    case "$st" in *$'\t'*) reason="${st#*$'\t'}" ;; esac
    rk="$(rank_of "$num")"
    rank="${rk%%$'\t'*}"
    why=""
    case "$rk" in *$'\t'*) why="${rk#*$'\t'}" ;; esac
    case "$rank" in '' | *[!0-9]*) rank=""; why="" ;; esac
    title="$(cell_text "$title")"
    reason="$(cell_text "$reason")"
    why="$(cell_text "$why")"
    cell=""
    [ -z "$status" ] || cell="**[${status}]**"
    [ -z "$reason" ] || cell="${cell:+${cell} }${reason}"
    line="| ${rank:--} | [#${num}](https://github.com/${repo}/issues/${num}) | $(category_of "$labels") | $(reported_of "$labels") | ${title} | ${cell} | ${why} |"
    if [ -n "$rank" ]; then
      ranked="${ranked}${rank}"$'\t'"${line}"$'\n'
    else
      others="${others}${line}"$'\n'
    fi
    count=$((count + 1))
  done <<< "$issues"
  if [ -n "$ranked" ]; then
    lines="$(printf '%s' "$ranked" | sort -s -n -t "$(printf '\t')" -k1,1 | cut -f2-)"$'\n'
  fi
  lines="${lines}${others}"
  printf '## %s\n\n%s Items: %d.\n\n' "$heading" "$blurb" "$count"
  if [ "$count" -eq 0 ]; then
    printf 'Nothing.\n\n'
  else
    printf '| Rank | Issue | Category | Reported | What | Status | Why this rank |\n| --- | --- | --- | --- | --- | --- | --- |\n%s\n' "$lines"
  fi
}

{
  cat <<EOF
# Roadmap: ${name}

Generated by \`scripts/roadmap-gen.sh\` from every open issue (its priority and category labels), from \`docs/roadmap-status.tsv\` (status tags and reasons) and from \`docs/rank.tsv\` (the rank of the top of the queue, with a reason). Do not edit by hand: change the labels or the tsv files and run \`bash scripts/roadmap-gen.sh\`.

Last regenerated: $(date +%F)

## How we work the roadmap

- Every feature, idea or bug the operator raises is an issue. A feature or idea gets \`roadmap\` and \`inbox\` (and \`idea\` while it is only an idea); a bug gets \`bug\`, \`inbox\` and a reported priority (below).
- A new item waits. It is not started the moment it is raised: it sits in the Inbox until the next priority check. Only a P0 starts at once.
- **P0 (\`p0-critical\`)** is urgent: data loss, a safety problem, or it stops the operator playing now. A P0 jumps the queue.
- Several tasks may be in flight in parallel. They are taken from the top of the list: \`p1-now\` first, then \`p2-next\`, then \`p3-later\`. \`p4-parked\` is not worked; each parked item says why.
- **Rank.** The top of the queue (all of \`p1-now\` and the first items of \`p2-next\`) has a rank in \`docs/rank.tsv\`, and within its priority a ranked issue comes before an unranked one, lowest rank number first. The rank rule is: data loss or safety first, then what blocks playing, then wrong information shown on screen, then a daily annoyance, then polish, and a tie goes to the smaller job; the operator reorders the file, and an unranked issue is not an error.
- The priorities are checked when a task finishes and at the start of each session: the Inbox is read, each new item gets exactly one of \`p1-now\`, \`p2-next\`, \`p3-later\` or \`p4-parked\` (and loses \`inbox\`), and items may move between them. Keep \`p1-now\` to what the operator needs now.
- **Bugs come before features** (operator rule, 2026-10-25): within the same priority, open bugs are worked first. A new bug the operator reports gets \`p1-now\` unless it is trivial or needs the operator or the game, in which case it is labelled so (waiting on the operator / needs the game) but never left in the inbox for long.
- **A bug carries two priorities.** The *reported priority* is the priority the bug was reported with, how bad the operator says it is (the labels \`rp1-critical\`, \`rp2-high\`, \`rp3-normal\`, \`rp4-low\`, or \`rp-unset\`); the *working priority* is \`p0-critical\` to \`p4-parked\`, how we schedule it. Exactly one \`rp\` label per bug.
- The reported priority is set from the operator's own words and nothing else: "this can't happen, ever" or "it stops me" is \`rp1-critical\`; "wrong, but I can work" is \`rp2-high\` or \`rp3-normal\`; cosmetic is \`rp4-low\`. When he says nothing the label is \`rp-unset\` and the assistant asks him at the next check. The label is changed only when he changes it; a comment on the issue says where it came from.
- The working priority is never lower than the reported priority without the operator's OK: \`rp1-critical\` means at least \`p1-now\`, and \`p0-critical\` when the bug loses data, is unsafe, or stops him playing now; \`rp2-high\` at least \`p2-next\`; \`rp3-normal\` at least \`p3-later\`. (\`rp-unset\` is asked about before it is parked.)
- This page lists the ranked issues first within each working priority, then the bugs ordered by reported priority (\`rp1\` first, \`rp-unset\` last) and by the date they were reported, oldest first, then the rest by number. The Rank column shows \`-\` for an unranked issue, and the Reported column shows the reported priority of a bug.
- An issue with no priority label that is not a roadmap item or a bug (an audit tracker, a design document) is listed last, under Reference (no priority).
- The operator decides every priority change. A check does not interrupt work already in flight.

Status tags: **in flight** (being worked), **waiting on the operator's in-game check** (built, nothing more to do until it is tried), **needs the game** (can only be finished and checked in the game), **blocked on a save** (needs a save that has the thing), **partly answered**, **parked** (with the reason). An item with no tag has not been started.

EOF
  section p0-critical "Urgent (P0)" "Jumps the queue: data loss, a safety problem, or it stops the operator playing now."
  section p1-now "Now (p1)" "What the operator needs now."
  section p2-next "Next (p2)" "Taken once the Now items are done or waiting."
  section p3-later "Later (p3)" "Wanted, not yet scheduled."
  section p4-parked "Parked (p4)" "Not worked until the reason given is gone."
  section inbox "Inbox (unprioritised)" "New items waiting for the next priority check."
  section reference "Reference (no priority)" "Audit trackers, design documents and other issues that are not scheduled work."
} > "$tmp"

mv "$tmp" "$out"
trap - EXIT
echo "wrote $out"
