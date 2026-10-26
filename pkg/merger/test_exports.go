// Wraps registry.go internals for other packages' tests — not available in
// the runtime/gateway builds, same as registry.go itself.

//go:build !runtime && !gateway

package merger

func ExportedGitHubRawURL(repoURL, ref, filePath string) string {
	return githubRawURL(repoURL, ref, filePath)
}

func ExportedGitLabRawURL(repoURL, ref, filePath string) string {
	return gitlabRawURL(repoURL, ref, filePath)
}

func ExportedIsGitHubURL(u string) bool {
	return isGitHubURL(u)
}

func ExportedIsGitLabURL(u string) bool {
	return isGitLabURL(u)
}

func ExportedValidatePatternStructure(dir, url, version string) error {
	return validatePatternStructure(dir, url, version)
}
