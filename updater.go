package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
)

var appVersion = "dev"

const githubRepo = "thomasjdelaney/BubbleWorldClock"

type githubRelease struct {
	TagName string        `json:"tag_name"`
	Assets  []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

func compareVersions(current, candidate string) int {
	currentParts, currentOK := parseVersion(current)
	candidateParts, candidateOK := parseVersion(candidate)
	if !currentOK || !candidateOK {
		return 0
	}
	maxLen := max(len(currentParts), len(candidateParts))
	for i := 0; i < maxLen; i++ {
		left := 0
		if i < len(currentParts) {
			left = currentParts[i]
		}
		right := 0
		if i < len(candidateParts) {
			right = candidateParts[i]
		}
		if left < right {
			return -1
		}
		if left > right {
			return 1
		}
	}
	return 0
}

func parseVersion(value string) ([]int, bool) {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.TrimPrefix(trimmed, "v")
	trimmed = strings.TrimPrefix(trimmed, "release-")
	if trimmed == "" || trimmed == "dev" || trimmed == "(devel)" {
		return nil, false
	}
	for i, char := range trimmed {
		if (char < '0' || char > '9') && char != '.' {
			trimmed = trimmed[:i]
			break
		}
	}
	if trimmed == "" {
		return nil, false
	}
	parts := strings.Split(trimmed, ".")
	parsed := make([]int, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			return nil, false
		}
		number, err := strconv.Atoi(part)
		if err != nil {
			return nil, false
		}
		parsed = append(parsed, number)
	}
	return parsed, true
}

func currentVersion() string {
	if appVersion != "" && appVersion != "dev" {
		return appVersion
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}

func releaseAssetName(osName, arch, version string) string {
	clean := strings.TrimPrefix(version, "v")
	switch osName {
	case "linux":
		switch arch {
		case "amd64", "x86_64":
			return fmt.Sprintf("BubbleWorldClock_%s_Linux_x86_64.tar.gz", clean)
		case "arm64", "aarch64":
			return fmt.Sprintf("BubbleWorldClock_%s_Linux_arm64.tar.gz", clean)
		}
	case "darwin":
		switch arch {
		case "amd64", "x86_64":
			return fmt.Sprintf("BubbleWorldClock_%s_Darwin_x86_64.tar.gz", clean)
		case "arm64", "aarch64":
			return fmt.Sprintf("BubbleWorldClock_%s_Darwin_arm64.tar.gz", clean)
		}
	case "windows":
		switch arch {
		case "amd64", "x86_64":
			return fmt.Sprintf("BubbleWorldClock_%s_Windows_x86_64.zip", clean)
		case "arm64", "aarch64":
			return fmt.Sprintf("BubbleWorldClock_%s_Windows_arm64.zip", clean)
		}
	}
	return ""
}

func fetchLatestRelease() (githubRelease, error) {
	request, err := http.NewRequest(http.MethodGet, "https://api.github.com/repos/"+githubRepo+"/releases/latest", nil)
	if err != nil {
		return githubRelease{}, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "BubbleWorldClock-updater")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return githubRelease{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return githubRelease{}, fmt.Errorf("GitHub release lookup failed: %s", response.Status)
	}
	var release githubRelease
	if err := json.NewDecoder(response.Body).Decode(&release); err != nil {
		return githubRelease{}, err
	}
	if release.TagName == "" {
		return githubRelease{}, errors.New("latest release has no tag")
	}
	return release, nil
}

func downloadFile(url, path string) error {
	request, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	request.Header.Set("User-Agent", "BubbleWorldClock-updater")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed for %s: %s", url, response.Status)
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := io.Copy(file, response.Body); err != nil {
		return err
	}
	return nil
}

func verifyChecksum(checksumPath, archiveName string) error {
	contents, err := os.ReadFile(checksumPath)
	if err != nil {
		return err
	}
	filePath := filepath.Join(filepath.Dir(checksumPath), archiveName)
	blob, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	hash := sha256.Sum256(blob)
	expected := strings.TrimSpace(string(contents))
	for _, line := range strings.Split(expected, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && (fields[len(fields)-1] == archiveName || strings.HasSuffix(fields[len(fields)-1], "/"+archiveName)) {
			if strings.EqualFold(hex.EncodeToString(hash[:]), fields[0]) {
				return nil
			}
			return fmt.Errorf("checksum mismatch for %s", archiveName)
		}
	}
	return fmt.Errorf("no checksum entry for %s", archiveName)
}

func extractReleaseArchive(archivePath string) (string, error) {
	if strings.HasSuffix(archivePath, ".zip") {
		archiveReader, err := zip.OpenReader(archivePath)
		if err != nil {
			return "", err
		}
		defer archiveReader.Close()
		dir := filepath.Dir(archivePath)
		for _, file := range archiveReader.File {
			if filepath.Base(file.Name) == "bubble-world-clock" || filepath.Base(file.Name) == "bubble-world-clock.exe" {
				targetPath := filepath.Join(dir, filepath.Base(file.Name))
				reader, err := file.Open()
				if err != nil {
					return "", err
				}
				defer reader.Close()
				writer, err := os.Create(targetPath)
				if err != nil {
					return "", err
				}
				if _, err := io.Copy(writer, reader); err != nil {
					writer.Close()
					return "", err
				}
				writer.Close()
				return targetPath, nil
			}
		}
		return "", fmt.Errorf("archive does not contain %s", filepath.Base(archivePath))
	}
	if strings.HasSuffix(archivePath, ".tar.gz") {
		file, err := os.Open(archivePath)
		if err != nil {
			return "", err
		}
		defer file.Close()
		gzipReader, err := gzip.NewReader(file)
		if err != nil {
			return "", err
		}
		defer gzipReader.Close()
		tarReader := tar.NewReader(gzipReader)
		dir := filepath.Dir(archivePath)
		for {
			header, err := tarReader.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return "", err
			}
			name := filepath.Base(header.Name)
			if name == "bubble-world-clock" || name == "bubble-world-clock.exe" {
				targetPath := filepath.Join(dir, name)
				writer, err := os.Create(targetPath)
				if err != nil {
					return "", err
				}
				if _, err := io.Copy(writer, tarReader); err != nil {
					writer.Close()
					return "", err
				}
				writer.Close()
				return targetPath, nil
			}
		}
		return "", fmt.Errorf("archive does not contain bubble-world-clock")
	}
	return "", fmt.Errorf("unsupported archive format: %s", archivePath)
}

