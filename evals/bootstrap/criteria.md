# Bootstrap eval criteria

Judges and the automated harness use the same bars.

1. No product leak: names of the cells the distribution was developed with must not appear in the installed kernel or distribution. The check matches hashed terms, so the list itself names no cell.
2. `init` on an empty dest writes `instance.yaml`, `00-Home.md`, system stubs, skills, and a portable lock that survives a clean Git clone and permits `update` there.
3. `adopt` and `update` preserve `instance.yaml`, `00-Home.md`, and all knowledge under `10/`–`70/` byte for byte.
4. Knowledge Markdown without a lock is refused.
5. Adapters install only when requested.
6. After init, `doctor` reports `start_here` beginning with `00-Home.md` and orientation `ready`.
7. The lock hashes every distribution-shipped router, Meta helper, kernel skill, and selected adapter file; cell-owned Bases, Meta extensions, recipes, and skills remain outside its drift boundary.
8. Every local command target named by an installed kernel procedure exists in a fresh cell.
9. The lock reports the exact distribution revision and dirty state used to install or update the cell.
10. The distribution and installed cell contain no retired graph-index reference, dependency, ignore rule, or generated index.
11. A minimal sync instruction points at Home + declared systems, not an empty graph walk.
