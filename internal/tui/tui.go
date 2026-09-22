package tui

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/AgusRdz/nvy/internal/platform"
	"github.com/AgusRdz/nvy/internal/store"
	"golang.org/x/term"
)

// ── ANSI ──────────────────────────────────────────────────────────────────────

const (
	ansiReset   = "\033[0m"
	ansiBold    = "\033[1m"
	ansiDim     = "\033[2m"
	ansiRed     = "\033[31m"
	ansiGreen   = "\033[32m"
	ansiYellow  = "\033[33m"
	ansiMagenta = "\033[35m"
	ansiCyan    = "\033[36m"
	ansiGray    = "\033[90m"
	ansiClear   = "\033[2J\033[H"
)

func bold(s string) string    { return ansiBold + s + ansiReset }
func dim(s string) string     { return ansiDim + s + ansiReset }
func red(s string) string     { return ansiRed + s + ansiReset }
func green(s string) string   { return ansiGreen + s + ansiReset }
func yellow(s string) string  { return ansiYellow + s + ansiReset }
func magenta(s string) string { return ansiMagenta + s + ansiReset }
func cyan(s string) string    { return ansiCyan + s + ansiReset }
func gray(s string) string    { return ansiGray + s + ansiReset }

// ── Types ─────────────────────────────────────────────────────────────────────

type varRow struct {
	key       string
	value     string
	expiresAt *time.Time
	note      string
	external  bool // set outside nvy (OS/registry); read-only until imported
	hidden    bool // marked hidden via `nvy hide` / [h]
}

type ui struct {
	section  int // 0=global 1=local 2=path
	cursor   int
	globals  []varRow
	locals   []varRow
	path     []string
	msg      string
	leadDays int
	dir      string
	fd       int
	width    int // terminal columns, sampled once per render(); min 20, fallback 80
	oldState *term.State
	reader   *bufio.Reader

	cfg        store.Config
	showHidden bool // [H] session toggle; default false (hidden entries omitted)

	// hidden entry counts per section, per current cfg — used for header notes.
	globalHiddenN int
	localHiddenN  int
	pathHiddenN   int

	// collapsed sections ("global"/"local"/"path"): [⏎] session toggle, seeded
	// from cfg.CollapsedSections on first load. Never written back to config.
	collapsed map[string]bool

	// settings screen: [c] toggle. When true, render()/the key loop switch to
	// the SETTINGS view; settingsCursor is the focused field (0..3).
	settings       bool
	settingsCursor int
}

// collapsedSectionOrder is the canonical section order used when persisting
// cfg.CollapsedSections from the settings screen (store.setHidden's sorted-list
// helper is alphabetical and doesn't fit here — global/local/path is fixed).
var collapsedSectionOrder = []string{"global", "local", "path"}

// sectionName maps a section index (0=global 1=local 2=path) to its config name.
func sectionName(section int) string {
	switch section {
	case 0:
		return "global"
	case 1:
		return "local"
	case 2:
		return "path"
	}
	return ""
}

// ── Run ───────────────────────────────────────────────────────────────────────

func Run() error {
	cfg, _ := store.LoadConfig()
	dir, err := os.Getwd()
	if err != nil {
		return err
	}

	u := &ui{leadDays: cfg.NotificationLeadDays, dir: dir}
	if err := u.reload(); err != nil {
		return err
	}

	// enter raw mode
	u.fd = int(os.Stdin.Fd())
	u.oldState, err = term.MakeRaw(u.fd)
	if err != nil {
		return err
	}
	defer u.restore()

	// re-enable VT input after MakeRaw (Windows clears it)
	enableVTInput()

	// Run in the alternate screen buffer so the shell's screen and cursor are
	// restored exactly on exit — otherwise the last frame lingers and the shell
	// prompt redraws over it, leaving line-editing/navigation janky.
	fmt.Print("\033[?1049h\033[?25h")
	defer fmt.Print("\033[?1049l\033[?25h")

	u.reader = bufio.NewReader(os.Stdin)

	for {
		u.render()

		key, err := readKey(u.reader)
		if err != nil {
			break
		}

		if u.settings {
			u.handleSettingsKey(key)
			continue
		}

		switch key {
		case "q", "ctrl+c":
			return nil

		case "up":
			if u.cursor > 0 {
				u.cursor--
			}
		case "down":
			if u.cursor < u.sectionLen()-1 {
				u.cursor++
			}
		case "left", "shift+tab":
			u.section = (u.section + 2) % 3
			u.cursor = 0
			u.msg = ""
		case "right", "tab":
			u.section = (u.section + 1) % 3
			u.cursor = 0
			u.msg = ""

		case "n":
			u.cmdNew()

		case "e":
			u.cmdEdit()

		case "d":
			u.cmdDelete()

		case "i":
			u.cmdImport()

		case "x":
			u.cmdExpiry()

		case "h":
			u.cmdToggleHidden()

		case "H":
			u.showHidden = !u.showHidden
			_ = u.reload()
			u.clampCursor()
			u.msg = ""

		case "enter":
			if name := sectionName(u.section); name != "" {
				u.collapsed[name] = !u.collapsed[name]
			}
			u.cursor = 0
			u.msg = ""

		case "c":
			if cfg, err := store.LoadConfig(); err == nil {
				u.cfg = cfg
			} else {
				u.msg = red("error: " + err.Error())
			}
			u.settings = true
			u.settingsCursor = 0
		}
	}
	return nil
}

