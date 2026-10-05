---
name: onboard-cell
description: "Trigger: create a vault for a cell or adopt an existing one, or change the cell's shared setup: identity, systems, repositories, reference branches, clouds, trackers, database targets or capability procedures; the first inventory and discovery of a new vault."
---

# Onboard a cell

The cell's setup is shared and versioned in `instance.yaml`: who the cell is, where its code lives, where it runs and how it is operated. A person's machine belongs to [onboard-developer](../onboard-developer/SKILL.md).

**Propose from evidence, then confirm.** Draft what you can infer, show it, ask only what is left: three blocks, one message each. The kernel has no default company, environment list, credential mechanism or procedure name.

## 1. Who

Name, purpose (1–3 sentences) and systems, drafted from what the user said. Notes are written in the user's language unless they choose another (`es` or `en`).

## 2. Where the code is and where it runs

- **Repositories.** The organization and the name prefixes. With `gh` logged in, list them (`gh repo list <org> --limit 1000 --json name,isArchived`, filtered by the prefixes) and show the count, a few names and the archived ones.
- **Reference branches.** Check which of `main`, `master` and `develop` exist in a sample of up to 20 of those repositories (`git ls-remote --heads <url> main master develop`) and propose the order that covers most; name the exceptions for `sources.reference_branches` ([reference-branches](../../../90-Meta/reference-branches.md)).
- **Clouds.** Propose from the repositories' infrastructure code (Terraform `google`, `aws` or `azurerm` providers; `gcloud`, `aws` or `az` in pipelines): one, several or none.

Done when the user confirmed the organization, prefixes, branch order and clouds.

## 3. How it is operated (optional)

Issue tracker URLs; database targets (system, environment, instance, database, access procedure, optional local port key, no credentials); capability procedures (`<CLI> config bind --capability ID --procedure BASENAME`). Each may be deferred; deferred means unconfigured.

## 4. Install

- **New vault:** `<CLI> init --vault "<vault>" --yes --cell-name "…" --purpose "…" --system "Name" … --github-org … --repo-prefix … --reference-branch … --platform … [--tracker URL] --locale …`.
- **Existing notes without the kernel:** `<CLI> adopt --vault "<vault>"`.
- **An installed vault:** change `instance.yaml` on a sync branch ([synchronize-ecosystem](../synchronize-ecosystem/SKILL.md)) and check it with `<CLI> config status`.

Then onboard yourself on this machine ([onboard-developer](../onboard-developer/SKILL.md)).

## 5. First reading

On a sync branch: `<CLI> inventory`, `<CLI> discover run` (the agent answers the judgments it leaves with `discover questions` and `discover answer`) and, with the user's authorization, `<CLI> discover platform --referenced`. Commit the discovery state, `sync verify`, and publish per the team's Git policy. When `discover run` references scopes of a cloud missing from `platform.providers`, propose adding it.

## 6. Readiness report

In a few lines: repositories found and missing, repositories without a note (`inventory` `new`), clouds and services readable or not, services no provider reads (`platform-unmanaged`), credentials versioned in the sources (`credentials_in_sources`: repository, file and key; their owners decide), and what the vault can answer now. Propose the first sync for the systems that matter most. Teammates only open the vault: onboard-developer runs in their first session.

Done when the report is delivered and every deferred item is named.
