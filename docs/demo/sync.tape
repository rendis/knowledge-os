Output ../assets/demo-sync.gif
Source prelude.tape
Type "demo-stage discovered; clear" Enter
Sleep 1s
Show

Type "# Knowledge changes travel on a sync branch" Enter
Type "kos sync start --name order-events | jq -c '{branch}'" Sleep 300ms Enter
Sleep 2s
Hide
Type "demo-stage topic-note; clear" Enter
Sleep 500ms
Show
Type "# The agent wrote a topic note; review is bound to the exact content" Enter
Type "git add 25-Topics && git commit -qm 'docs(topics): order-events'" Sleep 300ms Enter
Sleep 1s
Type "kos sync review --verdict accept --reviewer evidence-reviewer | jq -c '{recorded, files}'" Sleep 300ms Enter
Sleep 2s
Type "kos sync finish 2>/dev/null | jq -c '{ok, new_structural_issues}'" Sleep 300ms Enter
Sleep 4s

Ctrl+L
Type "# The gate caught an orphan note: link it from its system and review again" Enter
Type "echo 'Publishes [[order-events]].' >> 10-Sistemas/Orders.md && git commit -qam 'docs: link the topic'" Sleep 300ms Enter
Sleep 1s
Type "kos sync review --verdict accept --reviewer evidence-reviewer | jq -c '{recorded, files}'" Sleep 300ms Enter
Sleep 2s
Type "kos sync finish | jq -c ." Sleep 300ms Enter
Sleep 5s
