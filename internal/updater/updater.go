package updater

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"palworld-save-relay/internal/logger"
)

// UpdateInfo describes the result of a version check.
type UpdateInfo struct {
	HasUpdate   bool   `json:"hasUpdate"`
	CurrentVer  string `json:"currentVer"`
	LatestVer   string `json:"latestVer"`
	DownloadURL string `json:"downloadUrl"`
	Source      string `json:"source"` // "gitee" or "github"
	ReleaseNote string `json:"releaseNote"`
}

const (
	// Version check: raw files in the repo (no release needed).
	giteeVersionURL  = "https://gitee.com/aues6uen11z/palworld-save-relay/raw/master/version.txt"
	githubVersionURL = "https://raw.githubusercontent.com/Aues6uen11Z/palworld-save-relay/master/version.txt"
	// Binary download: version-specific release tags.
	giteeBinaryBase  = "https://gitee.com/aues6uen11z/palworld-save-relay/releases/download"
	githubBinaryBase = "https://github.com/Aues6uen11Z/palworld-save-relay/releases/download"
	binaryName       = "palworld-save-relay.exe"
)

// fetchVersion tries Gitee raw file first (fast for China), falls back to GitHub.
func fetchVersion() (version, source string, err error) {
	if v, e := fetchURL(giteeVersionURL, 5*time.Second); e == nil {
		ver := strings.TrimSpace(v)
		logger.Infof("updater: Gitee version=%s", ver)
		return ver, "gitee", nil
	}
	v, e := fetchURL(githubVersionURL, 15*time.Second)
	if e != nil {
		return "", "", fmt.Errorf("无法连接更新服务器（Gitee 和 GitHub 均不可达）: %w", e)
	}
	ver := strings.TrimSpace(v)
	logger.Infof("updater: GitHub version=%s", ver)
	return ver, "github", nil
}

// fetchURL fetches a text URL with a timeout.
func fetchURL(url string, timeout time.Duration) (string, error) {
	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "PalworldSaveRelay/Updater")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// CheckForUpdate compares the current version with the latest release.
func CheckForUpdate(currentVersion string) (*UpdateInfo, error) {
	latestVer, source, err := fetchVersion()
	if err != nil {
		return nil, err
	}
	hasUpdate := compareVersions(currentVersion, latestVer) < 0
	// Binary download URL uses the version-specific release tag.
	var binaryBase string
	if source == "gitee" {
		binaryBase = giteeBinaryBase
	} else {
		binaryBase = githubBinaryBase
	}
	downloadURL := binaryBase + "/" + latestVer + "/" + binaryName
	info := &UpdateInfo{
		HasUpdate:   hasUpdate,
		CurrentVer:  currentVersion,
		LatestVer:   latestVer,
		DownloadURL: downloadURL,
		Source:      source,
	}
	return info, nil
}

// compareVersions returns -1 if a < b, 0 if equal, 1 if a > b.
// Handles "v" prefix and semver (major.minor.patch).
func compareVersions(a, b string) int {
	a = strings.TrimPrefix(a, "v")
	b = strings.TrimPrefix(b, "v")
	var aMajor, aMinor, aPatch, bMajor, bMinor, bPatch int
	fmt.Sscanf(a, "%d.%d.%d", &aMajor, &aMinor, &aPatch)
	fmt.Sscanf(b, "%d.%d.%d", &bMajor, &bMinor, &bPatch)
	if aMajor != bMajor {
		if aMajor < bMajor {
			return -1
		}
		return 1
	}
	if aMinor != bMinor {
		if aMinor < bMinor {
			return -1
		}
		return 1
	}
	if aPatch != bPatch {
		if aPatch < bPatch {
			return -1
		}
		return 1
	}
	return 0
}

// DownloadAndUpdate downloads the new binary and applies it in-place.
// On Windows, it writes a batch script that waits for the current process to
// exit, replaces the binary, and restarts the app. The caller should exit
// immediately after this returns successfully.
func DownloadAndUpdate(downloadURL string) error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("自动更新仅支持 Windows")
	}

	// Determine the current executable path.
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("无法确定程序路径: %w", err)
	}
	exeDir := filepath.Dir(exePath)
	tempPath := filepath.Join(exeDir, binaryName+".new")

	// Download the new binary.
	logger.Infof("updater: downloading %s -> %s", downloadURL, tempPath)
	if err := downloadFile(downloadURL, tempPath, 10*time.Minute); err != nil {
		return fmt.Errorf("下载失败: %w", err)
	}
	logger.Info("updater: download complete")

	// Write a batch script to replace the binary and restart.
	pid := os.Getpid()
	batPath := filepath.Join(exeDir, "_update.bat")
	bat := fmt.Sprintf(`@echo off
chcp 65001 >nul 2>nul
:wait
tasklist /fi "pid eq %d" 2>nul | find "%d" >nul
if not errorlevel 1 (
    timeout /t 1 /nobreak >nul
    goto wait
)
move /y "%s" "%s" >nul 2>nul
if errorlevel 1 (
    copy /y "%s" "%s" >nul
    del "%s" >nul 2>nul
)
del "%s" >nul 2>nul
start "" "%s"
`, pid, pid, tempPath, exePath, tempPath, exePath, tempPath, batPath, exePath)

	if err := os.WriteFile(batPath, []byte(bat), 0644); err != nil {
		return fmt.Errorf("写入更新脚本失败: %w", err)
	}

	// Launch the batch script detached.
	if err := startDetached(batPath); err != nil {
		return fmt.Errorf("启动更新脚本失败: %w", err)
	}
	logger.Info("updater: update script launched, exiting app")
	return nil
}

// downloadFile downloads url to filePath with the given timeout.
func downloadFile(url, filePath string, timeout time.Duration) error {
	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "PalworldSaveRelay/Updater")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	out, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, resp.Body)
	return err
}

// startDetached starts a command detached from the current process.
func startDetached(batPath string) error {
	cmd := fmt.Sprintf(`start "" /b "%s"`, batPath)
	return execCmd(cmd)
}

// GetCountry detects the user's country via IP API (for logging/analytics).
func GetCountry() string {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://ip-api.com/json")
	if err != nil {
		return "unknown"
	}
	defer resp.Body.Close()
	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "unknown"
	}
	country, _ := result["country"].(string)
	return country
}
