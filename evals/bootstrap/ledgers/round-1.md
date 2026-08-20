# Round 1 ledger

Frozen tree sha256: `3bf8596dfabbeed76491ae0d4b209a0c9634ecf5d2c194e86a996ecedb316be6`

Judges: [Judge A](680a6138-acb2-4eec-ae5b-bf3ccc988517), [Judge B](0e183ada-52a9-4ecd-b10e-1ed1217c1b51) (pre-interrupt JSON), plus late leak-focused [Judge A](b35ba995-0493-47b9-936b-927f517db3c2) and [Judge B](ca6e7666-8dd6-48ec-8851-60c9eb74d1c5).
Persona A: [Persona A](4f8b101f-ed84-4426-aac8-1e7ba3efeaa5) — init exit 0; started at `00-Home.md`; did not need another company vault. Duplicate [Persona A](1cd94849-5779-4e16-8136-c109df2b3ffe) agreed on Home start; WARNING only.

Interrupted empty results from [Judge A](24b80555-0961-48cc-8f0a-76ab06111c81) and [Judge B](eb159553-5ba4-4c94-8ea4-9c9cd56529b0) are discarded.

## Confirmed SEVERE (both judges)

| ID | Pair | Claim | Fix |
|---|---|---|---|
| C1 | J1-01 + J2-01 | Reports adapter ships a live store-operations recipe (`cell-monthly-transactions`, `proj-a01-prd`, Chile/Colombia/Peru, PIC/PLF events) and the motor hard-codes that geography | Round-1 fix: generic `sample-monthly-events` |
| C2 | leak J1-02 + J2-02 | Kernel example `investigation-id` contains `tagger` | Round-2 fix: generic example id |
| C3 | leak J1-03 + J2-03 | Leak eval skips `.json`/`.sql` and does not match `cell-monthly` / `tagger` / `Cell A` | Round-2 fix: scan those suffixes and strings |

## Suspect (one judge SEVERE only)

| ID | Claim | Other judge |
|---|---|---|
| J1-04 | `init` on nonempty dest is destructive | leak J2-06 WARNING |
| J1-05 | `pending_inventory` true from Home copy | J2-04 / leak J2-05 WARNING; Persona A PA-001 WARNING |
| J1-06 | SYS001/SYS002, System B, System BR leftovers | J2-06 WARNING |

## INFO (WARNING/SUGGESTION)

Topics still seeded when `--disable-topics`; skill description bloat; missing `git-change-manifest.py` / `static-evidence-scan.py`; locale es vs English Home; system stubs copy cell purpose.

Persona A: cell identity and systems were obvious. Next step only partially obvious because of pending-inventory false positive.
