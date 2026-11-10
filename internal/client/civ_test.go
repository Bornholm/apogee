package client

import (
	"testing"
	"time"

	"apogee/internal/proto"
	"apogee/internal/sim"
)

func TestReservesHaveATrend(t *testing.T) {
	var g Game
	now := time.Unix(0, 0)
	o := &proto.Own{}
	o.Stock[sim.Wood] = 100
	g.logStock(o, now)
	if _, ok := g.trend(sim.Wood); ok {
		t.Fatal("a trend from one sample")
	}
	for i := 1; i <= 40; i++ {
		o.Stock[sim.Wood] = 100 + float64(i) // +1 every 15 s: +240 an hour
		g.logStock(o, now.Add(time.Duration(i)*stockEvery))
	}
	d, ok := g.trend(sim.Wood)
	if !ok || d < 230 || d > 250 {
		t.Fatalf("trend %.0f/h (%v), want 240", d, ok)
	}
	if span := g.stockLog[len(g.stockLog)-1].t.Sub(g.stockLog[0].t); span > stockSpan {
		t.Fatalf("the log spans %v", span)
	}
}

func TestSpanSpeaksPlainly(t *testing.T) {
	for d, want := range map[time.Duration]string{
		30 * time.Second:            "moins d'une minute",
		90 * time.Minute:            "1 heure",
		26 * time.Hour:              "1 jour et 2 heures",
		48 * time.Hour:              "2 jours",
		3*time.Minute + time.Second: "3 minutes",
	} {
		if got := span(d); got != want {
			t.Errorf("span(%v) = %q, want %q", d, got, want)
		}
	}
	if generation(3) != "Troisième génération" || generation(12) != "Génération 12" {
		t.Error("generations are misnamed")
	}
}

func TestCountdownToTheMinute(t *testing.T) {
	for ms, want := range map[int64]string{
		1:                    "1 min",
		47 * 60_000:          "47 min",
		(5*60 + 12) * 60_000: "5 h 12",
		12 * 3600_000:        "12 h 00",
	} {
		if got := countdown(ms); got != want {
			t.Errorf("countdown(%d) = %q, want %q", ms, got, want)
		}
	}
}
