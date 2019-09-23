// +build !windows

package update

import (
	"os"

	"github.com/pkg/term"
	"golang.org/x/sys/unix"
)

func terminalWidth() uint16 {
	ws, _ := unix.IoctlGetWinsize(int(os.Stdout.Fd()), unix.TIOCGWINSZ)
	if ws != nil && ws.Col != 0 {
		return ws.Col
	}
	return 9999
}

// See https://github.com/paulrademacher/climenu/blob/master/getchar.go
func getChar() (ascii int, keyCode int, err error) {
	t, _ := term.Open("/dev/tty")
	term.RawMode(t)
	bs := make([]byte, 3)

	var numRead int
	numRead, err = t.Read(bs)
	if err != nil {
		return
	}
	if numRead == 3 && bs[0] == 27 && bs[1] == 91 {
		// Three-character control sequence, beginning with "ESC-[".

		// Since there are no ASCII codes for arrow keys, we use
		// Javascript key codes.
		if bs[2] == 65 {
			// Up
			keyCode = 38
		} else if bs[2] == 66 {
			// Down
			keyCode = 40
		} else if bs[2] == 67 {
			// Right
			keyCode = 39
		} else if bs[2] == 68 {
			// Left
			keyCode = 37
		}
	} else if numRead == 1 {
		ascii = int(bs[0])
	} else {
		// Two characters read??
	}
	t.Restore()
	t.Close()
	return
}
