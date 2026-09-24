# Local retrieval CLI: product decision record and acceptance plan

Status: **v1 search architecture and Engram-style interaction selected; product implementation pending**,
2026-09-24. No hook is installed and no model is selected for distribution. A
single-process Go `search` command was tested on macOS ARM in an isolated PoC.
The user later discarded and deleted that PoC, including its handoff, so the
measurements recorded below are historical experimental results rather than
a runnable benchmark. Reproduce them against the selected CLI before release.
Jev routing was discarded by the user. Harness events will be selected later.

## Decision after the passage experiment

**Build v1 as one Go CLI per target OS/architecture, with a pure-Go SQLite FTS5
index of Markdown passages. Do not bundle an embedding model or a neural
reranker in v1.** Search returns five distinct source groups by default, with
short match-near excerpts and explicit provenance; an agent may request up to
ten when it needs broader evidence. Markdown remains authoritative. Persist
FTS5 locally and refresh changed files by SHA-256 at the selected invocation.

**V1 follows the Engram interaction pattern:** give the agent a bounded
project orientation at session start, then let it invoke the CLI's explicit
`search` command when it needs documentary evidence. A message hook records
the prompt in local, project- and session-scoped state, but returns no document
cards and does not search the vault on every turn. Do not ship `suggest` in v1. The tested
message classifiers could not reliably return zero cards when appropriate;
their research is retained in [suggestion-gate-models.md](suggestion-gate-models.md).
This decision is based on quality *and* resource cost, not an assumption that
FTS5 always beats semantic retrieval.

| Same 296-note Cell A corpus | First 3 | First 5 | First 10 | No-change CLI median, 5 runs |
| --- | ---: | ---: | ---: | ---: |
| File-level FTS5 (earlier PoC) | 27/36 | 30/36 | 32/36 | 144 ms |
| **Passage FTS5, chosen v1** | **33/36** | **34/36** | **36/36** | **152 ms** |
| Passage FTS5 + Bekko RRF | 33/36 | 34/36 | 36/36 | 863 ms |

These are two 18-question, source-derived regression sets; the target is a
labeled source path (including the artifact behind a grouped case card). The
passage change was made once, then tested on both sets without tuning weights
to their failures. It does **not** prove equal answer quality on all future
queries. It does show that the current hybrid adds no top-5 or top-10 target
coverage over passage FTS5 on this evidence while making every call about
5.7 times slower and loading roughly 700 MiB of model runtime on this Mac.
The hybrid and lexical misses differed, but the user discarded embeddings for
this product. Reconsider that decision only if new held-out tasks establish a
material improvement in useful evidence or answer correctness on target PCs.

A diagnostic check of seven actual Cell A task topics found useful FTS5
starting points for the TAGS investigation and BrandY relabeling. It also
showed weak cards for live synchronization status and inactive-store validation;
neither engine can establish current runtime state from Markdown. The topics
were shortened for this check and were not scored as a blind benchmark. They
reinforce the decision against unconditional message-hook injection.

For the one-file target, a pinned pure-Go SQLite driver is the selected
implementation direction. An isolated `modernc.org/sqlite` v1.59.0 prototype
created and queried FTS5 and built with `CGO_ENABLED=0` for macOS, Linux and
Windows on both amd64 and arm64 (six 9.0–9.4 MiB minimal binaries). On this
Mac, inserting the same 3,193 passages took 53 ms median in five hot runs;
36 repeated FTS5 queries took 86 ms; measured maximum RSS was about 31 MiB.
**Cross-compilation is not a runtime test:** Linux/Windows execution, locking
and release packaging still require target-host validation. Include the
driver's binary redistribution notices in release materials.

## Product boundary

Ship **one CLI interface** for each supported OS/architecture. Harness adapters
and agent skills invoke commands; they do not implement retrieval or ranking.
Each cell opts into its own local binary/configuration. Markdown in the cell is
authoritative; SQLite indexes and any conversation state are local, private,
reconstructible, and scoped by the canonical cell path. A result is a pointer
to evidence, never an answer or an instruction to trust the matched text.

The requested end state also replaces the distribution's Python runtime
commands with CLI subcommands. This is a separate migration gate from proving
retrieval quality. The current tree contains **32 Python files outside evals**
(including four skill scripts and four kernel tests), about **22,905 lines**,
and at least **66 Markdown files** refer to Python commands. The installer
itself calls `scripts/knowledge_os.py`. Removing those files before replacing
their behavior and command references would break init/update/doctor and skills.
The 66 Python files under `evals/` are development tests, not installed runtime;
they need not disappear to make the consumer Python-free.