// ── Render ────────────────────────────────────────────────────────────────────

func (u *ui) render() {
	w, _, err := term.GetSize(u.fd)
	width := 80
	if err == nil && w > 0 {
		width = w
	}
	if width < 20 {
		width = 20
	}
	u.width = width

	var sb strings.Builder
	sb.WriteString(ansiClear)
	u.writeLine(&sb, bold("nvy")+dim(" — environment variable manager"))
	sb.WriteString(dim(strings.Repeat("─", min(u.width, 80))) + "\n\n")

	if u.settings {
		u.renderSettings(&sb)
	} else {
		u.writeSection(&sb, 0, "GLOBAL VARS", u.renderGlobalRows(), u.globalHiddenN)
		u.writeSection(&sb, 1, "LOCAL VARS  "+dim("(.env)"), u.renderLocalRows(), u.localHiddenN)
		u.writePathSection(&sb)
	}

	sb.WriteString(dim(strings.Repeat("─", min(u.width, 80))) + "\n")
	if u.msg != "" {
		u.writeLine(&sb, "  "+u.msg)
	}
	if u.settings {
		u.writeSettingsFooter(&sb)
	} else {
		u.writeFooter(&sb)
	}

	fmt.Print(sb.String())
}

// renderSettings draws the SETTINGS screen: a cyan bold title (matching the
// section headers) followed by the four focusable fields, with a focus caret
// on the current one.
func (u *ui) renderSettings(sb *strings.Builder) {
	u.writeLine(sb, cyan(bold("  SETTINGS")))
	sb.WriteString("\n")

	fields := []string{
		fmt.Sprintf("Notification lead days: %d", u.cfg.NotificationLeadDays),
		"Collapse GLOBAL by default: " + checkbox(u.cfg.IsCollapsed("global")),
		"Collapse LOCAL by default:  " + checkbox(u.cfg.IsCollapsed("local")),
		"Collapse PATH by default:   " + checkbox(u.cfg.IsCollapsed("path")),
	}
	for i, f := range fields {
		var line string
		if i == u.settingsCursor {
			line = cyan("▸ ") + bold(f)
		} else {
			line = "  " + f
		}
		u.writeLine(sb, line)
	}
	sb.WriteString("\n")
}

// checkbox renders a settings checkbox field's current value.
func checkbox(on bool) string {
	if on {
		return "[x]"
	}
	return "[ ]"
}

// writeSettingsFooter renders the settings screen's hotkey hints.
func (u *ui) writeSettingsFooter(sb *strings.Builder) {
	line := footerItem("↑↓", "field") + "  " + footerItem("←→", "change") +
		"  " + footerItem("space", "toggle") + "  " + footerItem("esc", "back")
	u.writeLine(sb, "  "+line)
}

// writeFooter lays out the hotkey hints, flowing them onto as many lines as
// the terminal width allows so no item is ever cut mid-label.
func (u *ui) writeFooter(sb *strings.Builder) {
	items := []struct{ key, label string }{
		{"←→", "section"}, {"↑↓", "navigate"}, {"n", "new"}, {"e", "edit"},
		{"d", "delete"}, {"i", "import"}, {"x", "expiry"}, {"h", "hide"},
		{"H", "show-hidden"}, {"⏎", "fold"}, {"c", "config"}, {"q", "quit"},
	}
	const indent = "  "
	const gap = 2 // visible width of the "  " separator between items
	avail := u.width - len(indent)

	var line strings.Builder
	lineVis := 0
	flush := func() {
		if line.Len() > 0 {
			u.writeLine(sb, indent+line.String())
			line.Reset()
			lineVis = 0
		}
	}
	for _, it := range items {
		vis := utf8.RuneCountInString("[" + it.key + "] " + it.label)
		need := vis
		if lineVis > 0 {
			need += gap
		}
		if lineVis > 0 && lineVis+need > avail {
			flush()
		}
		if lineVis > 0 {
			line.WriteString("  ")
			lineVis += gap
		}
		line.WriteString(footerItem(it.key, it.label))
		lineVis += vis
	}
	flush()
}

