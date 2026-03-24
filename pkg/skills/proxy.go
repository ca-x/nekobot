package skills

import "strings"

func skillsProxyEnv(base []string, proxyURL string) []string {
	proxyURL = strings.TrimSpace(proxyURL)
	if proxyURL == "" {
		return base
	}

	env := make([]string, 0, len(base)+6)
	env = append(env, base...)
	for _, key := range []string{
		"HTTP_PROXY",
		"HTTPS_PROXY",
		"ALL_PROXY",
		"http_proxy",
		"https_proxy",
		"all_proxy",
	} {
		env = append(env, key+"="+proxyURL)
	}
	return env
}
