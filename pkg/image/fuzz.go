package image

func Fuzz(data []byte) int {
	_, err := ParseRef(string(data))
	if err != nil {
		return 0
	}
	return 1
}
