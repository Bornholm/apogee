package update

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewer(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"0.7.0", "0.6.0", true},
		{"v0.10.0", "0.9.3", true},
		{"1.0.0", "v0.99.99", true},
		{"0.6.0", "0.6.0", false},
		{"0.5.9", "0.6.0", false},
		{"0.7.0", "dev", false},
		{"0.7.0-rc1", "0.6.0", false},
		{"0.7.0", "5f9b48f", false},
	}
	for _, c := range cases {
		if got := Newer(c.a, c.b); got != c.want {
			t.Errorf("Newer(%q, %q) = %v", c.a, c.b, got)
		}
	}
}

func TestLatestReadsTheRelease(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != "apogee-update-check" {
			t.Errorf("user agent %q", r.Header.Get("User-Agent"))
		}
		w.Write([]byte(`{"tag_name":"v0.7.0","html_url":"https://github.com/Bornholm/apogee/releases/tag/v0.7.0","body":"…"}`))
	}))
	defer srv.Close()
	r, err := Latest(context.Background(), srv.Client(), srv.URL)
	if err != nil || r.Version != "0.7.0" || r.URL != "https://github.com/Bornholm/apogee/releases/tag/v0.7.0" {
		t.Fatalf("got %+v, %v", r, err)
	}
}

func TestLatestRejectsOddAnswers(t *testing.T) {
	for _, body := range []string{`{"tag_name":"nightly","html_url":"https://x"}`, `{"tag_name":"v1.0.0","html_url":"javascript:alert(1)"}`, `not json`} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(body)) }))
		if _, err := Latest(context.Background(), srv.Client(), srv.URL); err == nil {
			t.Errorf("accepted %s", body)
		}
		srv.Close()
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "limit", http.StatusForbidden) }))
	defer srv.Close()
	if _, err := Latest(context.Background(), srv.Client(), srv.URL); err == nil {
		t.Error("accepted a refusal")
	}
}

func TestADevBuildNeverChecks(t *testing.T) {
	if _, ok := Check(context.Background(), "dev"); ok {
		t.Fatal("a dev build found an update")
	}
}

func TestWatchAsksAgainAndTellsEachNewerRelease(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	answers := []string{"0.6.2", "0.6.2", "0.6.3", "0.6.3", "0.7.0"}
	asked := 0
	check := func(context.Context, string) (Release, bool) {
		v := answers[min(asked, len(answers)-1)]
		asked++
		if asked >= len(answers) {
			defer cancel()
		}
		return Release{Version: v, URL: "https://x/" + v}, Newer(v, "0.6.1")
	}
	var told []string
	Watch(ctx, "0.6.1", time.Millisecond, check, func(r Release) { told = append(told, r.Version) })
	if asked < len(answers) {
		t.Fatalf("asked %d times", asked)
	}
	if len(told) != 3 || told[0] != "0.6.2" || told[1] != "0.6.3" || told[2] != "0.7.0" {
		t.Fatalf("told %v, want each newer release once", told)
	}
}

func TestWatchNeverAsksForADevBuild(t *testing.T) {
	Watch(context.Background(), "dev", time.Millisecond, func(context.Context, string) (Release, bool) {
		t.Fatal("a dev build asked")
		return Release{}, false
	}, func(Release) {})
}
