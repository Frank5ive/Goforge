# Records docs/demo.gif for the README with VHS (https://github.com/charmbracelet/vhs).
#
#   go install ./ && vhs docs/demo.tape
#
# VHS needs ttyd and ffmpeg on your PATH. The goforge on your PATH is the one recorded.

Output docs/demo.gif

Require goforge
Require go

Set Shell "bash"
Set FontSize 16
Set Width 1280
Set Height 760
Set Padding 24
Set Theme "Catppuccin Mocha"
Set TypingSpeed 35ms
Env COLORTERM "truecolor"

# Work in a scratch directory
Hide
Type "cd $(mktemp -d) && clear"
Enter
Show

Type "goforge generate --name shop --module github.com/acme/shop --yes"
Sleep 400ms
Enter
Sleep 3s

Type "cd shop && goforge add resource product --fields 'name:string,price:float,in_stock:bool'"
Sleep 400ms
Enter
Sleep 3s

Type "go test ./..."
Sleep 400ms
Enter
Sleep 8s