// writeLine truncates s to the terminal width (no wrapping) and writes it
// with a trailing newline.
func (u *ui) writeLine(sb *strings.Builder, s string) {
	sb.WriteString(truncateVisible(s, u.width) + "\n")
}

// footerItem renders one footer hotkey as "[key] label", with the bracketed
// key in cyan (so it pops) and the label dim.
func footerItem(key, label string) string {
	return cyan("["+key+"]") + " " + dim(label)
}

// writeSection renders one global/local section header plus its rows, unless
// the section is collapsed — then only the header and a fold marker are
// written (rows suppressed). hiddenN feeds the header's "(N hidden)" note,
// which only appears when the section is expanded; a collapsed header shows
// the row count instead.
func (u *ui) writeSection(sb *strings.Builder, idx int, title string, rows []string, hiddenN int) {
	collapsed := u.collapsed[sectionName(idx)]

	full := title
	if collapsed {
		full += dim(fmt.Sprintf("  (%d)", len(rows)))
	} else {
		full += u.hiddenHeaderNote(hiddenN)
	}

	hdr := cyan(bold("  " + full))
	if u.section == idx {
		hdr = cyan(bold("▸ " + full))
	}
	u.writeLine(sb, hdr)

	if collapsed {
		u.writeLine(sb, dim("  [collapsed — Enter]"))
		sb.WriteString("\n")
		return
	}

	if len(rows) == 0 {
		u.writeLine(sb, dim("  (empty)"))
		sb.WriteString("\n")
		return
	}

	start, end := scrollWindow(u.cursor, len(rows), 6, u.section == idx)
	if start > 0 {
		u.writeLine(sb, dim(fmt.Sprintf("  ↑ %d more", start)))
	}
	for i := start; i < end; i++ {
		selected := u.section == idx && u.cursor == i
		var line string
		if selected {
			line = cyan("▸ ") + bold(rows[i])
		} else {
			line = "  " + rows[i]
		}
		u.writeLine(sb, line)
	}
	if end < len(rows) {
		u.writeLine(sb, dim(fmt.Sprintf("  ↓ %d more", len(rows)-end)))
	}
	sb.WriteString("\n")
}

func (u *ui) writePathSection(sb *strings.Builder) {
	collapsed := u.collapsed["path"]

	suffix := dim(fmt.Sprintf("(%d entries)", len(u.path)))
	if !collapsed {
		suffix += u.hiddenHeaderNote(u.pathHiddenN)
	}
	hdr := cyan(bold("  PATH  ") + suffix)
	if u.section == 2 {
		hdr = cyan(bold("▸ PATH  ") + suffix)
	}
	u.writeLine(sb, hdr)

	if collapsed {
		u.writeLine(sb, dim("  [collapsed — Enter]"))
		sb.WriteString("\n")
		return
	}

	if len(u.path) == 0 {
		u.writeLine(sb, dim("  (empty)"))
		sb.WriteString("\n")
		return
	}

	start, end := scrollWindow(u.cursor, len(u.path), 6, u.section == 2)
	if start > 0 {
		u.writeLine(sb, dim(fmt.Sprintf("  ↑ %d more", start)))
	}
	for i := start; i < end; i++ {
		selected := u.section == 2 && u.cursor == i
		text := u.path[i]
		if u.cfg.IsHiddenPath(text) {
			text += "  (hidden)"
		}
		var line string
		if selected {
			line = cyan("▸ ") + ansiBold + text + ansiReset
		} else {
			line = dim("  " + text)
		}
		u.writeLine(sb, line)
	}
	if end < len(u.path) {
		u.writeLine(sb, dim(fmt.Sprintf("  ↓ %d more", len(u.path)-end)))
	}
	sb.WriteString("\n")
}

// hiddenHeaderNote returns a dim header suffix describing a section's hidden
// entries: nothing when none are hidden, a count-with-hint when showHidden is
// off, or a "showing hidden" note when it's on.
func (u *ui) hiddenHeaderNote(n int) string {
	if n == 0 {
		return ""
	}
	if u.showHidden {
		return dim("  (showing hidden)")
	}
	return dim(fmt.Sprintf("  (%d hidden — H to show)", n))
}

func (u *ui) renderGlobalRows() []string {
	rows := make([]string, len(u.globals))
	for i, r := range u.globals {
		var line string
		if r.external {
			line = gray("ext ") + fmt.Sprintf("%-24s  %s  %s",
				bold(r.key),
				dim(maskValue(r.value)),
				dim("(external, read-only)"),
			)
		} else {
			line = green("nvy ") + fmt.Sprintf("%-24s  %s  %s",
				bold(r.key),
				dim(truncate(r.value, 18)),
				expiryLabel(r.expiresAt, u.leadDays),
			)
			if r.note != "" {
				line += "  " + magenta("["+r.note+"]")
			}
		}
		if r.hidden {
			line = dim(line + "  (hidden)")
		}
		rows[i] = line
	}
	return rows
}

