package git

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// DetectProjectSlug tries to derive a CircleCI project slug from the git remote.
// Returns slugs in the form "gh/org/repo" or "bb/org/repo".
func DetectProjectSlug() (string, error) {
	out, err := exec.Command("git", "remote", "get-url", "origin").Output()
	if err != nil {
		return "", fmt.Errorf("could not get git remote: %w", err)
	}
	return parseRemoteURL(strings.TrimSpace(string(out)))
}

func parseRemoteURL(raw string) (string, error) {
	// SSH: git@github.com:org/repo.git
	sshRe := regexp.MustCompile(`git@([^:]+):([^/]+)/(.+?)(?:\.git)?$`)
	if m := sshRe.FindStringSubmatch(raw); m != nil {
		return buildSlug(m[1], m[2], m[3])
	}

	// HTTPS: https://github.com/org/repo.git
	httpsRe := regexp.MustCompile(`https?://([^/]+)/([^/]+)/(.+?)(?:\.git)?$`)
	if m := httpsRe.FindStringSubmatch(raw); m != nil {
		return buildSlug(m[1], m[2], m[3])
	}

	return "", fmt.Errorf("unrecognised remote URL format: %s", raw)
}

func buildSlug(host, org, repo string) (string, error) {
	var vcs string
	switch {
	case strings.Contains(host, "github"):
		vcs = "gh"
	case strings.Contains(host, "bitbucket"):
		vcs = "bb"
	case strings.Contains(host, "gitlab"):
		vcs = "gl"
	default:
		return "", fmt.Errorf("unsupported VCS host: %s", host)
	}
	return fmt.Sprintf("%s/%s/%s", vcs, org, repo), nil
}