### Proposed command contract

| Command | Purpose | Output |
| --- | --- | --- |
| `vaultctl search --vault PATH --query TEXT` | Refresh the derived index and retrieve evidence when the agent asks directly | Bounded JSON cards, source paths, rank and timings |
| `vaultctl context --vault PATH --session ID` | Prepare bounded context for a future session-start adapter | Project identity, navigation pointers, limited recent session state and search guidance; no topical result cards |
| `vaultctl session record --vault PATH --session ID` | Accept a prompt from a future message adapter via JSON stdin | Local acknowledgement; no document search or injected cards |
| `vaultctl index status|refresh --vault PATH` | Inspect or refresh corpus and per-file hashes | Counts of unchanged/inserted/updated/deleted files; index schema version |
| `vaultctl vault init|adopt|update|doctor ...` | Replace `install.sh` / `knowledge_os.py` behavior | Existing exit/output contract, after parity tests |
| `vaultctl check ...`, `vaultctl graph ...`, `vaultctl investigation ...` | Replace kernel and skill script families | Existing schemas and failure semantics, after parity tests |

Command names are provisional. `context` is bounded orientation and recent
conversation state, not a query against every document; `search` is invoked
for a concrete information need. The prompt record and the returned context
must remain local to the cell and distinguish conversational state from
documentary evidence. A skill documents the stable CLI contract, when to
search and how to open original sources. None of these commands requires a
model. Select exact harness events and session-ID mapping later; an ID must be
stable within one conversation and distinct across conversations.

### Selected interaction sequence

1. A project opts into its harness adapter. At session start, `context`
   returns a size-limited orientation and a small recap of recent state from
   **that project** when available. It labels session records as conversation
   history, not as source evidence; it does not paste whole notes or a full
   transcript.
2. On each user message, the adapter records the prompt locally under the
   project and session IDs. This operation returns no search results to the
   agent. A failed capture must not block the user's request.
3. When the task needs documentary evidence, the agent invokes `search` with
   an explicit query, receives a few source pointers with provenance, then
   opens the chosen Markdown. Zero search matches are valid and do not prove
   that the answer is unavailable elsewhere.

The SQLite conversation state and the FTS5 projection are local derived data;
the Markdown remains the documentary authority. Compaction and session-end
events, if supported by a harness, will be mapped after the CLI contract is
tested. They do not change the no-search-on-every-message decision.

## Retrieval implementation to test

1. Enumerate eligible Markdown by the cell's explicit scope and visibility
   policy, including permitted private local material. Exclude generated/local
   state and instructions from result cards unless a task explicitly searches
   them. Record path, origin class and exact SHA-256 for every included file.
2. On a selected invocation, compare hashes for **all** eligible files and
   update passages for only changed/deleted files, as previously agreed.
   Publish the new source metadata and FTS5 rows in one transaction. Extraction
   rules and scope changes invalidate the projection. If a later semantic
   experiment is enabled, it must use the same source generation.
3. Search separate fields (title, aliases, heading, body/passage) with FTS5 and
   weighted BM25. The passage, not the whole file, is the scoring unit; return
   each logical source once. FTS5 BM25 is a ranking score, not comparable
   across arbitrary queries or calibrated as answerability. A vector dot
   product/cosine is likewise only similarity. Do not average raw scores.
4. Merge and deduplicate candidates by logical source. An investigation's
   `investigation.md` may be the display route while the result preserves the
   matched artifact path. Preserve the difference between project notes,
   guidance, investigation material, and private/local sources; directory
   location alone does not imply canonicality or approval.
5. Return a small card: title, cell, display path, matched path, source class,
   heading/line if verified, a short excerpt near the match, rank, and source
   hash. Require the agent to open the Markdown before making factual claims.

FTS5 is the chosen v1 engine and works without a model. Semantic retrieval
remains an isolated experimental capability; no model is distributed in v1.
`rg` remains an emergency read-only fallback if
the local index cannot be opened; it does not impersonate ranked FTS5 results.

### Candidate selection and the rejected automatic gate

