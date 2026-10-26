//go:build windows

package main

const (
	githubRepository = "FyarrAja/Senja-Bar"
	releaseAssetName = "Senja-Bar-windows-x64.zip"
	updateStateName  = "update-state.json"
)

// buildVersion is replaced by scripts/package-release.ps1 for official builds.
// The fallback keeps local development builds usable.
var buildVersion = "1.1.5"

func latestReleaseURL() string {
	return "https://github.com/" + githubRepository + "/releases/latest"
}
