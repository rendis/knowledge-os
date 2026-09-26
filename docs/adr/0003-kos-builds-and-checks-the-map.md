# kos builds and checks the map; agents read and search with their own tools

Up to 0.20.0 `kos` also answered questions: `ask` assembled an evidence pack (hit map, paragraphs with their
source state, cited code, the matching function with its exits, topic wiring), `read` returned a note with its
footnotes checked, `code` read a repository at its reference branch (grep, files, functions with callers and
exits), and `overview`, `search`, `links` and `index` navigated a local SQLite index. `discover claims`
checked a draft answer's resource names, Jev (`TYPESAFE_API_KEY`) answered discovery judgments and checked
cited sentences (`discover check --semantic`), and an Obsidian-native mode let agents query the vault through
the Obsidian app (`check obsidian-binding`).

Rounds of agent feedback kept asking for more query features. Two comparisons then measured them directly: ten
held-out questions over three real cell vaults, each answered twice in fresh headless sessions with `kos` on
`PATH` and twice without it (any `kos` binary refused by the sandbox), the same prompt, model and fixture,
graded blind against reference answers.

| | With kos | Without kos |
|---|---|---|
| Evidence pack (`ask` 0.21.0-dev) | score 0.671, 9 violations, USD 1.01, 150 s | 0.639, 8 violations, USD 1.05, 128 s |
| Vault map (`ask` listing every matching note) | 0.647, 8 violations, USD 0.99, 177 s | 0.672, 10 violations, USD 1.08, 155 s |

Every score fell inside the 0.60–0.71 spread between two runs of the same arm: the query layer made no
measurable difference in accuracy, violations or cost, and took longer. Without `kos`, agents read the
reference branch with `git show` and `git grep` because the router's evidence contract requires it. The pack
also anchored agents on the most mentioned note: in one question it hid the second of two code paths, which
agents without it found by searching the notes. Agents with `kos` used `discover claims` in most answers and
still made as many violations; those were errors of interpretation, not invented names. What improved answers
in both arms was the content: notes corrected through `sync` after the comparison.

`kos` keeps what belongs to the vault as a kernel and what an agent cannot do alone:

- identity and installation: `init`, `adopt`, `doctor`, `kernel`, `config`, `vaults`, `version`, `update`;
- building and checking the map: `discover` (run, questions and answers, platform snapshots, note gates,
  reports, corrections), `inventory`, `audit`, `check links|bases`;
- publication with gates and an independent review: `sync`;
- state with gates: `investigation` (whose gate still checks the names a case uses against the discovery
  facts) and `handoff`.

Removed: `ask`, `read`, `code`, `overview`, `search`, `links`, `index` and the SQLite index; `discover claims`;
Jev (`--classify`, `--semantic`, `TYPESAFE_API_KEY`); `check obsidian-binding` and the resolver's Obsidian
probe; `check map-closure`, which was documented but never dispatched. The router now tells agents to search and
read notes with their own tools, to read code at the reference branch with git (`inventory --repo` names the
branch, `config locate` the checkout), to run `discover check` on the repository notes they rely on, and to
confirm every resource an answer names in a note, the discovery facts or the platform snapshots.

**Considered options**: keep the query layer as an optional aid (it has to be taught by skills and router text,
and it measured neutral while anchoring answers on one note); reduce `ask` to a map of every matching note with
its source state (measured in the second comparison: neutral as well); keep `discover claims` and Jev because
they check rather than search (neither reduced errors in the measurements, and the agent does the same checks
with the notes, facts and snapshots).

**Consequences**: about 9 000 lines and the SQLite dependency are gone, and the skills describe techniques
instead of commands. A question about a cell is answered with the harness's own tools over a vault whose notes
the gates keep fresh; accuracy is improved by correcting notes through `sync`, not by query features. The
measurement covered one strong model on well-curated vaults; a weaker model or a larger vault could show a
difference, and a query feature returns only with a with/without comparison that shows it.
