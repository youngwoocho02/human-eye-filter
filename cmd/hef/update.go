package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const repoAPI = "https://api.github.com/repos/youngwoocho02/human-eye-filter/releases/latest"

type ghRelease struct {
	TagName string    `json:"tag_name"`
	Assets  []ghAsset `json:"assets"`
}

type ghAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

func runUpdate(args []string, stdout, stderr io.Writer) (int, error) {
	flags := flag.NewFlagSet("hef update", flag.ContinueOnError)
	flags.SetOutput(stderr)
	checkOnly := flags.Bool("check", false, "check for updates without installing")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0, nil
		}
		return 2, err
	}

	if _, err := fmt.Fprintln(stdout, "Checking for updates..."); err != nil {
		return 1, err
	}
	release, err := fetchLatestRelease()
	if err != nil {
		return 1, fmt.Errorf("failed to check for updates: %w", err)
	}

	current := Version
	latest := release.TagName
	if current == latest {
		if _, err := fmt.Fprintf(stdout, "Already up to date (%s)\n", current); err != nil {
			return 1, err
		}
		return 0, nil
	}

	if _, err := fmt.Fprintf(stdout, "Update available: %s -> %s\n", current, latest); err != nil {
		return 1, err
	}
	if *checkOnly {
		return 0, nil
	}

	asset := findAsset(release.Assets)
	if asset == nil {
		return 1, fmt.Errorf("no binary found for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	exe, err := os.Executable()
	if err != nil {
		return 1, fmt.Errorf("cannot locate current binary: %w", err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return 1, fmt.Errorf("cannot resolve binary path: %w", err)
	}

	if _, err := fmt.Fprintf(stdout, "Downloading %s...\n", asset.Name); err != nil {
		return 1, err
	}
	tmpFile, err := download(asset.BrowserDownloadURL, filepath.Dir(exe))
	if err != nil {
		return 1, fmt.Errorf("download failed: %w", err)
	}
	defer func() { _ = os.Remove(tmpFile) }()

	if err := os.Chmod(tmpFile, 0o755); err != nil {
		return 1, fmt.Errorf("chmod failed: %w", err)
	}

	backup := exe + ".bak"
	if err := os.Rename(exe, backup); err != nil {
		return 1, fmt.Errorf("backup failed: %w", err)
	}

	if err := os.Rename(tmpFile, exe); err != nil {
		if restoreErr := os.Rename(backup, exe); restoreErr != nil {
			return 1, fmt.Errorf("replace failed: %w (restore also failed: %v)", err, restoreErr)
		}
		return 1, fmt.Errorf("replace failed: %w", err)
	}

	_ = os.Remove(backup)
	fmt.Fprintf(stdout, "Updated to %s\n", latest)
	return 0, nil
}

func fetchLatestRelease() (*ghRelease, error) {
	resp, err := http.Get(repoAPI)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}
	return &release, nil
}

func findAsset(assets []ghAsset) *ghAsset {
	suffix := fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH)
	for i, asset := range assets {
		if strings.Contains(asset.Name, suffix) {
			return &assets[i]
		}
	}
	return nil
}

func download(url string, targetDir string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download returned %d", resp.StatusCode)
	}

	tmp, err := os.CreateTemp(targetDir, "hef-update-*")
	if err != nil {
		return "", err
	}
	defer tmp.Close()

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		_ = os.Remove(tmp.Name())
		return "", err
	}

	return tmp.Name(), nil
}
