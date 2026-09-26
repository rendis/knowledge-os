Output ../assets/demo-onboard.gif
Source prelude.tape
Type `cd ..; export PS1='\[\e[35m\]~/vaults\[\e[0m\] \$ '; clear` Enter
Sleep 500ms
Show

Type "# kos is installed once per machine; a new cell vault starts with kos init" Enter
Type "kos init --vault payments | jq -c '{status, cell}'" Sleep 300ms Enter
Sleep 1s
Type "Payments" Sleep 400ms Enter
Type "Card checkout and settlement." Sleep 400ms Enter
Type "Checkout, Settlement" Sleep 400ms Enter
Type "acme" Sleep 400ms Enter
Type "pay-" Sleep 400ms Enter
Sleep 600ms Enter
Type "gcp" Sleep 400ms Enter
Sleep 600ms Enter
Type "en" Sleep 400ms Enter
Sleep 3s

Ctrl+L
Type "# The vault is ready for your agent: it finishes onboarding from here" Enter
Type "kos doctor --vault payments | jq '{state, kernel_current, start_here}'" Sleep 300ms Enter
Sleep 2.5s
Type "ls payments" Sleep 300ms Enter
Sleep 5s
