//go:build !windows

package platform

import (
	"reflect"
	"testing"
)

func TestFilterExternalUnix(t *testing.T) {
	environ := []string{
		"MY_TOKEN=abc",
		"API_KEY=xyz",
		"PATH=/usr/bin:/bin",
		"HOME=/home/user",
		"PWD=/home/user/project",
		"OLDPWD=/home/user",
		"SHELL=/bin/bash",
		"SHLVL=1",
		"TERM=xterm-256color",
		"TMPDIR=/tmp",
		"TMP=/tmp",
		"TEMP=/tmp",
		"USER=user",
		"LOGNAME=user",
		"HOSTNAME=host",
		"LANG=en_US.UTF-8",
		"LANGUAGE=en_US",
		"DISPLAY=:0",
		"COLORTERM=truecolor",
		"PS1=$ ",
		"_=/usr/bin/env",
		"LC_ALL=en_US.UTF-8",
		"SSH_AUTH_SOCK=/tmp/ssh.sock",
		"XDG_SESSION_TYPE=tty",
		"DBUS_SESSION_BUS_ADDRESS=unix:path=/run/dbus",
		"GPG_TTY=/dev/pts/0",
		"XAUTHORITY=/home/user/.Xauthority",
		"GNOME_KEYRING_CONTROL=/run/keyring",
		"KDE_SESSION_VERSION=5",
		"MALFORMED",
		"=weird",
	}

	got := filterExternalUnix(environ)
	want := map[string]string{
		"MY_TOKEN": "abc",
		"API_KEY":  "xyz",
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("filterExternalUnix() = %v, want %v", got, want)
	}
}

func TestCronLines(t *testing.T) {
	cases := []struct {
		hours int
		want  string
	}{
		{1, "0 */1 * * * /bin/nvy check  # nvy-check"},
		{4, "0 */4 * * * /bin/nvy check  # nvy-check"},
		{12, "0 */12 * * * /bin/nvy check  # nvy-check"},
		{24, "0 0 * * * /bin/nvy check  # nvy-check"},
	}
	for _, c := range cases {
		got := cronLines("/bin/nvy", c.hours)
		want := []string{"@reboot /bin/nvy check  # nvy-check", c.want}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("cronLines(%d) = %q, want %q", c.hours, got, want)
		}
	}
}

func TestLaunchdStartInterval(t *testing.T) {
	cases := []struct{ hours, want int }{
		{1, 3600}, {4, 14400}, {24, 86400},
	}
	for _, c := range cases {
		if got := launchdStartInterval(c.hours); got != c.want {
			t.Errorf("launchdStartInterval(%d) = %d, want %d", c.hours, got, c.want)
		}
	}
}

func TestUTF16LEWithBOM(t *testing.T) {
	got := utf16LEWithBOM("aé😀")
	want := []byte{0xFF, 0xFE, 'a', 0x00, 0xE9, 0x00, 0x3D, 0xD8, 0x00, 0xDE}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("utf16LEWithBOM = % X, want % X", got, want)
	}
}