A *passage* is a searchable piece of one Markdown note, usually text under a
heading and, if the section is long, a bounded group of paragraphs. The tested
PoC split at headings and grouped paragraphs to about 900 bytes. BM25 ranks
these pieces; the result card points back to the containing source note and
shows a short nearby excerpt. The passage is an indexing unit, not a new
authoritative document.

SQLite FTS5 returns passages containing query terms and BM25 orders matches.
It does not classify whether a user turn is a standalone information request.
A vector extension likewise measures supplied vectors, not conversational
intent. Explicit `search` can return `cards: []` when no passage matches, but
that does not amount to a validated per-message suggestion policy. No model or
Go NLP library earned inclusion for such a gate.
The model and dataset review, including fastText and FastFit candidates, is in
[suggestion-gate-models.md](suggestion-gate-models.md).
That review now includes authored cross-validation plus held-out turns from
this kernel discussion and Cell A. The tested fastText, FastFit and TF-IDF
gates injected unwanted cards on real instructions, and transferring real
labels between the two conversations did not make them safe. The user selected
bounded project orientation at session start and explicit agent-invoked
`search`, as in Engram's hook design. Automatic document suggestions are
deferred research, not a condition for the selected v1. A future change to
this decision needs a separately validated gate; do not infer permission to
inject cards from a nonempty FTS5 result.

In an Cell A probe, the current OR-term passage search returned five cards
for each of `Continúa.`, `¿Cuál fue el valor dado?`, `¿Cómo seguimos?`, and
`Sí, pero evita demorarte demasiado.` BM25 top scores were respectively
-5.508, -7.955, -2.6301 and -8.3871 (more negative is better in SQLite).
A score cutoff is not safe: `Gracias por resumir la política de pull requests
hacia main.` is a conversational acknowledgement, yet scored -33.9934, better
than several genuine information questions in the 36-case regression set
(their top scores ranged from -42.4033 to -13.4778). A title/path overlap gate
rejected all 15 handpicked nonqueries, but also rejected **12 of 36** useful
source-derived questions. Both probes are diagnostic, not a held-out estimate
of production accuracy.

I also tested `github.com/tsawler/prose/v3` v3.0.0-beta2, a pure-Go library
advertising Spanish POS tagging. On eight Spanish/English probe messages, it
tagged `Continúa` as a proper noun and tagged nearly every token of `¿Cuál fue
el valor dado?` and `Sí, pero evita demorarte demasiado.` as a noun. Those
outputs cannot reliably distinguish a new information request from a reply;
do not add this dependency to the product. Its multilingual implementation
uses hand-coded language patterns and small n-gram tables, not an intent model.

If automatic suggestions are reconsidered, freeze a labeled set of actual, sanitized
single-turn messages before tuning; include both standalone information needs
and context-dependent acknowledgements, corrections and continuations that
mention domain terms. Run `suggest` over Cell A and another cell, and require
the selected policy to return zero for the low-information turns while retaining
useful source suggestions for standalone questions. Measure false injections,
false suppressions, latency and memory on target hardware. A failed gate blocks
that optional feature; it must not be hidden by returning the top five matches
anyway. No particular word such as `continúa` is the decision rule.

Validate the chosen passage FTS5 on new, held-out Cell A turns and at least
one smaller cell; include exact
names, Spanish paraphrases, false friends, generic follow-ups, confirmation,
and genuinely answerable short turns. Label useful *source groups*, not only
one expected file. Compare source recall/precision at 3, 5 and 8 cards, human
usefulness, unsupported-answer rate, source opening, injected tokens and
end-to-end latency. Existing source-derived 18-question sets are regression
checks only.

The earlier file-level same-corpus PoC showed complementarity: across four 18-question
model/set combinations, FTS5 top-5 found the labeled path 15/18 in each;
vector found 13–17/18; the union of their top-5 found 17–18/18. Five-card RRF
found 15–17/18 and simple interleaving 14–17/18. Thus no rank-fusion rule or
five-card budget earned production status. The subsequent passage experiment
matched the Bekko hybrid at five and ten candidates, so v1 uses BM25 directly
without rank fusion or a cross-encoder. Revisit only against held-out answer
quality and resource measurements. Avoid tuning on the existing development
questions.

