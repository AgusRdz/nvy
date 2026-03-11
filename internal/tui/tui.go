package tui

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/AgusRdz/nvy/internal/platform"
	"github.com/AgusRdz/nvy/internal/store"
	"golang.org/x/term"
)

// ── ANSI ──────────────────────────────────────────────────────────────────────

const (
	ansiReset  = "\033[0m"
	ansiBold   = "\033[1m"
	ansiDim    = "\033[2m"
	ansiRed    = "\033[31m"
	ansiYellow = "\033[33m"
	ansiCyan   = "\033[36m"
	ansiClear  = "\033[2J\033[H"
)

func bold(s string) string   { return ansiBold + s + ansiReset }
func dim(s string) string    { return ansiDim + s + ansiReset }
func red(s string) string    { return ansiRed + s + ansiReset }
func yellow(s string) string { return ansiYellow + s + ansiReset }
func cyan(s string) string   { return ansiCyan + s + ansiReset }

// ── Types ─────────────────────────────────────────────────────────────────────

type varRow struct {
	key       string
	value     string
	expiresAt *time.Time
	note      string
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
	oldState *term.State
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

	reader := bufio.NewReader(os.Stdin)

	for {
		u.render()

		key, err := readKey(reader)
		if err != nil {
			break
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
			u.restore()
			u.cmdNew()
			u.reenter()
			enableVTInput()

		case "e":
			u.restore()
			u.cmdEdit()
			u.reenter()
			enableVTInput()

		case "d":
			u.restore()
			u.cmdDelete()
			u.reenter()
			enableVTInput()
		}
	}
	return nil
}

// ── Render ────────────────────────────────────────────────────────────────────

func (u *ui) render() {
	var sb strings.Builder
	sb.WriteString(ansiClear)
	sb.WriteString(bold("nvy") + dim(" — environment variable manager") + "\n")
	sb.WriteString(dim(strings.Repeat("─", 60)) + "\n\n")

	u.writeSection(&sb, 0, "GLOBAL VARS", u.renderGlobalRows())
	u.writeSection(&sb, 1, "LOCAL VARS  "+dim("(.env)"), u.renderLocalRows())
	u.writePathSection(&sb)

	sb.WriteString(dim(strings.Repeat("─", 60)) + "\n")
	if u.msg != "" {
		sb.WriteString("  " + u.msg + "\n")
	}
	sb.WriteString(dim("  [←→] section  [↑↓] navigate  [n] new  [e] edit  [d] delete  [q] quit") + "\n")

	fmt.Print(sb.String())
}

func (u *ui) writeSection(sb *strings.Builder, idx int, title string, rows []string) {
	hdr := cyan(bold("  " + title))
	if u.section == idx {
		hdr = cyan(bold("▸ " + title))
	}
	sb.WriteString(hdr + "\n")

	if len(rows) == 0 {
		sb.WriteString(dim("  (empty)") + "\n\n")
		return
	}

	start, end := scrollWindow(u.cursor, len(rows), 6, u.section == idx)
	if start > 0 {
		sb.WriteString(dim(fmt.Sprintf("  ↑ %d more", start)) + "\n")
	}
	for i := start; i < end; i++ {
		selected := u.section == idx && u.cursor == i
		if selected {
			sb.WriteString(bold(ansiBold+"▸ "+rows[i]) + ansiReset + "\n")
		} else {
			sb.WriteString("  " + rows[i] + "\n")
		}
	}
	if end < len(rows) {
		sb.WriteString(dim(fmt.Sprintf("  ↓ %d more", len(rows)-end)) + "\n")
	}
	sb.WriteString("\n")
}

