# nvy — TUI demo tape.
#
# Renders docs/demo.gif with vhs (https://github.com/charmbracelet/vhs):
#
#     vhs demo.tape        # or: make demo
#
# Requires `nvy` and `vhs` on PATH. The demo runs entirely in a throwaway
# temp directory using --local vars, so it never mutates your real
# environment. The expiry dates below are hardcoded — bump them to a few
# upcoming days before recording so the red/yellow coloring shows.

Output docs/demo.gif

Set Shell "bash"
Set FontSize 16
Set Width 1100
Set Height 700
Set Padding 24
Set Theme "Dracula"
Set TypingSpeed 55ms

# --- Setup (hidden): isolate the demo in a fresh project ---------------------
Hide
Type "cd \"$(mktemp -d)\" && clear" Enter
Show

Type "# nvy — env vars that expire and remind you, offline"
Sleep 1200ms
Enter
Sleep 400ms

# A normal var, plus three with expiries: past (red), soon (yellow), far (fine).
# Values are obvious placeholders — swap in whatever you like before recording.
Type "nvy set API_URL=https://api.dev --local" Enter
Sleep 700ms
Type "nvy set OLD_TOKEN=demo-value --local --expires 2026-09-18 --note 'rotate me'" Enter
Sleep 700ms
Type "nvy set API_TOKEN=demo-value --local --expires 2026-09-26" Enter
Sleep 700ms
Type "nvy set STRIPE_KEY=demo-value --local --expires 2027-03-01" Enter
Sleep 900ms

Type "clear" Enter
Sleep 300ms

# --- The TUI -----------------------------------------------------------------
Type "nvy ui" Enter
Sleep 2s

# Walk the list — keys are colored by urgency (red expired, yellow near).
Down
Sleep 900ms
Down
Sleep 900ms
Down
Sleep 1200ms

# The [?] help overlay: the full key reference, grouped.
Type "?"
Sleep 2500ms
Escape
Sleep 1s

# The [c] settings screen.
Type "c"
Sleep 2s
Escape
Sleep 800ms

# Quit.
Type "q"
Sleep 1200ms
