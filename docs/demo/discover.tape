Output ../assets/demo-discover.gif
Source prelude.tape
Show

Type "# Scan every repository of the cell at its reference branch" Enter
Type "kos discover run | jq '{scanned, pending_questions}'" Sleep 300ms Enter
Sleep 3s
Type "# The agent answers the judgments discovery could not make alone" Enter
Type "kos discover answer --file .scratch/answers.json | jq -c ." Sleep 300ms Enter
Sleep 2.5s

Ctrl+L
Type "# Read the cloud: names and relations only, with your own login" Enter
Type "kos discover platform --provider gcp --scope acme-prd | jq -c .captured" Sleep 300ms Enter
Sleep 2s
Type "kos discover run --classify off | jq -c '{pending_items}'" Sleep 300ms Enter
Sleep 2s
Type "# Every connection keeps its evidence" Enter
Type "kos discover report --repo acme-ledger | jq '.facts.resources[]|{name, via: [.evidence[].kind]}'" Sleep 300ms Enter
Sleep 5s
