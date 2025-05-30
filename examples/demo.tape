# VHS tape demonstrating enhanced git-add-interactive features
# Auto-split, Global filter, and Auto-accept functionality

Output demo.gif
Set FontSize 14
Set Width 1200
Set Height 800
Set TypingSpeed 1ms
Set Theme "Molokai"

Hide

# Setup - create a test repository with various changes
Type "mkdir -p /tmp/git-demo && cd /tmp/git-demo"
Enter

Type "export PS1='$ '"
Enter

Type "git init"
Enter

# Create initial file with multiple sections
Type "cat > test.js << 'EOF'"
Enter
Type "function hello() {"
Enter
Type "    console.log('Hello');"
Enter
Type "    var x = 1;"
Enter
Type "    var y = 2;"
Enter
Type "}"
Enter
Type ""
Enter
Type "function goodbye() {"
Enter
Type "    console.log('Goodbye');"
Enter
Type "    var a = 3;"
Enter
Type "    var b = 4;"
Enter
Type "}"
Enter
Type ""
Enter
Type "function test() {"
Enter
Type "    console.log('Test');"
Enter
Type "    var z = 5;"
Enter
Type "}"
Enter
Type "EOF"
Enter

Type "git add test.js && git commit -m 'Initial commit'"
Enter

# Make multiple changes that can be split
Type "cat > test.js << 'EOF'"
Enter
Type "function hello() {"
Enter
Type "    console.log('Hello World!');"
Enter
Type "    var x = 1;"
Enter
Type "    var y = 2;"
Enter
Type "    console.log('Variables set');"
Enter
Type "}"
Enter
Type ""
Enter
Type "function goodbye() {"
Enter
Type "    console.log('Goodbye World!');"
Enter
Type "    var a = 3;"
Enter
Type "    var b = 4;"
Enter
Type "    console.log('More variables');"
Enter
Type "}"
Enter
Type ""
Enter
Type "function test() {"
Enter
Type "    console.log('Test Function');"
Enter
Type "    var z = 5;"
Enter
Type "    console.log('Final variable');"
Enter
Type "}"
Enter
Type "EOF"
Enter

# Build and install our enhanced git-add-interactive
Type "go install github.com/cwarden/git-add--interactive@latest"
Enter

Type 'export GIT_EXEC_PATH="$(go env GOPATH)/bin:$(git --exec-path)"'
Enter

# Start the demo - show what changes we have
Type "clear"
Enter

Show

Set TypingSpeed 50ms

Type "echo 'git-add--interactive Enhanced Features Demo'"
Enter
Type "echo '============================================'"
Enter
Sleep 1s

Type "echo 'First, let'\''s see what changes we have:'"
Enter
Type "git diff"
Enter
Sleep 4s

Type "echo"
Enter
Type "echo 'Now let'\''s use git add -p to demonstrate enhanced features:'"
Enter
Sleep 2s

# Demo 1: Auto-split functionality
Type "echo"
Enter
Type "echo '1. AUTO-SPLIT FEATURE (S command)'"
Enter
Type "echo 'Automatically splits all hunks where possible'"
Enter
Sleep 2s

# Start patch mode with standard git command
Type "git add -p"
Sleep 1s
Enter
Sleep 2s

# Show the first hunk and demonstrate auto-split
Type "S"
Sleep 2s
Enter
Type "q"
Enter
Sleep 2s

# Demo 2: Global filter functionality
Type "echo"
Enter
Type "echo '2. GLOBAL FILTER FEATURE (G command)'"
Enter
Type "echo 'Filter hunks across all files by regex pattern'"
Enter
Sleep 2s

Type "git add -p"
Sleep 1s
Enter
Sleep 2s

# Set global filter for console.log changes
Type "S"
Sleep 2s
Enter
Sleep 2s
Type "G console"
Sleep 2s
Enter
Sleep 2s
Type "q"
Enter
Sleep 2s

# Demo 3: Auto-accept functionality
Type "echo"
Enter
Type "echo '3. AUTO-ACCEPT FEATURE (A command)'"
Enter
Type "echo 'Accept all hunks after splitting and filtering'"
Enter
Sleep 3s

Type "git add -p"
Sleep 1s
Enter
Sleep 2s
Type "S"
Sleep 2s
Enter
Sleep 2s
Type "G console"
Sleep 2s
Enter
Sleep 2s
Type "A"
Sleep 2s
Enter
Sleep 2s

# Show final result
Type "echo"
Enter
Type "echo 'DEMO COMPLETE!'"
Enter
Type "echo 'Features demonstrated:'"
Enter
Type "echo '• S - Auto-split all hunks globally'"
Enter
Type "echo '• G - Global filter with regex patterns'"
Enter
Type "echo '• A - Auto-accept all hunks (after filtering)'"
Enter
Sleep 3s

Type "echo"
Enter
Type "echo 'Check git status to see applied changes:'"
Enter
Type "git status"
Enter
Sleep 2s

Type "echo"
Enter
Type "echo 'View the staged changes:'"
Enter
Type "git diff --cached"
Enter
Sleep 5s
