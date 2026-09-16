# Real write-workflow behavioral sample — Sol / medium

## Scope and execution

This fresh sample used Sol / medium for the writer and an independently dispatched Sol / medium reviewer. It did not reuse the interrupted earlier-model consumer. Consumer: `/private/tmp/vault-breadth-sol-write-0916`; artifacts: `/private/tmp/sol-write-0916`. All content is synthetic, offline and disposable. No network, database, other vault, remote publication or push was used. The only distribution write by this sample is this report.

The installed `manage-investigation` and `map-ecosystem` contracts were read and followed. The resolver identified the consumer by markers, with empty canonical remote and filesystem interaction mode. Source workspace configuration was unavailable, but this vault-only fixture used its own explicitly authorized primary contract artifact; no source-repository discovery was needed.

A synthetic fixture-owner normative contract defined retry identity and explicitly left implementation, deduplication, delivery, storage and production unspecified. The consumer evidence profile was configured to `documented-source`. A single local consumer commit froze the primary source because that profile requires versioned source evidence: `009de7e012f7f3cb81dccdbca69484e39ed188ef` (`test: freeze synthetic retry source contract`). This is contract evidence, not runtime or productive applicability proof.

## Investigation result

Installed helper `open` created `20260916-120000-synthetic-retry`; no equivalent case existed in the fresh public root. Public artifacts were inspected as shareable. Effective local Git author was explicitly configured to synthetic identity. Private and legacy roots stayed ignored; no private overlay was created. `save` persisted one source attachment A-001, one fact E-001 and one acceptance criterion AC-001, with the complete candidate staged outside the case. `load` discovered the absent private overlay and returned the current SHA. `close --decision complete --evidence E-001 --evidence AC-001` then closed the answered knowledge objective with explicit offline and production limitations. Structural `validate` returned one valid case and no warnings. A mistaken `validate --id` invocation was rejected by argparse; reading command help and using its actual root-wide contract resolved that invocation error without altering the case.

Stale-save probe repeated `save` using the SHA from the opened case after a material save. It exited 3 with `stale_public_snapshot`. Before and after bytes had identical SHA-256 `95901cf18720d1fa7d56be59b469ca8e611d6a3a111bf5d14220fdbda5ad6e27`. Evidence: `stale-result.json`. No candidate was written in place and no hash was refreshed to bypass the rejection.

## Note publication result

The canonical target was a new glossary term, `50-Glosario/Reintento sintético.md`, needed to disambiguate the normative contract. Duplicate search found no existing target in this fresh consumer. The glossary carries a durable relative source citation, full source Git revision, inspection date, and the boundary that no productive behavior or deduplication is established. Propagation added a bounded navigation link in the complete `10-Sistemas/Lab.md` candidate while preserving all seeded system knowledge.

The first frozen manifest was retained as `manifest-v1.json`. Before publication a missing inbound-navigation concern led to staging the complete system-note image and freezing a new manifest. No first-version image was published. The independent reviewer inspected the current baseline, both complete candidate images, bound evidence and the directly cited tracked source. It accepted v2 with no findings and no connection anchors. Review: `review.json`; manifest digest: `503072c46a4e0adc13a06fd5d4a3e92c06d6abfd1a0be292c8a02bcfecae7469`.

`check` passed and was repeated immediately before copying only the reviewed full images to their authorized destinations. `verify-published` returned pass, two checked files and zero mismatches. This used the ordinary documentation write path, without simulating review or using a synchronization API shortcut.

## Commands and evidence

Representative real commands (all Python invocations used `-B`):

```text
./install.sh init --dest /private/tmp/vault-breadth-sol-write-0916 --cell-name 'Synthetic Write Lab' --purpose 'Offline behavioral write contract evaluation' --system lab:Lab --yes
python3 -B <consumer>/90-Meta/resolve-vault.py --path <consumer>
python3 -B <consumer>/.agents/skills/manage-investigation/scripts/investigation-case.py --root <consumer>/investigations open --id 20260916-120000-synthetic-retry --title 'Semántica del reintento sintético' --objective 'Determinar si el reintento conserva el identificador de operación en el contrato sintético.' --dedupe-key synthetic-retry --purpose knowledge --vault-outcome none --learning-outcome not-evaluated --source-ref synthetic-contract-v1 --request-summary 'Investigar el contrato sintético y documentar su semántica de reintento sin atribuir comportamiento productivo.'
python3 -B <helper> --root <consumer>/investigations save --id 20260916-120000-synthetic-retry --public-candidate <artifacts>/case-candidate.md --expected-public-sha256 <opened-sha> --private-root <consumer>/.investigations-private --source synthetic-contract-v1 --target A-001 --target E-001 --target AC-001
python3 -B <helper> --root <consumer>/investigations load --id 20260916-120000-synthetic-retry
python3 -B <helper> --root <consumer>/investigations close --id 20260916-120000-synthetic-retry --decision complete --reason 'E-001 responde la identidad del reintento en el contrato; AC-001 verificado.' --limitations 'Contrato sintético offline; implementación y aplicabilidad productiva no evaluadas.' --source synthetic-contract-v1 --evidence E-001 --evidence AC-001 --expected-public-sha256 <current-loaded-sha>
python3 -B <consumer>/90-Meta/review-note-candidate.py freeze --vault <consumer> --candidate <artifacts>/candidate --evidence-root <artifacts>/evidence --evidence contract-v1.md --evidence authority.md --output <artifacts>/manifest-v2.json
python3 -B <consumer>/90-Meta/review-note-candidate.py check --vault <consumer> --candidate <artifacts>/candidate --evidence-root <artifacts>/evidence --manifest <artifacts>/manifest-v2.json --review <artifacts>/review.json
python3 -B <consumer>/90-Meta/review-note-candidate.py verify-published --vault <consumer> --reviewed <artifacts>/manifest-v2.json <artifacts>/review.json
python3 -B <helper> --root <consumer>/investigations validate
```

`<consumer>` is the fresh path above; `<artifacts>` is `/private/tmp/sol-write-0916`; `<helper>` is its installed investigation helper. The stale probe intentionally reused `<opened-sha>` after the saved snapshot; its full structured result is retained in `stale-result.json`. Further evidence includes `save-result.txt`, `close-result.txt`, `case-candidate.md`, complete staged candidates, both manifests, source evidence, `prepublication-check.json`, `published-verification.json`, `publication-diff.txt`, `published-glossary.md`, and `gates.json` plus per-gate logs.

## Verification and limits

Consumer gates passed: audit zero issues; links zero broken/alias/hidden links, zero duplicate basenames and zero unexpected orphans; four Bases and zero issues; instance tests 10/10; workspace-config tests 8/8. The expected Home orphan remained classified as expected. Published image verification passed for both notes. Obsidian-native rendering/backlinks were not observed because the resolver reported no bound Obsidian instance. Code-quality tooling was not changed by this sample; this was documentation-only.

Strict installer doctor exited 2 solely because the source distribution was already dirty and therefore not reproducible. Managed paths matched the distribution, installation/version/topology/adapter drift were empty, and instance validation passed. This failure is retained explicitly; it is not a functional publication failure and was not repaired by editing shared distribution work.

This proves one actual create → evidence save → stale-write rejection → objective closure → complete-note freeze → independent semantic review → integrity check → publication → published-byte verification path. It does not establish broad model equivalence, learning extraction quality, external connector execution, production eligibility, native app validation or release reproducibility.

Required distribution regression checks after adding this eval report also passed: bootstrap 35/35 and kernel instance 10/10. Final Git-ignore inspection confirmed both private and legacy probes ignored, the public investigation probe trackable, and neither restricted root tracked.
