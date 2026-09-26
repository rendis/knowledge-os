Output ../assets/demo-investigation.gif
Source prelude.tape
Type "demo-stage discovered; clear" Enter
Sleep 1s
Show

Type "# One case per line of work" Enter
Type `ID=$(kos investigation new --title "Duplicate ledger entries" --type development --objective "One entry per event." | jq -r .id); echo $ID` Sleep 300ms Enter
Sleep 2.5s
Type "# Every write goes through the CLI, and its gate" Enter
Type "kos investigation add --id $ID --kind evidence --level demonstrated --text 'No idempotency key'" Sleep 300ms Enter
Sleep 3.5s
Type "kos investigation add --id $ID --kind evidence --level demonstrated --text 'No idempotency key' --source '`cmd/main.go@84a6f11`'" Sleep 300ms Enter
Sleep 2.5s

Ctrl+L
Type "# A conclusion names the evidence it rests on" Enter
Type "kos investigation add --id $ID --kind finding --level inferred --from E-001 --text 'A redelivery adds a second entry'" Sleep 300ms Enter
Sleep 2.5s
Type "kos investigation check --id $ID | jq -c '.cases[0] | {status, ok, records}'" Sleep 300ms Enter
Sleep 5s
