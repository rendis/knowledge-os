---
name: obsidian-cli
description: "Trigger: read, search, backlinks, unresolved, or orphans through the Obsidian CLI."
---

# Obsidian CLI

Target the vault explicitly: `obsidian "vault=<name>" <command>`. Never rely on the focused vault.

Use `90-Meta/check-obsidian-binding.py` before native unresolved/orphans checks. If the CLI cannot connect, fall back to `90-Meta/verify-links.py` and report that native checks were not observed.


Details: [references/guide.md](references/guide.md)
