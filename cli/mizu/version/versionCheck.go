package version

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/up9inc/mizu/cli/apiserver"
	"github.com/up9inc/mizu/cli/mizu"
	"github.com/up9inc/mizu/shared/logger"

	"github.com/google/go-github/v37/github"
	"github.com/up9inc/mizu/cli/uiUtils"
	"github.com/up9inc/mizu/shared/semver"
)

func CheckVersionCompatibility() (bool, error) {
	apiSemVer, err := apiserver.Provider.GetVersion()
	if err != nil {
		return false, err
	}

	if semver.SemVersion(apiSemVer).Major() == semver.SemVersion(mizu.SemVer).Major() &&
		semver.SemVersion(apiSemVer).Minor() == semver.SemVersion(mizu.SemVer).Minor() {
		return true, nil
	}

	logger.Log.Errorf(uiUtils.Red, fmt.Sprintf("cli version (%s) is not compatible with api version (%s)", mizu.SemVer, apiSemVer))
	return false, nil
}

func CheckNewerVersion(versionChan chan string) {
	logger.Log.Debugf("Checking for newer version...")
	start := time.Now()
	client := github.NewClient(nil)
	latestRelease, _, err := client.Repositories.GetLatestRelease(context.Background(), "up9inc", "mizu")
	if err != nil {
		logger.Log.Debugf("[ERROR] Failed to get latest release")
		versionChan <- ""
		return
	}

	versionFileUrl := ""
	for _, asset := range latestRelease.Assets {
		if *asset.Name == "version.txt" {
			versionFileUrl = *asset.BrowserDownloadURL
			break
		}
	}
	if versionFileUrl == "" {
		logger.Log.Debugf("[ERROR] Version file not found in the latest release")
		versionChan <- ""
		return
	}

	res, err := http.Get(versionFileUrl)
	if err != nil {
		logger.Log.Debugf("[ERROR] Failed to get the version file %v", err)
		versionChan <- ""
		return
	}

	data, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		logger.Log.Debugf("[ERROR] Failed to read the version file -> %v", err)
		versionChan <- ""
		return
	}
	gitHubVersion := string(data)
	gitHubVersion = gitHubVersion[:len(gitHubVersion)-1]

	gitHubVersionSemVer := semver.SemVersion(gitHubVersion)
	currentSemVer := semver.SemVersion(mizu.SemVer)
	if !gitHubVersionSemVer.IsValid() || !currentSemVer.IsValid() {
		logger.Log.Debugf("[ERROR] Semver version is not valid, github version %v, current version %v", gitHubVersion, currentSemVer)
		versionChan <- ""
		return
	}

	logger.Log.Debugf("Finished version validation, github version %v, current version %v, took %v", gitHubVersion, currentSemVer, time.Since(start))

	if gitHubVersionSemVer.GreaterThan(currentSemVer) {
		versionChan <- fmt.Sprintf("Update available! %v -> %v (curl -Lo mizu %v/mizu_%s_amd64 && chmod 755 mizu)", mizu.SemVer, gitHubVersion, strings.Replace(*latestRelease.HTMLURL, "tag", "download", 1), runtime.GOOS)
	} else {
		versionChan <- ""
	}
}
