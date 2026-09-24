---
name: obsidian-cli
description: Interact with the Obsidian application, query Base views, or inspect native link resolution, rendering and plugins. Use for app-specific work or an explicit request to use Obsidian; ordinary kernel retrieval uses vaultctl.
---

# Obsidian CLI

Use this skill when the task requires the application: opening notes, querying a Base view, checking native link resolution, or inspecting rendering and plugins. For indexed vault search and kernel state operations, follow `use-vault-cli`.

Resolve the vault through `vaultctl`, then validate the selected Obsidian registration with `<VAULTCTL> check obsidian-binding --vault "<VAULT_ROOT>" --vault-name "<OBSIDIAN_VAULT>"` before application queries. `<VAULTCTL>` is the installed executable identified by `use-vault-cli`.

Target every application command explicitly: `obsidian "vault=<OBSIDIAN_VAULT>" <command>`. Apply that prefix to the guide's examples; a focused note or vault is not an identity binding.

If the application cannot be reached, use `<VAULTCTL> check links --vault "<VAULT_ROOT>"` for filesystem link checks when relevant, and report app-specific behavior as unverified. This fallback does not evaluate Base expressions, rendering or plugins. Keep mutations within the current workflow's authority.

Consult [the command guide](references/guide.md) for application operations and `obsidian help` for the installed command syntax.
