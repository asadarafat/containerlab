// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package version

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/hashicorp/go-version"
	"github.com/spf13/cobra"
	"github.com/srl-labs/containerlab/cmd/common"
)

const (
	downloadURL = "https://github.com/srl-labs/containerlab/raw/main/get.sh"
	tagsURL     = "https://api.github.com/repos/srl-labs/containerlab/tags"
)

// upgradeCmd represents the upgrade command.
var upgradeCmd = &cobra.Command{
	Use:     "upgrade",
	Short:   "upgrade containerlab to latest available version",
	PreRunE: common.CheckAndGetRootPrivs,
	RunE: func(_ *cobra.Command, args []string) error {
		// Determine the latest version tag using GitHub's API.
		latest, err := getLatestTag()
		if err != nil {
			return fmt.Errorf("failed to determine latest version: %w", err)
		}
		fmt.Printf("aarafat-tag: getting the latest version using GitHub tags method.... %s\n", latest)
		fmt.Printf("Latest tag: %s\n", latest)

		// Create a temporary file to hold the upgrade script.
		f, err := os.CreateTemp("", "containerlab")
		if err != nil {
			return fmt.Errorf("failed to create temp file: %w", err)
		}
		defer os.Remove(f.Name())

		// Download the upgrade script into the temp file.
		if err = downloadFile(downloadURL, f); err != nil {
			return fmt.Errorf("failed to download upgrade script: %w", err)
		}

		// Ensure the file is executable.
		if err = f.Chmod(0755); err != nil {
			return fmt.Errorf("failed to set script as executable: %w", err)
		}

		// Prepare and run the upgrade script.
		// The environment variable CLAB_VERSION is passed so that the script installs the latest version.
		c := exec.Command("sudo", "bash", f.Name())
		c.Env = append(os.Environ(), "CLAB_VERSION="+latest)
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		if err = c.Run(); err != nil {
			return fmt.Errorf("upgrade failed: %w", err)
		}

		return nil
	},
}

// tagInfo represents a minimal structure for GitHub tag API responses.
type tagInfo struct {
	Name string `json:"name"`
}

// getLatestTag retrieves the list of tags from GitHub and returns the latest valid version tag.
func getLatestTag() (string, error) {
	resp, err := http.Get(tagsURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var tags []tagInfo
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return "", err
	}

	var latestVersion *version.Version
	var latestTag string
	for _, t := range tags {
		// Filter out tags that do not start with "v" immediately followed by a digit.
		if !strings.HasPrefix(t.Name, "v") || len(t.Name) < 2 || (t.Name[1] < '0' || t.Name[1] > '9') {
			continue
		}

		// Remove the "v" prefix for proper semantic version parsing.
		vStr := strings.TrimPrefix(t.Name, "v")
		v, err := version.NewVersion(vStr)
		if err != nil {
			continue
		}
		if latestVersion == nil || v.GreaterThan(latestVersion) {
			latestVersion = v
			latestTag = t.Name
		}
	}

	if latestTag == "" {
		return "", fmt.Errorf("no valid version tag found")
	}

	return latestTag, nil
}

// downloadFile downloads a file from the specified URL and writes its contents to the provided file.
func downloadFile(url string, file *os.File) error {
	// Get the data.
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close() // skipcq: GO-S2307

	// Write the body to file.
	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return err
	}

	// Seek back to the beginning of the file.
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return err
	}

	return nil
}

func init() {
	VersionCmd.AddCommand(upgradeCmd)
}
