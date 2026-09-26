# Shared look (the flow guide's dark palette) and off-camera shell setup for every recording. Needs DEMO (the demo directory).
Set Shell "bash"
Set FontFamily "Menlo"
Set FontSize 17
Set Width 1280
Set Height 560
Set Padding 26
Set WindowBar Colorful
Set BorderRadius 10
Set TypingSpeed 25ms
Set Theme { "name": "knowledge-os", "background": "#12151e", "foreground": "#e7eaf3", "cursor": "#5ce1e6", "selection": "#232b4a", "black": "#2e3546", "red": "#ff7a6d", "green": "#5ce1e6", "yellow": "#ffc24b", "blue": "#7a91ff", "magenta": "#8ea2ff", "cyan": "#5ce1e6", "white": "#c9cedb", "brightBlack": "#7c869c", "brightRed": "#ff9d93", "brightGreen": "#8cebef", "brightYellow": "#ffd37a", "brightBlue": "#a3b3ff", "brightMagenta": "#b8c4ff", "brightCyan": "#8cebef", "brightWhite": "#ffffff" }

Hide
Type `set +m; export PS1='\[\e[34m\]acme-vault\[\e[0m\] \$ ' PATH="$DEMO/bin:$PATH" KOS_NO_UPDATE_CHECK=1 GIT_AUTHOR_NAME=demo GIT_AUTHOR_EMAIL=demo@example.com GIT_COMMITTER_NAME=demo GIT_COMMITTER_EMAIL=demo@example.com; cd "$DEMO/acme-vault"; clear`
Enter
Sleep 300ms
