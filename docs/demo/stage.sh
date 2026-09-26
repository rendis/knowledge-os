#!/usr/bin/env sh
# Advance the demo cell to the state a recording starts from, off camera.
# Usage (inside the vault, with the demo environment): demo-stage discovered|case|handoff-work|topic-note
set -eu
V=$(pwd)
DEMO=$(dirname "$V")
WT="$DEMO/worktrees/acme-ledger/idempotent-entries"

case "$1" in
  discovered)
    kos discover run >/dev/null
    kos discover answer --file .scratch/answers.json >/dev/null
    kos discover platform --provider gcp --scope acme-prd >/dev/null
    kos discover run --classify off >/dev/null
    git add 90-Meta/discovery && git commit -q -m "chore(discovery): first run"
    ;;
  case)
    id=$(kos investigation new --title "Duplicate ledger entries" --type development \
      --objective "One ledger entry per order event." | jq -r .id)
    kos investigation add --id "$id" --kind requirement --text "A retried event creates one ledger entry" \
      --origin "requester 2026-09-26" >/dev/null
    mkdir -p ".investigations/$id/handoffs"
    cat > ".investigations/$id/handoffs/DH-001.md" <<PKG
---
handoff: DH-001
case: $id
repository: https://github.com/acme/acme-ledger.git
base: main
branch: issue/idempotent-entries
---

# Idempotent ledger entries

## Task

Meet R-001.

## Changes

- \`cmd/main.go\`: key each entry by the order ID.

## Acceptance criteria

- A retried event creates one entry (unit test, R-001).
PKG
    echo "$id" > "$DEMO/case-id"
    ;;
  handoff-work)
    export GIT_AUTHOR_DATE=2026-09-02T10:00:00Z GIT_COMMITTER_DATE=2026-09-02T10:00:00Z
    printf '\n// Entries are keyed by the order ID.\n' >> "$WT/cmd/main.go"
    git -C "$WT" commit -q -am "feat: key ledger entries by order ID" -m "Handoff: DH-001"
    commit=$(git -C "$WT" rev-parse --short HEAD)
    cat >> "$WT/.handoff/deltas.md" <<DELTA

## DELTA-001 — The order ID arrives as an attribute
- Handoff: DH-001
- Type: finding
- Detail: order events carry the ID in the orderId attribute, not in the body.
- Evidence: \`cmd/main.go@$commit\`
DELTA
    ;;
  topic-note)
    cat > 25-Topics/order-events.md <<'NOTE'
---
tipo: topic
nombre-raw: "projects/acme-prd/topics/order-events"
sistema: "[[Orders]]"
tags: []
---

# order-events

## Qué representa

Order lifecycle events published by acme-orders.

## Contrato

Each message carries an `orderId` attribute.

## Infraestructura verificada

Topic and subscriptions captured from `gcp:acme-prd`: `ledger-order-events-sub`, `notifier-order-events-sub`.

## Limitaciones y desconocimientos

The message schema is not versioned in the repositories.
NOTE
    ;;
  *) echo "unknown stage: $1" >&2; exit 2 ;;
esac
