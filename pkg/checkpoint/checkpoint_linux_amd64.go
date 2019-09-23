package checkpoint

import (
	"syscall"
)

func getKernelVersion() string {
	var uts syscall.Utsname
	syscall.Uname(&uts)
	return cstringToString(uts.Release[:])
}

func cstringToString(c []int8) string {
	s := make([]byte, len(c))
	i := 0
	for ; i < len(c); i++ {
		if c[i] == 0 {
			break
		}
		s[i] = uint8(c[i])
	}
	return string(s[:i])
}
