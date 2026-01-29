# claude-perms CLI demo
Output docs/demo/demo.gif

Set Shell "bash"
Set FontSize 14
Set Width 1100
Set Height 650
Set Padding 15
Set Theme "Catppuccin Mocha"
Set TypingSpeed 40ms

# Build and launch silently
Hide
Type "cd /Users/satchmo/code/claude-perms && go build -o bin/perms ./cmd/perms/"
Enter
Sleep 2s
Show

# Launch the tool
Type "./bin/perms"
Enter
Sleep 3s

# Navigate down through frequency list
Down
Sleep 400ms
Down
Sleep 400ms
Down
Sleep 400ms
Down
Sleep 400ms
Down
Sleep 400ms

# Expand a group to show variants
Enter
Sleep 1s

# Navigate into children
Down
Sleep 400ms
Down
Sleep 400ms
Down
Sleep 400ms

# Open apply modal on a child permission
Enter
Sleep 2s

# Close modal
Escape
Sleep 1s

# Switch to Matrix view
Tab
Sleep 2s

# Navigate matrix
Down
Sleep 400ms
Down
Sleep 400ms
Down
Sleep 400ms

# Open agent detail modal
Enter
Sleep 2s

# Close modal
Escape
Sleep 1s

# Switch to Help view
Tab
Sleep 2s

# Quit
Type "q"
Sleep 500ms
