# The gates enforce a usable map, and the remote enforces the gates

**Status**: accepted (v0.22.0–v0.22.3)

**Context**: after ADR 0003 agents answer from the notes with their own tools, so answer quality depends on the
notes working as a map. A review of three development vaults found notes that passed G1–G3 and still misled:
relations the code shows but no relation field declared, core sections saying "not observed" while the claims sat
in infrastructure sections, paragraphs copied between repository, topic and flow notes, and permalinks so long
that anchors went missing. The defects entered through a direct commit outside a sync branch, which no gate saw.

**Decision**:

- `discover check` adds G4, an `error` that blocks publication: `G4-relation` (a topic, queue or event the
  repository names, the host of another repository's HTTP or gRPC service, or an SFTP/FTP host of an
  integration, whose other end has a note, is neither in a relation field nor explained under the limitations
  section), `G4-structure` (an empty core section while most cited claims sit outside the core sections) and
  `G4-duplicate` (a paragraph repeated in a non-repository note). A note touched only to re-anchor may keep the
  errors its base had, never add one.
- A repository note cites its own repository at `commit-analizado` in the short form `path#Lfrom-Lto — text`; a
  permalink there fails `G1-format`, and `discover shorten` rewrites it mechanically.
- `sync verify` refuses a non-repository note on the branch that copies a repository note.
- The kernel installs `.github/workflows/knowledge-gates.yml`, which runs `sync verify --allow-no-change` on every
  push and pull request: each knowledge change must be covered by an accepted review, located by the
  `Knowledge-Base` trailer the review commit records. Its runner comes from `instance.yaml` `ci.runner`, because
  an organisation may block GitHub-hosted runners.
- `config locate --repo` names a repository's checkout, reference branch and note state for code reads.

**Considered options**: a rule that infrastructure claims must not outweigh core claims (false positives on
frontends and libraries); comparing repository notes with each other for copies (shared CI text is legitimate);
keeping the gates advisory in skills (the defects entered exactly where no one ran them).

**Consequences**: consistency is enforced by `kos` and by the remote, not advised. Paraphrased copies, the
correctness of a core flow and misplaced content in a non-empty section remain the reviewer's job. The effect on
answers is not yet measured: `evals/benchmark/bench.py --heldout` against the restructured vaults is the check.