// maskValue previews an external value without exposing the secret: up to the
// first 4 runes, then a fixed mask that reveals neither the tail nor the length.
func maskValue(v string) string {
	r := []rune(v)
	if len(r) == 0 {
		return "••••"
	}
	n := 4
	if len(r) < n {
		n = len(r)
	}
	return string(r[:n]) + "••••"
}

func (u *ui) renderLocalRows() []string {
	rows := make([]string, len(u.locals))
	for i, r := range u.locals {
		line := green("nvy ") + fmt.Sprintf("%-24s  %s  %s",
			bold(r.key),
			dim(truncate(r.value, 18)),
			expiryLabel(r.expiresAt, u.leadDays),
		)
		if r.note != "" {
			line += "  " + magenta("["+r.note+"]")
		}
		if r.hidden {
			line = dim(line + "  (hidden)")
		}
		rows[i] = line
	}
	return rows
}

// ── Commands ──────────────────────────────────────────────────────────────────

func (u *ui) cmdNew() {
	clearScreen()
	var scope string
	switch u.section {
	case 0:
		scope = "global"
	case 1:
		scope = "local"
	case 2:
		entry, ok := u.promptLine(cyan("New PATH entry:") + dim("  (Esc cancels)") + " ")
		if !ok {
			u.msg = dim("cancelled")
			return
		}
		if entry != "" {
			if err := platform.Get().AddToPath(entry); err != nil {
				u.msg = red("error: " + err.Error())
			} else {
				u.msg = ""
			}
			_ = u.reload()
		}
		return
	}

	input, ok := u.promptLine(cyan(fmt.Sprintf("New %s var (KEY=VALUE):", scope)) + dim("  (Esc cancels)") + " ")
	if !ok {
		u.msg = dim("cancelled")
		return
	}
	if input == "" {
		return
	}
	key, value, err := parseKV(input)
	if err != nil {
		u.msg = red("error: " + err.Error())
		return
	}

	if scope == "global" {
		gs, err := store.LoadGlobal()
		if err != nil {
			u.msg = red("error: " + err.Error())
			return
		}
		gs[key] = store.GlobalEntry{Value: value, UpdatedAt: time.Now().UTC()}
		if err := store.SaveGlobal(gs); err != nil {
			u.msg = red("error: " + err.Error())
			return
		}
		_ = platform.Get().ApplyGlobalVar(key, value)
	} else {
		if err := store.SetLocalVar(u.dir, key, value); err != nil {
			u.msg = red("error: " + err.Error())
			return
		}
	}
	_ = u.reload()
	u.msg = ""
}

func (u *ui) cmdEdit() {
	switch u.section {
	case 0:
		if u.cursor >= len(u.globals) {
			return
		}
		r := u.globals[u.cursor]
		if r.external {
			u.msg = dim("external var — read-only; press [i] to import it")
			return
		}
		clearScreen()
		label := fmt.Sprintf(cyan("Edit ")+bold(r.key)+" "+dim("(current: %s)")+"\r\nNew value: ", r.value) + dim("(Esc cancels) ")
		value, ok := u.promptLine(label)
		if !ok {
			u.msg = dim("cancelled")
			return
		}
		if value == "" {
			return
		}
		gs, err := store.LoadGlobal()
		if err != nil {
			u.msg = red("error: " + err.Error())
			return
		}
		existing := gs[r.key]
		gs[r.key] = store.GlobalEntry{Value: value, UpdatedAt: time.Now().UTC(), ExpiresAt: existing.ExpiresAt, Note: existing.Note}
		if err := store.SaveGlobal(gs); err != nil {
			u.msg = red("error: " + err.Error())
			return
		}
		_ = platform.Get().ApplyGlobalVar(r.key, value)

	case 1:
		if u.cursor >= len(u.locals) {
			return
		}
		r := u.locals[u.cursor]
		clearScreen()
		label := fmt.Sprintf(cyan("Edit ")+bold(r.key)+" "+dim("(current: %s)")+"\r\nNew value: ", r.value) + dim("(Esc cancels) ")
		value, ok := u.promptLine(label)
		if !ok {
			u.msg = dim("cancelled")
			return
		}
		if value == "" {
			return
		}
		if err := store.SetLocalVar(u.dir, r.key, value); err != nil {
			u.msg = red("error: " + err.Error())
			return
		}

	case 2:
		if u.cursor >= len(u.path) {
			return
		}
		entry := u.path[u.cursor]
		clearScreen()
		label := fmt.Sprintf(cyan("Edit PATH entry ")+dim("(current: %s)")+"\r\nNew value: ", entry) + dim("(Esc cancels) ")
		newEntry, ok := u.promptLine(label)
		if !ok {
			u.msg = dim("cancelled")
			return
		}
		if newEntry == "" || newEntry == entry {
			return
		}
		_ = platform.Get().RemoveFromPath(entry)
		if err := platform.Get().AddToPath(newEntry); err != nil {
			u.msg = red("error: " + err.Error())
			return
		}
	}

	_ = u.reload()
	u.msg = ""
}

