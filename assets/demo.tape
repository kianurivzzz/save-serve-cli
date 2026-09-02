Output assets/demo.gif

Require sv
Require sh

Set Shell "bash"
Set FontSize 18
Set Width 1200
Set Height 600
Set Padding 24
Set Theme "Catppuccin Mocha"
Set TypingSpeed 40ms

Hide
Type "eval $(sh assets/demo-env.sh) && clear"
Enter
Sleep 1s
Show

Type "sv ls"
Sleep 500ms
Enter
Sleep 3s

Type "sv add api root@10.0.0.7 --group work --tag prod"
Sleep 500ms
Enter
Sleep 2.5s

Type "sv"
Sleep 500ms
Enter
Sleep 2s
Type@120ms "cash"
Sleep 2s
Backspace 4
Sleep 1s
Tab
Sleep 1.5s
Tab
Sleep 1.5s
Tab
Sleep 1s
Escape
Sleep 1s

Type "tail -7 ~/.ssh/config"
Sleep 500ms
Enter
Sleep 4s
