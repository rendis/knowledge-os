# Disposable release-artifact validation

## Scope

Mechanical validation of the current candidate, executed by the explicitly selected Sol agent at medium reasoning. No domain behavior samples were generated. The real distribution and real consumers were not committed, updated, or propagated by this validation. This report is the only repository addition from the validation.

The source distribution was dirty. Its installable candidate was frozen in `/private/tmp/vault-release-artifact-0916`, with a fresh local Git repository, synthetic identity, and no remotes. This is a **test commit**, not a distribution release or a commit in the original repository.

## Frozen candidate

- Disposable commit: `b59db580472ba2456485c5e61a103f25ecc2237c`.
- Candidate version: `0.9.0`.
- Source file count: **186**, including **158** kernel managed files for a consumer with no optional adapter.
- Manifest: `/private/tmp/vault-release-manifest-0916.json`, with one SHA-256 per copied source file.
- Manifest SHA-256: `93519adb902d0dd232bb83b56ab36aab9dd3344ea69358ab8e30ffdad217868f`.

The copy includes the complete regular-file kernel payload, templates, skeleton, Bases, current user guide (including ignored files used by the installer), optional adapter files, `install.sh`, `scripts/knowledge_os.py`, both kernel/root versions, `MANAGED_PATHS`, and `instance.schema.yaml`. It excludes the original Git repository, local configuration, evals, and Python caches. The source `kernel/CLAUDE.md` symlink is not an installer input; the installer creates the installed `CLAUDE.md` symlink itself. Its omission does not omit a managed file.

All copied source bytes and permission modes were compared to the original before the test commit. After installation/update, every manifest entry was checked again against both the original and the frozen copy: all bytes still matched. Every installed managed file matched its frozen source byte for byte. The disposable distribution remained clean after the checks.

## Execution and outcome

The installer initialized `/private/tmp/vault-release-installed-0916` using explicit synthetic cell identity and system `fixture:Fixture`. `doctor --strict` exited **0** immediately after init and again after a same-version update. Both runs reported:

- `reproducible_distribution: true`;
- `distribution_dirty_dist: false` and `distribution_dirty_installed: false`;
- matching installed/distribution revision `b59db580472ba2456485c5e61a103f25ecc2237c`;
- `managed_matches_dist: true`;
- empty `drift`, `distribution_drift`, `topology_drift`, and `adapter_configuration_drift`;
- valid instance and ready orientation.

The update exited **0**, reported version `0.9.0`, and retired no files. SHA-256 preservation checks before/after update passed for `instance.yaml`, `00-Home.md`, and `10-Sistemas/Fixture.md`:

| Consumer-owned file | Preserved SHA-256 |
|---|---|
| `instance.yaml` | `93f42b0df44c99cd4a59d26e3d8875d653962c8e544f8ae830fa63e7b6b73f98` |
| `00-Home.md` | `1553c475d839a10ce25e58c70b83a8ba24018a3e37ed32a6a192e966eef51ec9` |
| `10-Sistemas/Fixture.md` | `9fcbb58b362f1b14a537fd8bf9ef49dc6e47504313855f5515f508480b3b77a1` |

Execution script: `/private/tmp/vault-release-snapshot-0916.py`. Logs: `/private/tmp/vault-release-init-0916.log`, `/private/tmp/vault-release-doctor-before-0916.log`, `/private/tmp/vault-release-update-0916.log`, and `/private/tmp/vault-release-doctor-after-0916.log`.

## Limits

This proves that the frozen current installer payload can be recorded in a clean disposable commit, initialized, and updated without mechanical drift or changes to the checked synthetic consumer-owned files. It does not certify the dirty original checkout as a released distribution, propagate anything to real consumers, validate optional-adapter installation, prove future agent behavior, or guarantee future generations. The temporary artifacts are local evidence and are not a published release channel.