func (u *ui) cmdDelete() {
	var key string
	switch u.section {
	case 0:
		if u.cursor >= len(u.globals) {
			return
		}
		if u.globals[u.cursor].external {
			u.msg = dim("external var — read-only; press [i] to import it")
			return
		}
		key = u.globals[u.cursor].key
	case 1:
		if u.cursor >= len(u.locals) {
			return
		}
		key = u.locals[u.cursor].key
	case 2:
		if u.cursor >= len(u.path) {
			return
		}
		key = u.path[u.cursor]
	}

	clearScreen()
	label := cyan("Delete ") + bold(key) + "? [y/N]" + dim("  (Esc cancels)") + " "
	if !u.confirm(label) {
		return
	}

	switch u.section {
	case 0:
		gs, err := store.LoadGlobal()
		if err != nil {
			u.msg = red("error: " + err.Error())
			return
		}
		delete(gs, key)
		_ = store.SaveGlobal(gs)
		_ = platform.Get().RemoveGlobalVar(key)
	case 1:
		_ = store.RemoveLocalVar(u.dir, key)
	case 2:
		_ = platform.Get().RemoveFromPath(key)
	}

	_ = u.reload()
	u.clampCursor()
	u.msg = ""
}

// cmdImport adopts the selected external global var into nvy's store so it can
// carry metadata and be managed. The value already exists in the OS, so this
// only records it in global.json (no ApplyGlobalVar), mirroring `nvy import`.
func (u *ui) cmdImport() {
	if u.section != 0 || u.cursor >= len(u.globals) {
		return
	}
	r := u.globals[u.cursor]
	if !r.external {
		u.msg = dim("not an external var — nothing to import")
		return
	}
	gs, err := store.LoadGlobal()
	if err != nil {
		u.msg = red("error: " + err.Error())
		return
	}
	gs[r.key] = store.GlobalEntry{Value: r.value, UpdatedAt: time.Now().UTC()}
	if err := store.SaveGlobal(gs); err != nil {
		u.msg = red("error: " + err.Error())
		return
	}
	_ = u.reload()
	u.clampCursor()
	u.msg = "imported " + r.key
}

// cmdExpiry sets or clears the expiry on the selected global or local var.
// An external global var is first adopted into the managed store (same as
// cmdImport) so it can carry metadata. Not applicable to the PATH section.
func (u *ui) cmdExpiry() {
	switch u.section {
	case 0:
		if u.cursor >= len(u.globals) {
			return
		}
		r := u.globals[u.cursor]
		if r.external {
			u.cmdImport()
			for i, g := range u.globals {
				if g.key == r.key {
					u.cursor = i
					r = g
					break
				}
			}
		}

		clearScreen()
		input, ok := u.promptLine(cyan("Expires (YYYY-MM-DD, blank to clear):") + dim("  (Esc cancels)") + " ")
		if !ok {
			u.msg = dim("cancelled")
			return
		}
		expiresAt, ok := parseExpiryInput(input)
		if !ok {
			u.msg = red("error: invalid date (YYYY-MM-DD)")
			return
		}

		gs, err := store.LoadGlobal()
		if err != nil {
			u.msg = red("error: " + err.Error())
			return
		}
		entry := gs[r.key]
		entry.ExpiresAt = expiresAt
		entry.UpdatedAt = time.Now().UTC()
		gs[r.key] = entry
		if err := store.SaveGlobal(gs); err != nil {
			u.msg = red("error: " + err.Error())
			return
		}

	case 1:
		if u.cursor >= len(u.locals) {
			return
		}
		r := u.locals[u.cursor]

		clearScreen()
		input, ok := u.promptLine(cyan("Expires (YYYY-MM-DD, blank to clear):") + dim("  (Esc cancels)") + " ")
		if !ok {
			u.msg = dim("cancelled")
			return
		}
		expiresAt, ok := parseExpiryInput(input)
		if !ok {
			u.msg = red("error: invalid date (YYYY-MM-DD)")
			return
		}

		meta, err := store.LoadLocalMeta(u.dir)
		if err != nil {
			u.msg = red("error: " + err.Error())
			return
		}
		m := meta[r.key]
		m.ExpiresAt = expiresAt
		m.UpdatedAt = time.Now().UTC()
		meta[r.key] = m
		if err := store.SaveLocalMeta(u.dir, meta); err != nil {
			u.msg = red("error: " + err.Error())
			return
		}

	case 2:
		u.msg = dim("expiry not applicable to PATH")
		return
	}

	_ = u.reload()
	u.msg = "updated expiry"
}