The abandoned every-turn search design would have needed `suggest` to return
**zero cards**. In an Cell A probe, `Sí.` yielded no FTS5 hit but a vector
top score of 0.137; `Continúa.` yielded FTS5 hits and vector top 0.223;
`¿Cuál fue el valor dado?` yielded both and vector top 0.282. Topical turns
sampled scored 0.50–0.72. These seven handpicked probes do **not** establish a
safe threshold. If this feature is reconsidered, evaluate the no-card decision
separately, based on whether the turn supplies an independent search subject
and whether candidates supply evidence. Do not encode specific words such as
`continúa` as the rule. The selected explicit-search path allows the agent to
consult sources for a dependent follow-up or prior commitment.

The earlier two-process hybrid wrapper took about 0.62–0.91 seconds per
unchanged E5 invocation and 0.80–0.85 seconds for Bekko; its lexical-only run
took about 152 ms. Running embeddings on every message has a real latency and
RAM cost. For any future `suggest` experiment, measure it end to end,
including no-card turns, rather than extrapolating from SQLite query time.

The first single-process Go `search` prototype was measured with file-level
FTS5 at 144 ms lexical-only and 810 ms Bekko hybrid. After switching the
lexical unit to passages, the medians were 152 ms and 863 ms. A single process
did not eliminate model startup cost. That prototype supported explicit search
only; its semantic mode returned cards for `Sí.` and therefore was not safe to
wire to a message hook.

## One-file packaging feasibility

The deleted Go PoC was **not** one-file portable: it linked a tokenizer static
library at build time, used CGO SQLite, and loaded an ONNX Runtime dylib plus
model/tokenizer files at runtime. On macOS ARM, its binary is 28 MiB, the
runtime dylib 32 MiB, and the quantized E5 model/tokenizer about 129 MiB before
packaging. Bekko model/tokenizer is about 157 MiB. Warm vector search loaded
roughly 685–715 MiB RSS in the PoC. These are local observations, not targets
or Windows/Linux measurements.

A literal hybrid one-file release would have to embed or safely supply the
native runtime and model resources. Runtime extraction to a private cache
would still make extra on-disk files; the per-cell SQLite index is necessarily
another file. V1 avoids that package entirely: one pure-Go executable per
target plus its derived local SQLite state. An optional future semantic build
would need separate quality, resource and redistribution-license approval.

Official implementation references: [SQLite FTS5 ranking and snippets](https://sqlite.org/fts5.html),
[sqlite-vec vector functions](https://github.com/asg017/sqlite-vec),
[multilingual Go NLP candidate evaluated above](https://github.com/tsawler/prose),
[pure-Go SQLite driver](https://gitlab.com/cznic/sqlite),
[go-sqlite3 CGO/FTS5 build tags](https://github.com/mattn/go-sqlite3),
[Go tokenizer static-library requirement](https://github.com/daulet/tokenizers),
[Go ONNX Runtime shared-library requirement](https://github.com/yalue/onnxruntime_go),
[ONNX Runtime deployment](https://onnxruntime.ai/docs/get-started/with-c.html).

## Migration gates before hooks

1. **Search binary:** one pure-Go process owns per-file refresh, persistent
   passage FTS5, deduplication, card formatting and JSON errors; prove
   insert/edit/delete/rename, private scope, interrupted refresh, simultaneous
   invocations and stale-source detection. Validate performance on
   Mac/Windows/Linux including an 8 GiB-class machine.
2. **Quality validation:** test the selected five-card default and ten-card
   explicit expansion on held-out real queries and agent answers. Count
   irrelevant sources, missed useful sources and unsupported claims. Check
   that `context` stays bounded and directs the agent to explicit search when
   evidence is needed. Do not claim token savings from query timing alone.
3. **Runtime migration:** inventory every Python command's arguments, JSON,
   error codes and side effects; port related command families behind the same
   CLI and test old/new parity. Update `README`, `MANAGED_PATHS`, installer,
   kernel references and skills only with a tested replacement available.
   `init`, `adopt`, `update`, `doctor --strict`, source resolution,
   investigation and sync paths need real cell fixtures. Remove installed
   Python only after those tests pass; keep `evals` as development tools.
4. **Release:** produce signed/checksummed binaries per target OS/architecture;
   test clean installation/update and a clean consumer vault on each platform.
   Then select per-project harness events and opt-in adapter wiring. Hook
   failures must not prevent an explicit user task from proceeding.

This order delivers one stable interface without treating a successful search
PoC as evidence that the whole Python runtime has already been replaced.
