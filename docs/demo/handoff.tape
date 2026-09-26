Output ../assets/demo-handoff.gif
Source prelude.tape
Type `demo-stage discovered; demo-stage case; ID=$(cat ../case-id); CASE=.investigations/$ID; WT=../worktrees/acme-ledger/idempotent-entries; clear` Enter
Sleep 1s
Show

Type "# The case packages an atomic task for the ledger repository" Enter
Type "kos investigation add --id $ID --kind handoff --text 'Idempotent entries' --package handoffs/DH-001.md | jq -c '{ok, record}'" Sleep 300ms Enter
Sleep 2s
Type "kos handoff start --package $CASE/handoffs/DH-001.md --apply | jq '{branch, effects: [.effects[].kind]}'" Sleep 300ms Enter
Sleep 4s

Hide
Type "demo-stage handoff-work; clear" Enter
Sleep 500ms
Show
Type "# Any harness works the task in the worktree; kos reads its progress" Enter
Type "kos handoff status --worktree $WT | jq '{commits_since_base, deltas: [.deltas[].title]}'" Sleep 300ms Enter
Sleep 3s
Type "# and reconciles it back into the case as graded evidence" Enter
Type "kos handoff reconcile --worktree $WT --apply | jq -c '{records, through}'" Sleep 300ms Enter
Sleep 1.5s
Type "grep '^- \*\*E-' $CASE/investigation.md" Sleep 300ms Enter
Sleep 6s
