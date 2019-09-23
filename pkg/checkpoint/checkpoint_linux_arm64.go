package checkpoint

import (
	"syscall"
)

func getKernelVersion() string {
	var uts syscall.Utsname
	syscall.Uname(&uts)
	return cstringToString(uts.Release[:])
}

func cstringToString(arr []int8) string {
	b := make([]byte, 0, len(arr))
	for _, v := range arr {
		if v == 0x00 {
			break
		}
		b = append(b, byte(v))
	}
	return string(b)
}