func updateBinary(newBinaryPath string) error {
	executablePath, err := os.Executable()
	if err != nil {
		return err
	}
	tmpPath := filepath.Join(filepath.Dir(executablePath), ".bubble-world-clock-update")
	if err := copyFile(newBinaryPath, tmpPath); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, 0o755); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, executablePath); err != nil {
		return err
	}
	return nil
}

func copyFile(source, destination string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.Create(destination)
	if err != nil {
		return err
	}
	if _, err := io.Copy(output, input); err != nil {
		output.Close()
		return err
	}
	return output.Close()
}

func runUpdate() int {
	current := currentVersion()
	if current == "dev" || current == "(devel)" {
		fmt.Println("This build is not a released version, so no automatic update is available.")
		return 0
	}
	release, err := fetchLatestRelease()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not check for updates: %v\n", err)
		return 1
	}
	if compareVersions(current, release.TagName) >= 0 {
		fmt.Printf("Bubble World Clock is already up to date (%s).\n", current)
		return 0
	}
	assetName := releaseAssetName(runtime.GOOS, runtime.GOARCH, release.TagName)
	if assetName == "" {
		fmt.Fprintf(os.Stderr, "This platform does not have a published binary for %s/%s.\n", runtime.GOOS, runtime.GOARCH)
		return 1
	}
	var assetURL, checksumURL string
	for _, asset := range release.Assets {
		switch asset.Name {
		case assetName:
			assetURL = asset.BrowserDownloadURL
		case "checksums.txt":
			checksumURL = asset.BrowserDownloadURL
		}
	}
	if assetURL == "" || checksumURL == "" {
		fmt.Fprintf(os.Stderr, "The latest release is missing the %s asset or checksum file.\n", assetName)
		return 1
	}
	tempDir, err := os.MkdirTemp("", "bubbleworldclock-update-")
	if err != nil {
		return 1
	}
	defer os.RemoveAll(tempDir)
	archivePath := filepath.Join(tempDir, assetName)
	checksumPath := filepath.Join(tempDir, "checksums.txt")
	fmt.Println("Checking for updates...")
	if err := downloadFile(assetURL, archivePath); err != nil {
		fmt.Fprintf(os.Stderr, "Download failed: %v\n", err)
		return 1
	}
	if err := downloadFile(checksumURL, checksumPath); err != nil {
		fmt.Fprintf(os.Stderr, "Checksum download failed: %v\n", err)
		return 1
	}
	if err := verifyChecksum(checksumPath, assetName); err != nil {
		fmt.Fprintf(os.Stderr, "Checksum verification failed: %v\n", err)
		return 1
	}
	newBinaryPath, err := extractReleaseArchive(archivePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Archive extraction failed: %v\n", err)
		return 1
	}
	if err := updateBinary(newBinaryPath); err != nil {
		fmt.Fprintf(os.Stderr, "Could not install update: %v\n", err)
		return 1
	}
	fmt.Printf("Updated Bubble World Clock to %s.\n", release.TagName)
	return 0
}