// parseExpiryInput parses the expiry prompt's raw input: blank clears the
// expiry, a valid YYYY-MM-DD date sets it. ok is false for anything else.
func parseExpiryInput(s string) (expiresAt *time.Time, ok bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, true
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return nil, false
	}
	return &t, true
}

// ── Settings ──────────────────────────────────────────────────────────────────

// handleSettingsKey routes a key while the settings screen owns input. It
// fully replaces the normal-mode switch — no list-view hotkey acts here.
func (u *ui) handleSettingsKey(key string) {
	switch key {
	case "up":
		if u.settingsCursor > 0 {
			u.settingsCursor--
		}
	case "down":
		if u.settingsCursor < 3 {
			u.settingsCursor++
		}
	case "left", "right":
		if u.settingsCursor == 0 {
			delta := 1
			if key == "left" {
				delta = -1
			}
			u.settingsAdjustLeadDays(delta)
		} else {
			u.toggleCollapsedDefault(collapsedSectionOrder[u.settingsCursor-1])
		}
	case " ", "enter":
		if u.settingsCursor == 0 {
			u.settingsAdjustLeadDays(1)
		} else {
			u.toggleCollapsedDefault(collapsedSectionOrder[u.settingsCursor-1])
		}
	case "esc", "q", "c":
		u.leaveSettings()
	}
}

// settingsAdjustLeadDays adjusts cfg.NotificationLeadDays by delta, clamped
// to 0..365, and persists the change.
func (u *ui) settingsAdjustLeadDays(delta int) {
	v := u.cfg.NotificationLeadDays + delta
	if v < 0 {
		v = 0
	}
	if v > 365 {
		v = 365
	}
	u.cfg.NotificationLeadDays = v
	u.saveConfig()
}

// toggleCollapsedDefault flips whether section starts collapsed by default,
// mutating cfg.CollapsedSections and persisting the change.
func (u *ui) toggleCollapsedDefault(section string) {
	if u.cfg.IsCollapsed(section) {
		u.cfg.CollapsedSections = removeCollapsedSection(u.cfg.CollapsedSections, section)
	} else {
		u.cfg.CollapsedSections = addCollapsedSection(u.cfg.CollapsedSections, section)
	}
	u.saveConfig()
}

// saveConfig persists u.cfg, surfacing a failure via u.msg without crashing.
func (u *ui) saveConfig() {
	if err := store.SaveConfig(u.cfg); err != nil {
		u.msg = red("error: " + err.Error())
	}
}

// leaveSettings exits the settings screen, re-seeds the session collapsed-map
// from the (possibly changed) cfg.CollapsedSections so new defaults take
// effect, and reloads.
func (u *ui) leaveSettings() {
	u.settings = false
	u.collapsed = make(map[string]bool, len(u.cfg.CollapsedSections))
	for _, s := range u.cfg.CollapsedSections {
		u.collapsed[s] = true
	}
	_ = u.reload()
}

