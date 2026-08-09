package biceprpcclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
)

const downloadBaseURL = "https://downloads.bicep.azure.com"

// DownloadURL returns the Bicep CLI download URL for a version and the current platform.
func DownloadURL(ctx context.Context, version string) (string, error) {
	return downloadURLForPlatform(ctx, version, runtime.GOOS, runtime.GOARCH)
}

func downloadURLForPlatform(ctx context.Context, version, goos, goarch string) (string, error) {
	tag := "v" + version
	if version == "" {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadBaseURL+"/releases/latest", nil)
		if err != nil {
			return "", fmt.Errorf("create latest Bicep release request: %w", err)
		}

		response, err := http.DefaultClient.Do(request)
		if err != nil {
			return "", fmt.Errorf("get latest Bicep release: %w", err)
		}
		defer response.Body.Close()
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			return "", fmt.Errorf("get latest Bicep release: unexpected HTTP status %s", response.Status)
		}

		var release struct {
			TagName string `json:"tag_name"`
		}
		if err := json.NewDecoder(response.Body).Decode(&release); err != nil {
			return "", fmt.Errorf("decode latest Bicep release: %w", err)
		}
		if release.TagName == "" {
			return "", fmt.Errorf("decode latest Bicep release: response did not contain tag_name")
		}
		tag = release.TagName
	}

	artifact, err := artifactName(goos, goarch)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/%s/%s", downloadBaseURL, tag, artifact), nil
}

func artifactName(goos, goarch string) (string, error) {
	switch goos + "/" + goarch {
	case "windows/amd64":
		return "bicep-win-x64.exe", nil
	case "windows/arm64":
		return "bicep-win-arm64.exe", nil
	case "linux/amd64":
		return "bicep-linux-x64", nil
	case "linux/arm64":
		return "bicep-linux-arm64", nil
	case "darwin/amd64":
		return "bicep-osx-x64", nil
	case "darwin/arm64":
		return "bicep-osx-arm64", nil
	default:
		return "", fmt.Errorf("Bicep CLI is not available for platform %s and architecture %s", goos, goarch)
	}
}

// Install downloads the Bicep CLI into basePath and returns its executable path.
func Install(ctx context.Context, basePath, version string) (string, error) {
	if err := os.MkdirAll(basePath, 0o755); err != nil {
		return "", fmt.Errorf("create Bicep install directory: %w", err)
	}

	executableName := "bicep"
	if runtime.GOOS == "windows" {
		executableName += ".exe"
	}
	executablePath := filepath.Join(basePath, executableName)
	if _, err := os.Stat(executablePath); err == nil {
		return executablePath, nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("inspect Bicep executable: %w", err)
	}

	downloadURL, err := DownloadURL(ctx, version)
	if err != nil {
		return "", err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return "", fmt.Errorf("create Bicep download request: %w", err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("download Bicep CLI: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("download Bicep CLI: unexpected HTTP status %s", response.Status)
	}

	temporary, err := os.CreateTemp(basePath, "bicep-download-*")
	if err != nil {
		return "", fmt.Errorf("create temporary Bicep executable: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)

	if _, err := io.Copy(temporary, response.Body); err != nil {
		temporary.Close()
		return "", fmt.Errorf("write Bicep executable: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return "", fmt.Errorf("close Bicep executable: %w", err)
	}
	if err := os.Chmod(temporaryPath, 0o755); err != nil {
		return "", fmt.Errorf("make Bicep executable: %w", err)
	}
	if err := os.Rename(temporaryPath, executablePath); err != nil {
		return "", fmt.Errorf("install Bicep executable: %w", err)
	}

	return executablePath, nil
}
