package config

func getPath() []byte {
	// TODO: for now it's linux-only.
	return []byte("~/.config/zz/setting.json") // does tilde mean anything here?
}