// addCollapsedSection adds section to list if absent, keeping the list in
// collapsedSectionOrder (global, local, path).
func addCollapsedSection(list []string, section string) []string {
	for _, s := range list {
		if s == section {
			return list
		}
	}
	out := append(append([]string{}, list...), section)
	rank := func(s string) int {
		for i, o := range collapsedSectionOrder {
			if o == s {
				return i
			}
		}
		return len(collapsedSectionOrder)
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && rank(out[j]) < rank(out[j-1]); j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// removeCollapsedSection removes section from list, if present.
func removeCollapsedSection(list []string, section string) []string {
	out := make([]string, 0, len(list))
	for _, s := range list {
		if s != section {
			out = append(out, s)
		}
	}
	return out
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func (u *ui) reload() error {
	cfg, err := store.LoadConfig()
	if err != nil {
		return err
	}
	u.cfg = cfg

	if u.collapsed == nil {
		u.collapsed = make(map[string]bool, len(cfg.CollapsedSections))
		for _, s := range cfg.CollapsedSections {
			u.collapsed[s] = true
		}
	}

	gs, err := store.LoadGlobal()
	if err != nil {
		return err
	}
	allGlobals := make([]varRow, 0, len(gs))
	managed := make(map[string]bool, len(gs))
	for k, e := range gs {
		managed[k] = true
		allGlobals = append(allGlobals, varRow{key: k, value: e.Value, expiresAt: e.ExpiresAt, note: e.Note, hidden: cfg.IsHiddenGlobal(k)})
	}
	sortRows(allGlobals)

	// Reflect OS-level user vars nvy did not set (read-only), excluding any
	// key already managed. Mirrors `nvy list`. A read error is non-fatal.
	if ext, extErr := platform.Get().ExternalVars(); extErr == nil {
		extRows := make([]varRow, 0, len(ext))
		for k, v := range ext {
			if managed[k] {
				continue
			}
			extRows = append(extRows, varRow{key: k, value: v, external: true, hidden: cfg.IsHiddenGlobal(k)})
		}
		sortRows(extRows)
		allGlobals = append(allGlobals, extRows...)
	}
	u.globals, u.globalHiddenN = filterHiddenRows(allGlobals, u.showHidden)

	env, err := store.LoadEnv(u.dir)
	if err != nil {
		return err
	}
	meta, _ := store.LoadLocalMeta(u.dir)
	allLocals := make([]varRow, 0, len(env))
	for k, v := range env {
		r := varRow{key: k, value: v, hidden: cfg.IsHiddenLocal(k)}
		if m, ok := meta[k]; ok {
			r.expiresAt = m.ExpiresAt
			r.note = m.Note
		}
		allLocals = append(allLocals, r)
	}
	sortRows(allLocals)
	u.locals, u.localHiddenN = filterHiddenRows(allLocals, u.showHidden)

	allPath, _ := platform.Get().GetPath()
	u.path, u.pathHiddenN = filterHiddenPath(allPath, cfg, u.showHidden)

	return nil
}

// filterHiddenRows splits rows into the visible subset (per showHidden) and
// reports how many are hidden per cfg, regardless of visibility.
func filterHiddenRows(rows []varRow, showHidden bool) (visible []varRow, hiddenN int) {
	for _, r := range rows {
		if r.hidden {
			hiddenN++
		}
	}
	if showHidden {
		return rows, hiddenN
	}
	visible = make([]varRow, 0, len(rows)-hiddenN)
	for _, r := range rows {
		if !r.hidden {
			visible = append(visible, r)
		}
	}
	return visible, hiddenN
}

// filterHiddenPath is filterHiddenRows for the PATH section, whose entries
// are plain strings rather than varRow.
func filterHiddenPath(entries []string, cfg store.Config, showHidden bool) (visible []string, hiddenN int) {
	for _, e := range entries {
		if cfg.IsHiddenPath(e) {
			hiddenN++
		}
	}
	if showHidden {
		return entries, hiddenN
	}
	visible = make([]string, 0, len(entries)-hiddenN)
	for _, e := range entries {
		if !cfg.IsHiddenPath(e) {
			visible = append(visible, e)
		}
	}
	return visible, hiddenN
}

// cmdToggleHidden toggles the hidden flag on the currently selected entry —
// global/local by key, PATH by entry string — and persists it to config.
func (u *ui) cmdToggleHidden() {
	if u.sectionLen() == 0 {
		return
	}

	cfg, err := store.LoadConfig()
	if err != nil {
		u.msg = red("error: " + err.Error())
		return
	}

	switch u.section {
	case 0:
		if u.cursor >= len(u.globals) {
			return
		}
		r := u.globals[u.cursor]
		cfg.SetHiddenGlobal(r.key, !cfg.IsHiddenGlobal(r.key))
	case 1:
		if u.cursor >= len(u.locals) {
			return
		}
		r := u.locals[u.cursor]
		cfg.SetHiddenLocal(r.key, !cfg.IsHiddenLocal(r.key))
	case 2:
		if u.cursor >= len(u.path) {
			return
		}
		entry := u.path[u.cursor]
		cfg.SetHiddenPath(entry, !cfg.IsHiddenPath(entry))
	}

	if err := store.SaveConfig(cfg); err != nil {
		u.msg = red("error: " + err.Error())
		return
	}

	_ = u.reload()
	u.clampCursor()
	u.msg = ""
}

func (u *ui) restore() {
	if u.oldState != nil {
		_ = term.Restore(u.fd, u.oldState)
	}
}

func (u *ui) reenter() {
	var err error
	u.oldState, err = term.MakeRaw(u.fd)
	if err != nil {
		u.oldState = nil
	}
}

func (u *ui) sectionLen() int {
	if u.collapsed[sectionName(u.section)] {
		return 0
	}
	switch u.section {
	case 0:
		return len(u.globals)
	case 1:
		return len(u.locals)
	case 2:
		return len(u.path)
	}
	return 0
}

func (u *ui) clampCursor() {
	if n := u.sectionLen(); u.cursor >= n && n > 0 {
		u.cursor = n - 1
	} else if n == 0 {
		u.cursor = 0
	}
}

func scrollWindow(cursor, total, max int, active bool) (start, end int) {
	if total <= max {
		return 0, total
	}
	if !active {
		return 0, max
	}
	start = cursor - max/2
	if start < 0 {
		start = 0
	}
	end = start + max
	if end > total {
		end = total
		start = end - max
		if start < 0 {
			start = 0
		}
	}
	return
}

func readKey(r *bufio.Reader) (string, error) {
	b, err := r.ReadByte()
	if err != nil {
		return "", err
	}
	if b == 0x1b {
		b2, err := r.ReadByte()
		if err != nil || b2 != '[' {
			return "esc", nil
		}
		b3, err := r.ReadByte()
		if err != nil {
			return "esc", nil
		}
		switch b3 {
		case 'A':
			return "up", nil
		case 'B':
			return "down", nil
		case 'C':
			return "right", nil
		case 'D':
			return "left", nil
		case 'Z':
			return "shift+tab", nil
		}
		return "esc", nil
	}
	switch b {
	case '\r', '\n':
		return "enter", nil
	case 3:
		return "ctrl+c", nil
	case 9:
		return "tab", nil
	}
	return string(b), nil
}

// promptLine prints label and reads a line in raw mode from u.reader.
// Returns (text, true) on Enter; ("", false) if the user cancels with Esc or Ctrl+C.
func (u *ui) promptLine(label string) (string, bool) {
	fmt.Print(label)
	var buf []byte
	for {
		b, err := u.reader.ReadByte()
		if err != nil {
			return "", false
		}
		switch {
		case b == 0x1b:
			b2, err := u.reader.ReadByte()
			if err != nil {
				return "", false
			}
			if b2 == '[' {
				// arrow/nav escape sequence — consume and ignore
				if _, err := u.reader.ReadByte(); err != nil {
					return "", false
				}
				continue
			}
			return "", false
		case b == 3:
			return "", false
		case b == '\r' || b == '\n':
			fmt.Print("\r\n")
			return strings.TrimSpace(string(buf)), true
		case b == 0x7f || b == 8:
			if len(buf) > 0 {
				buf = buf[:len(buf)-1]
				fmt.Print("\b \b")
			}
		case b >= 0x20 && b != 0x7f:
			buf = append(buf, b)
			fmt.Print(string(b))
		}
	}
}

// confirm reads a y/N answer in raw mode; Esc/Ctrl+C or anything but y/Y is false.
func (u *ui) confirm(label string) bool {
	text, ok := u.promptLine(label)
	return ok && strings.EqualFold(strings.TrimSpace(text), "y")
}

func clearScreen() {
	fmt.Print(ansiClear)
}

func expiryLabel(expiresAt *time.Time, leadDays int) string {
	if expiresAt == nil {
		return dim("no expiry")
	}
	days := int(time.Until(*expiresAt).Hours() / 24)
	if days < 0 {
		return red("EXPIRED")
	}
	if days <= leadDays {
		return red(fmt.Sprintf("expires in %d days", days))
	}
	if days <= 30 {
		return yellow(fmt.Sprintf("expires in %d days", days))
	}
	return dim(fmt.Sprintf("expires %s", expiresAt.Format("2006-01-02")))
}

func parseKV(s string) (key, value string, err error) {
	idx := strings.Index(s, "=")
	if idx <= 0 {
		return "", "", fmt.Errorf("invalid format: expected KEY=VALUE")
	}
	return strings.TrimSpace(s[:idx]), s[idx+1:], nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}

// truncateVisible returns s limited to at most max VISIBLE columns, ignoring
// ANSI escape sequences (ESC [ ... final-byte) when counting. If it truncates,
// it appends "…" and an ANSI reset. Escape sequences pass through intact and
// are never cut mid-sequence.
func truncateVisible(s string, max int) string {
	var b strings.Builder
	visible := 0
	i := 0
	for i < len(s) {
		if s[i] == 0x1b {
			b.WriteByte(s[i])
			i++
			if i < len(s) && s[i] == '[' {
				b.WriteByte(s[i])
				i++
				for i < len(s) {
					c := s[i]
					b.WriteByte(c)
					i++
					if c >= 0x40 && c <= 0x7e {
						break
					}
				}
			}
			continue
		}
		if visible >= max {
			b.WriteString("…")
			b.WriteString(ansiReset)
			return b.String()
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		b.WriteRune(r)
		i += size
		visible++
	}
	return b.String()
}

func sortRows(rows []varRow) {
	for i := 1; i < len(rows); i++ {
		for j := i; j > 0 && rows[j].key < rows[j-1].key; j-- {
			rows[j], rows[j-1] = rows[j-1], rows[j]
		}
	}
}
