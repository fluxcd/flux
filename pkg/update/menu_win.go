// +build windows

package update

import "errors"

func terminalWidth() uint16 {
	return 9999
}

func getChar() (ascii int, keyCode int, err error) {
	return 0, 0, errors.New("Error: Interactive mode is not supported on Windows")
}
