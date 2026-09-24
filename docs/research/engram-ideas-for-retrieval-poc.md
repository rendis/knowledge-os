# Engram patterns to evaluate in the retrieval PoC

Status: **historical retrieval research**, reconciled 2026-09-24. The current
CLI uses explicit FTS5 search; session storage and hooks discussed below were
not adopted. See [the current runtime contract](../../README.md#native-cli).
The original proposal and measurements remain in
[the historical decision record](hybrid-retrieval-cli.md).

## Scope and boundary

Use Engram as a design reference, not as a dependency or a component to install. The vault's Markdown remains the authority for documentation. The local SQLite index is a disposable projection of eligible Markdown. The proposed local session store was not implemented. Neither search results nor agent memories establish a claim until the agent reads the cited source.

Engram's own database has a different role: it is authoritative for curated agent observations, user prompts, and sessions. Its Obsidian export writes Engram observations into Markdown; it does not index an existing vault. These boundaries matter when adapting the design. [Engram schema](https://github.com/Gentleman-Programming/engram/blob/main/DOCS.md#database-schema), [memory core](https://github.com/Gentleman-Programming/engram/blob/main/docs/codebase/memory-core.md), [Obsidian export](https://github.com/Gentleman-Programming/engram/blob/main/docs/beta/obsidian-brain.md#how-it-works).

## Patterns worth evaluating

| Observed Engram practice | Possible PoC adaptation | What to verify |
| --- | --- | --- |
| FTS5 over distinct fields, with weighted BM25: title 5, content 1, topic key 3. Other indexed metadata fields have zero BM25 weight. [Source](https://github.com/Gentleman-Programming/engram/blob/main/internal/store/store.go#L4428-L4471) | Index document title, aliases, heading, and passage text as separate fields. Start with BM25 and tune field weights on labeled vault queries; do not copy Engram's weights without evaluation. | Top-k relevance by query type; whether title or heading matches crowd out better body passages. |
| External-content FTS5 table synchronized to the underlying rows. [Source](https://github.com/Gentleman-Programming/engram/blob/main/internal/store/store.go#L1090-L1105) | Keep a reconstructible `documents`/`passages` projection with stable path and section identity. Rebuild or incrementally update from Markdown changes and deletions. | No stale hits after rename, edit, removal, exclusion changes, or index rebuild. SQLite documents external-content consistency risks. [SQLite reference](https://www.sqlite.org/fts5.html#external_content_tables). |
| Trigram tokenizer for substring matching; short query terms use an escaped `LIKE` fallback. [Source](https://github.com/Gentleman-Programming/engram/blob/main/internal/store/store.go#L4473-L4543) | Compare `unicode61` and `trigram` on actual Spanish queries, accents, identifiers, and short terms. Treat short or common words as an input-quality case, not as automatic evidence of relevance. | Recall, noise, size, and latency; trigram is lexical substring matching, not semantic matching. SQLite notes its three-character limit and accent option. [SQLite reference](https://www.sqlite.org/fts5.html#the_trigram_tokenizer). |
| Search supports `all` and `any` match modes and an explicit project filter. [Source](https://github.com/Gentleman-Programming/engram/blob/main/internal/store/store.go#L4131-L4227) | Try a precise initial query, then broaden only when needed. Always bind the eligible vault/project explicitly. | Broadening improves recall without flooding the agent; results never cross the project or excluded paths. |
| Search can return bounded previews before full content; a later call retrieves the full observation. [Source](https://github.com/Gentleman-Programming/engram/blob/main/internal/store/store.go#L4281-L4309), [architecture](https://github.com/Gentleman-Programming/engram/blob/main/docs/ARCHITECTURE.md#progressive-disclosure-3-layer-pattern) | Return at most a few cards: origin class, vault, title, path, heading, short **match-near** passage, and why it matched. Open the original Markdown only for selected cards. | Token volume, usefulness of snippets, and whether the agent actually reads a source before asserting a fact. Engram's preview takes the first 300 characters, so the PoC should specifically test a match-near excerpt instead. |
| Scope and lifecycle are explicit: project scope, topic updates, deduplication, review dates, and deleted records hidden from search. [Source](https://github.com/Gentleman-Programming/engram/blob/main/docs/ARCHITECTURE.md#memory-hygiene) | For any *session memory* experiment, attach project/session identity and distinguish active, superseded, and uncertain observations. Keep this separate from the document index and from authoritative investigation records. | Whether recalled context is fresh, correctly scoped, and supported by source links. Do not infer truth from repetition or recency. |

## PoC output contract to consider

A result card should contain `vault`, `path`, `title`, `heading`, `origin_class`, `excerpt`, and optionally `score` and `match_reason`. `origin_class` must distinguish, at minimum, a published investigation from project documentation. Do not label a document *canonical* based only on its directory. Scores order candidates; they do not express probability that the content is true. A downstream answer cites the Markdown path and relevant section, not the SQLite row.

The search index should honor the vault's current eligible paths, including `investigations/` and excluding the private `.investigations/` path where applicable. Confirm those rules against the actual consumer vault before evaluating results. Avoid indexing credentials or private local state.

## Evaluation sequence for the main session

1. Freeze a small labeled query set: exact terminology, synonyms, Spanish accent variants, identifiers, Git workflow queries, broad questions, common-word queries, and questions with no relevant document. Include cases where a published investigation competes with current documentation.
2. Record a baseline with the current PoC: top-3/top-5 relevant result coverage, wrong-origin cards, empty-result behavior, p50/p95 latency, index build/update time, database size, and tokens in returned cards. Mark queries and expected documents before tuning.
3. Change one retrieval variable at a time: fielded BM25, tokenizer, query broadening, passage segmentation, then previews. Compare against the same queries. Preserve a holdout set to catch overfitting.
4. Test source freshness after edit, rename, deletion, and path-exclusion changes. Test that the agent opens the selected Markdown and does not answer from a card alone.
5. Adopt only changes that improve retrieval or token use without increasing unsupported answers or provenance errors. Keep the `rg` fallback independently usable if the index is unavailable.

## Ideas to defer unless the PoC shows a gap

- Embeddings or an LLM reranker: discarded for the selected product. Reconsider only on new held-out evidence of a material lexical-search gap and a resource case. Engram's shown search path uses FTS5/BM25; its presence of embedding columns does not establish that its documented `mem_search` performs semantic retrieval. [Search implementation](https://github.com/Gentleman-Programming/engram/blob/main/internal/store/store.go#L4131-L4227).
- Engram-style pinned/recency boosts: those help recall favored or recent memories. For vault documents they could bury a better source. Test a source-type or freshness policy separately from lexical relevance.
- Full conversational memory in the document index: user and assistant turns serve session-context routing, while Markdown passages serve knowledge retrieval. Keep their retention and trust rules distinct.

## Reference map

- [Engram repository](https://github.com/Gentleman-Programming/engram)
- [Engram SQLite schema and search interface](https://github.com/Gentleman-Programming/engram/blob/main/DOCS.md#database-schema)
- [Engram search SQL and BM25 ranking](https://github.com/Gentleman-Programming/engram/blob/main/internal/store/store.go#L4428-L4471)
- [Engram FTS5 schema](https://github.com/Gentleman-Programming/engram/blob/main/internal/store/store.go#L1090-L1151)
- [Engram progressive retrieval](https://github.com/Gentleman-Programming/engram/blob/main/docs/ARCHITECTURE.md#progressive-disclosure-3-layer-pattern)
- [Engram's one-way Obsidian export](https://github.com/Gentleman-Programming/engram/blob/main/docs/beta/obsidian-brain.md#how-it-works)
- [SQLite FTS5: BM25, tokenizers, snippets, and external content](https://www.sqlite.org/fts5.html)
