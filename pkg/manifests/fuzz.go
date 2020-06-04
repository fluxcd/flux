package manifests

func Fuzz(data []byte) int {
	var cf ConfigFile
	err := ParseConfigFile(data, &cf)
	if err != nil {
		return 0
	}
	return 1
}
