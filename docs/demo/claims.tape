Output ../assets/demo-claims.gif
Source prelude.tape
Type "demo-stage discovered; clear" Enter
Sleep 1s
Show

Type "# The agent's draft answer" Enter
Type "cat .scratch/answer.md" Sleep 300ms Enter
Sleep 2.5s
Type "# Before delivering it, check every resource it names" Enter
Type "kos discover claims --file .scratch/answer.md | jq ." Sleep 300ms Enter
Sleep 5s
Type "# refund-requested exists in no repository, snapshot or note: confirm it or drop it" Enter
Sleep 3s
