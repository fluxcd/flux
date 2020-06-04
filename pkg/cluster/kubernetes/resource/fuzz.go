package resource

func Fuzz(data []byte) int {
	_, err := ParseMultidoc(data, string(data))
	if err != nil {
		return 0
	}
	return 1
}
