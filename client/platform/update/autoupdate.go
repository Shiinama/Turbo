package update

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"time"

	"golang.org/x/mod/semver"
)

var Version = "v0.1.0-dev"

type GitHubRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

const url = "https://api.github.com/repos/Shiinama/Turbo/releases/latest"

type UpdateResult struct {
	CurrentVersion string
	LatestVersion  string
	Updated        bool
}

func CurrentVersion() string {
	return Version
}

func AutoUpdate() error {
	_, err := CheckAndUpdate()
	return err
}

func CheckAndUpdate() (UpdateResult, error) {
	return checkAndUpdate(true)
}

func CheckLatestVersion() (UpdateResult, bool, error) {
	client := http.Client{
		Timeout: 10 * time.Second,
	}
	result := UpdateResult{CurrentVersion: Version}
	release, hasUpdate, err := checkForUpdate(client)
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			return result, false, nil
		}
		return result, false, fmt.Errorf("checking for updates: %w", err)
	}
	result.LatestVersion = release.TagName
	return result, hasUpdate, nil
}

func checkAndUpdate(install bool) (UpdateResult, error) {
	result := UpdateResult{CurrentVersion: Version}
	client := http.Client{
		Timeout: 10 * time.Second,
	}

	release, hasUpdate, err := checkForUpdate(client)
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			return result, nil // No release yet
		}
		return result, fmt.Errorf("checking for updates: %w", err)
	}
	result.LatestVersion = release.TagName
	if !hasUpdate {
		return result, nil
	}
	if !install {
		return result, nil
	}

	assetURL, err := findAssetForPlatform(release)
	if err != nil {
		return result, fmt.Errorf("finding asset url: %w", err)
	}

	assetData, err := downloadUpdate(client, assetURL)
	if err != nil {
		return result, fmt.Errorf("downloading update: %w", err)
	}

	if err := replaceExecutable(assetData); err != nil {
		return result, fmt.Errorf("replacing executable: %w", err)
	}

	result.Updated = true
	return result, nil
}

func checkForUpdate(client http.Client) (*GitHubRelease, bool, error) {
	req, err := http.NewRequest("GET", url, nil)

	if err != nil {
		return nil, false, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("User-Agent", "Turbo-updater/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("fetching release info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, false, fmt.Errorf("decoding release info: %w", err)
	}
	hasUpdate := semver.Compare(normalizeSemver(release.TagName), normalizeSemver(Version)) == +1

	return &release, hasUpdate, nil
}

func normalizeSemver(version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		return version
	}
	if !strings.HasPrefix(version, "v") {
		return "v" + version
	}
	return version
}

func findAssetForPlatform(release *GitHubRelease) (string, error) {
	var assetURL string
	for _, asset := range release.Assets {
		assetName := strings.ToLower(asset.Name)

		if !isInstallableAsset(assetName) {
			continue
		}

		if strings.Contains(assetName, runtime.GOOS+"-"+runtime.GOARCH) {
			assetURL = asset.BrowserDownloadURL
			break
		}
	}

	if assetURL == "" {
		return "", fmt.Errorf("no suitable asset found for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	return assetURL, nil
}

func isInstallableAsset(assetName string) bool {
	if strings.HasSuffix(assetName, ".sha256") ||
		strings.HasSuffix(assetName, ".zip") ||
		strings.HasSuffix(assetName, ".dmg") {
		return false
	}
	return true
}

func downloadUpdate(client http.Client, url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating download request: %w", err)
	}

	req.Header.Set("User-Agent", "Turbo-updater/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("downloading asset: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}
