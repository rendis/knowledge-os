# Shared look and off-camera shell setup for every recording. Needs DEMO (the demo directory).
Set Shell "bash"
Set FontFamily "Menlo"
Set FontSize 17
Set Width 1280
Set Height 560
Set Padding 26
Set WindowBar Colorful
Set BorderRadius 10
Set TypingSpeed 25ms
Set Theme { "name": "knowledge-os", "background": "#0d1117", "foreground": "#e6edf3", "cursor": "#afa9ec", "selection": "#26215c", "black": "#484f58", "red": "#ff7b72", "green": "#3fb950", "yellow": "#d29922", "blue": "#58a6ff", "magenta": "#afa9ec", "cyan": "#5dcaa5", "white": "#b1bac4", "brightBlack": "#6e7681", "brightRed": "#ffa198", "brightGreen": "#56d364", "brightYellow": "#e3b341", "brightBlue": "#79c0ff", "brightMagenta": "#cecbf6", "brightCyan": "#9fe1cb", "brightWhite": "#ffffff" }

Hide
Type `set +m; export PS1='\[\e[35m\]acme-vault\[\e[0m\] \$ ' PATH="$DEMO/bin:$PATH" KOS_NO_UPDATE_CHECK=1 GIT_AUTHOR_NAME=demo GIT_AUTHOR_EMAIL=demo@example.com GIT_COMMITTER_NAME=demo GIT_COMMITTER_EMAIL=demo@example.com; cd "$DEMO/acme-vault"; clear`
Enter
Sleep 300ms