func (u *ui) writePathSection(sb *strings.Builder) {
	hdr := cyan(bold(fmt.Sprintf("  PATH  ")) + dim(fmt.Sprintf("(%d entries)", len(u.path))))
	if u.section == 2 {
		hdr = cyan(bold(fmt.Sprintf("▸ PATH  ")) + dim(fmt.Sprintf("(%d entries)", len(u.path))))
	}
	sb.WriteString(hdr + "\n")

	if len(u.path) == 0 {
		sb.WriteString(dim("  (empty)") + "\n\n")
		return
	}

	start, end := scrollWindow(u.cursor, len(u.path), 6, u.section == 2)
	if start > 0 {
		sb.WriteString(dim(fmt.Sprintf("  ↑ %d more", start)) + "\n")
	}
	for i := start; i < end; i++ {
		selected := u.section == 2 && u.cursor == i
		if selected {
			sb.WriteString(ansiBold + "▸ " + u.path[i] + ansiReset + "\n")
		} else {
			sb.WriteString(dim("  "+u.path[i]) + "\n")
		}
	}
	if end < len(u.path) {
		sb.WriteString(dim(fmt.Sprintf("  ↓ %d more", len(u.path)-end)) + "\n")
	}
	sb.WriteString("\n")
}

func (u *ui) renderGlobalRows() []string {
	rows := make([]string, len(u.globals))
	for i, r := range u.globals {
		rows[i] = fmt.Sprintf("%-24s  %s  %s",
			bold(r.key),
			dim(truncate(r.value, 18)),
			expiryLabel(r.expiresAt, u.leadDays),
		)
	}
	return rows
}

func (u *ui) renderLocalRows() []string {
	rows := make([]string, len(u.locals))
	for i, r := range u.locals {
		rows[i] = fmt.Sprintf("%-24s  %s  %s",
			bold(r.key),
			dim(truncate(r.value, 18)),
			expiryLabel(r.expiresAt, u.leadDays),
		)
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
		fmt.Print(cyan("New PATH entry: "))
		entry := readLine()
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

	fmt.Print(cyan(fmt.Sprintf("New %s var (KEY=VALUE): ", scope)))
	input := readLine()
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
		clearScreen()
		fmt.Printf(cyan("Edit ")+bold(r.key)+" "+dim("(current: %s)")+"\nNew value: ", r.value)
		value := readLine()
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
		fmt.Printf(cyan("Edit ")+bold(r.key)+" "+dim("(current: %s)")+"\nNew value: ", r.value)
		value := readLine()
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
		fmt.Printf(cyan("Edit PATH entry ")+dim("(current: %s)")+"\nNew value: ", entry)
		newEntry := readLine()
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
	fmt.Printf(cyan("Delete ")+bold(key)+"? [y/N] ")
	answer := readLine()
	if strings.ToLower(strings.TrimSpace(answer)) != "y" {
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

// ── Helpers ───────────────────────────────────────────────────────────────────

func (u *ui) reload() error {
	gs, err := store.LoadGlobal()
	if err != nil {
		return err
	}
	u.globals = make([]varRow, 0, len(gs))
	for k, e := range gs {
		u.globals = append(u.globals, varRow{key: k, value: e.Value, expiresAt: e.ExpiresAt, note: e.Note})
	}
	sortRows(u.globals)

	env, err := store.LoadEnv(u.dir)
	if err != nil {
		return err
	}
	meta, _ := store.LoadLocalMeta(u.dir)
	u.locals = make([]varRow, 0, len(env))
	for k, v := range env {
		r := varRow{key: k, value: v}
		if m, ok := meta[k]; ok {
			r.expiresAt = m.ExpiresAt
			r.note = m.Note
		}
		u.locals = append(u.locals, r)
	}
	sortRows(u.locals)

	u.path, _ = platform.Get().GetPath()
	return nil
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

func readLine() string {
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		return strings.TrimSpace(scanner.Text())
	}
	return ""
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

func sortRows(rows []varRow) {
	for i := 1; i < len(rows); i++ {
		for j := i; j > 0 && rows[j].key < rows[j-1].key; j-- {
			rows[j], rows[j-1] = rows[j-1], rows[j]
		}
	}
}
