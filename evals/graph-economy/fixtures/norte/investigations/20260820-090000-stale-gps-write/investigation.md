---
id: 20260820-090000-stale-gps-write
title: Stale GPS writes into dispatch
dedupe-key: stale-gps-write
status: investigating
created-at: 2026-08-20T09:00:00-04:00
updated-at: 2026-08-20T09:00:00-04:00
source-type: message
source-ref: fixture
requester-role: engineer
export-intent: none
purpose: knowledge
vault-outcome: candidate-for-audit
learning-outcome: documented
---

# Stale GPS writes into dispatch

Touches [[routing-planner]] and [[Flujo - Dispatch]]. GPS older than the plan horizon overwrites a live assignment.
