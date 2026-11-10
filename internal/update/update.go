// Package update tells whether a newer Apogée was published on GitHub, so
// that the game can point the player to it (GDD 14). It sends nothing about
// the player: one anonymous request to the public API, at launch.
package update

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// LatestURL is the GitHub API address of the latest release.
const LatestURL = "https://api.github.com/repos/Bornholm/apogee/releases/latest"

// Release is a published version and the page to download it from.
type Release struct {
	Version string // without the leading v
	URL     string
}

// Latest asks the API at url for the latest release.
func Latest(ctx context.Context, client *http.Client, url string) (Release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "apogee-update-check")
	resp, err := client.Do(req)
	if err != nil {
		return Release{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Release{}, fmt.Errorf("github answered %s", resp.Status)
	}
	var body struct {
		Tag  string `json:"tag_name"`
		Page string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return Release{}, err
	}
	if _, ok := parse(body.Tag); !ok || !strings.HasPrefix(body.Page, "https://") {
		return Release{}, fmt.Errorf("unexpected release %q at %q", body.Tag, body.Page)
	}
	return Release{Version: strings.TrimPrefix(body.Tag, "v"), URL: body.Page}, nil
}

// Check returns the latest release when it is newer than current. A
// version that is not a release, such as "dev", is never checked.
func Check(ctx context.Context, current string) (Release, bool) {
	if _, ok := parse(current); !ok {
		return Release{}, false
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	r, err := Latest(ctx, http.DefaultClient, LatestURL)
	if err != nil || !Newer(r.Version, current) {
		return Release{}, false
	}
	return r, true
}

// Newer reports whether version a comes after version b; both must be
// releases (1.2.3, with or without a v).
func Newer(a, b string) bool {
	va, okA := parse(a)
	vb, okB := parse(b)
	if !okA || !okB {
		return false
	}
	for i := range va {
		if va[i] != vb[i] {
			return va[i] > vb[i]
		}
	}
	return false
}

// parse reads a release version, major.minor.patch; a pre-release
// (1.2.3-rc1) is not one.
func parse(v string) ([3]int, bool) {
	var out [3]int
	parts := strings.Split(strings.TrimPrefix(v, "v"), ".")
	if len(parts) != 3 {
		return out, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return out, false
		}
		out[i] = n
	}
	return out, true
}

// Open shows a page in the player's browser.
func Open(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait() // reap it
	return nil
}
