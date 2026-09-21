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
