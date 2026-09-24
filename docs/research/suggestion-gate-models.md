# Learned gate for proactive vault suggestions: model and data review

Status: **research closed for the selected v1; candidate models rejected**,
2026-09-24. This experiment evaluated `suggest` on the **current user
message**; the user subsequently chose explicit `search` instead. Session
hooks and message capture were not adopted; the current contract is in
[README — Native CLI](../../README.md#native-cli). A positive test label meant the message had a
recoverable subject and a present request for information. It does not prove
that the vault contains an answer. Retrieval must still be able to return
`cards: []`. The agent may use conversation context for its broader task, but
that context is not silently joined into this proactive-message classifier.

The standard problem is **selective retrieval / retrieval routing** for a
conversation turn, with a **context-dependence** check. It has two decisions:
whether this message independently expresses a useful information need, and
whether the returned document actually helps. Research on
[adaptive retrieval](https://aclanthology.org/2024.naacl-long.389/) and
[conversational question rewriting](https://github.com/apple/ml-qrecc)
studies related decisions, but neither supplies the exact label “show a
document from this particular vault now.” A classifier can reject a message
before retrieval; BM25 or a result-quality test must still be evaluated after
retrieval. Model confidence is not a correctness guarantee.

## Decision

**Do not ship the tested fastText, FastFit, or TF-IDF models as a message-hook
gate.** They produced unwanted suggestions on held-out real turns. The
selected v1 has bounded session orientation and agent-invoked, ungated
`search`; it does not include `suggest`. Do not infer a safe cutoff from the
classifier's probability or from BM25. This rules out the current models and
labels, not all future learned gates. SetFit remains a research comparator,
not a fallback already proven for this task.

| Method | Published evidence | Product interpretation |
| --- | --- | --- |
| [FastFit](https://aclanthology.org/2024.naacl-demo.18.pdf) | On multilingual MASSIVE's 60 intents, Spanish accuracy with its smaller backbone was 65.9% at 5 examples **per intent** and 71.7% at 10, versus SetFit 64.0% and 71.4%. The authors report 3–20x shorter training than their SetFit setup. | Strongest documented few-shot challenger, but 5 examples per intent means about 300 examples for that benchmark, not 5 total or our two-class gate. The multilingual backbone is much larger than FTS5. |
| [SetFit](https://github.com/huggingface/setfit) | Its maintainers report 8 examples per class competitive with a fully trained large model on one sentiment dataset; it accepts multilingual sentence-transformer backbones. | Research comparator after obtaining real labels. The eight-shot sentiment result is not a vault-routing result. |
| [fastText](https://fasttext.cc/docs/en/supervised-tutorial.html) | Mature supervised classifier with optional [quantization](https://fasttext.cc/docs/en/quantization.html). | Very small runtime candidate, but requires labels for our actual task. |
| [GLiClass Multilang](https://docs.knowledgator.com/docs/models/text-classification/gliclass/) | Supports few-shot examples. The developer lists ~140M parameters and multilingual zero-shot F1 0.3959 for `edge`, ~288M and 0.5378 for `mini`, averaged over six other datasets. | Too large and too weakly evidenced for the first low-RAM trial. The reported GPU throughput is not our CPU latency. |
| [ALBETO base](https://github.com/dccuchile/lightweight-spanish-language-models) | Spanish encoder with 12M parameters and a stronger reported task average than its 5M tiny version (79.35 vs 70.86). | Tested below as the FastFit backbone; the current gate fails real turns. English performance and redistribution license need resolution before any future package. |

FastFit's published advantage mainly concerns **many similar classes**. Our
gate has two classes, so its advantage over SetFit or fastText is a hypothesis,
not a conclusion. GLiClass would need reviewed task labels and a resource case
before another trial, given its model size.
Generic multilingual NLI is not the first training candidate after the local
zero-shot probe below.

## Datasets: useful utterances, mismatched labels

| Source | What it supplies | Why labels cannot be copied |
| --- | --- | --- |
| [MIAM / DIHANA](https://huggingface.co/datasets/PierreColombo/miam) | Spanish conversation turns and dialogue acts. | `Pregunta` includes context-dependent turns and requests whose subject is elsewhere. |
| [QReCC](https://github.com/apple-aiml-research/ml-qrecc) | 14K conversations, 81K questions, originals and independent rewrites. | Some originals are independently searchable. English; dataset CC BY-SA 3.0. |
| [COQAR](https://github.com/Orange-OpenSource/COQAR) | 53K follow-up questions with independent rewrites. | Same original/rewrite issue; English and mixed source-content licenses. |
| [MASSIVE](https://github.com/alexa/massive) | More than 1M voice-assistant utterances in 52 languages, including Spanish. | Its 60 action intents do not mean “offer document evidence.” |

These are **candidate utterance reservoirs after relabeling**, not ready-made
train/test sets. Real, reviewed messages from Cell A and another cell should
be the primary gold data. Keep private messages local unless separately
authorized to export them. An offline teacher can propose labels for review;
it is not the evaluator. Split by conversation and by project to prevent
adjacent turns and paraphrases leaking into the holdout. A noun phrase that
worked as an explicit retrieval query is not automatically a present user
request for a proactive suggestion. No fixed words define the classes.

## Local diagnostic experiments

These are **small exploratory probes, not a blind benchmark**. A 56-message
scratch set combined source-derived retrieval questions and 19 authored
conversational negatives. Two retrieval-positive descriptions should be
negative for this gate, making the corrected task split 35 positive and 21
negative. The set is too small and coupled for a release estimate.

- fastText trained on 7,881 Spanish MIAM user utterances (`Pregunta` versus
  other acts) reached 0.89 in-domain precision at one on 972 MIAM test turns,
  yet wrongly suggested search for **15/19** conversational negatives. The
  model was ~9.7 MiB; one 56-message prediction process took ~10 ms wall time
  and ~12.2 MiB maximum RSS on this Mac. This reveals label/domain mismatch,
  not an intrinsic fastText limit.
- fastText trained on the tiny task-specific set with five-fold splits and
  three seeds still produced 5–7 false injections and 0–1 false suppressions
  per 56 probes. More diverse reviewed data are necessary.
- A generic multilingual MiniLM NLI checkpoint was ~428 MiB ONNX and used
  ~1.15 GiB RSS in a fresh Python process; it frequently treated low-information
  turns as queries. This rejects that zero-shot configuration for low-RAM use,
  not all fine-tuned encoders.
- On sanitized examples only, remote GPT-4.1-mini matched 53/56 corrected
  labels and Claude Haiku 4.5 matched 48/56. Both made errors and took roughly
  1.3–1.5 s median per individual call. Neither is local runtime or truth.

### Iteration with separate conversations

I then fixed a 60-message authored set (30 messages with an independent
documentary subject, 30 without one) and manually labeled two separate real
conversation samples kept local: 24 turns from this kernel discussion (12/12)
and 42 from one Cell A operational conversation (6/36). Those labels are
diagnostic and partly judgment-dependent; neither sample is a representative
production holdout. The Cell A sample deliberately contains many low-information
replies and commands. No raw private conversation is committed here.

| Trained on | Tested on | Unwanted cards on negatives | Missed positives | Resource observation |
| --- | --- | ---: | ---: | --- |
| fastText, 4,800 weak QReCC/SQAC/DIHANA labels | authored 60 | 25/30 | 1/30 | Public labels transfer badly. |
| fastText, authored task labels, 5-fold | authored 60 | 15/30 | 3/30 | 50k subword bucket: ~6.4 MB model, ~0.15 s training. |
| fastText, authored 60 | kernel 24; Cell A 42 | 9/12; 21/36 | 0/12; 0/6 | Compact but noisy. |
| FastFit + 12M-parameter Spanish ALBETO, 5-fold | authored 60 | 1/30 | 1/30 | Attractive on the authored set only. |
| FastFit + ALBETO, authored 60 | kernel 24; Cell A 42 | 6/12; 3/36 | 1/12; 1/6 | Confident errors on real turns. |
| Character TF-IDF + logistic regression, 5-fold | authored 60 | 19/30 | 7/30 | On kernel 24: 5/12 unwanted, 0/12 missed. |
| Word TF-IDF + logistic regression, 5-fold | authored 60 | 17/30 | 8/30 | On kernel 24: 3/12 unwanted, 5/12 missed. |

The exact same FastFit recipe with 24 real kernel labels added to training
still produced 4/36 unwanted cards and 1/6 missed positives on the held-out
Cell A conversation. Reversing the conversations produced 5/12 unwanted cards
and 3/12 misses on kernel turns. Compact fastText transfer was likewise poor:
20/36 unwanted Cell A cards in one direction; 4/12 unwanted and 4/12 missed
kernel turns in the other. This is conversation transfer, not project-wide
cross-validation; there is still too little reviewed data to train a product
gate.

FastFit's exported weights were 47.7 MB. A fresh Python process needed 3.36 s
and reached 529 MB maximum RSS to import, load, and classify four messages on
this Mac; four forward passes after loading took 0.72 s. These are measurements
of this Python stack, **not** proof that an optimized Go/ONNX runtime has the
same cost. `fast-fit` 1.2.1 also failed to import with current `datasets` 5.0.1
because it still calls `load_metric`; the scratch trial required `datasets`
2.19.0 and `transformers` 4.40.1. The ALBETO model card does not state a
redistribution license, so binary distribution would need separate review.

**Decision from these trials:** reject the tested classifiers for automatic
per-message suggestions. They fail on clear instructions such as “ejecutada,
revisa” and “lo dejé en la raíz el .env…”; FastFit assigned some wrong results
near 1.0 probability, so a simple confidence cutoff would still inject noise.
Across the two held-out conversation samples, raising the FastFit acceptance
score from 0.5 to 0.99 still left 7 unwanted cards among 48 negative turns
and increased missed positives from 2/18 to 3/18. This cutoff was inspected
after seeing the results and is only a diagnostic, not a calibrated policy.
The apparent success on authored cross-validation does not generalize to the
two real conversations. Do not train another generic model on weak public
labels and call it solved.

### Selected interaction pattern after the failed gate trials

[Engram's Codex prompt hook](https://github.com/Gentleman-Programming/engram/blob/33337716a842db167a2b66a7fb5d0f7ed2e6461f/plugin/codex/scripts/user-prompt-submit.sh)
captures the prompt and usually returns no content after the first-turn
instructions. Its
[session-start hook](https://github.com/Gentleman-Programming/engram/blob/33337716a842db167a2b66a7fb5d0f7ed2e6461f/plugin/codex/scripts/session-start.sh)
provides bounded project context. It does not search all memory and inject
matching records on every user message. After the failed model trials above,
the user selected this pattern for v1: offer minimal orientation on a new
session, include a bounded checkpoint from the **same session** only on
resume/compaction, and make explicit CLI search available to the agent. A
prompt hook records local session state but does not search or inject document
cards. It does not automatically inject other sessions' recent prompts.
The agent can still search when a dependent follow-up requires evidence.
Automatic document suggestions are deferred research, not a v1 release gate.

## Conditions if automatic suggestions are reconsidered

1. Freeze a short labeling guide, then collect actual, consented messages
   from Cell A and another cell. Cover Spanish/English, topical short
   questions, domain-bearing acknowledgements, corrections, and dependent
   follow-ups. Human-adjudicate ambiguous cases. Keep a project and complete
   conversations untouched for final evaluation.
2. Compare 8, 16, 32 and 64 reviewed examples per class for fastText and
   FastFit, with multiple seeds and identical holdouts. These sizes form an
   experiment grid, not a claim that eight examples suffice. Measure false
   injections and false suppressions separately, then precision, recall and
   calibration.
3. On the best quality candidates, measure cold start, p95 latency, peak RSS,
   packaged bytes and real Mac/Windows/Linux behavior on 8 GiB-class CPUs.
   Weights, tokenizer and native runtime count toward single-CLI packaging.
   Quantize only after measuring full-precision quality; repeat the same test.
4. Run `suggest` end to end: classifier → FTS5 → bounded cards or zero cards.
   Measure source usefulness and whether the agent opens original Markdown.
   A correct gate does not establish answer correctness. No automatic hook
   injection until the full path meets the agreed error/resource budget.

**Current conclusion:** v1 proceeds with bounded session orientation and
explicit FTS5 search. There is no automatic per-message card gate to ship or
block this release. Any future proposal to restore it needs a reviewed label
set from independent conversations and cells and the same resource limits;
published few-shot scores from another task do not override these failures.
