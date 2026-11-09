// Command apogee-balance is the automated balancing bench of GDD 14.4: it
// runs whole skies without rendering, each holding one civilisation per
// strategy, and compares how long they live and how high they rise. A
// dominant strategy shows as a lifetime or a peak far above the others.
package main

import (
	"flag"
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"apogee/internal/sim"
)

// strategy is a pie whispered for a whole life, and after every rebirth.
type strategy struct {
	name string
	pie  sim.Pie
}

var strategies = []strategy{
	{"prospère", sim.Pie{sim.Prosper: 60, sim.Fortify: 40}},
	{"tout prospère", sim.Pie{sim.Prosper: 100}},
	{"rempart", sim.Pie{sim.Fortify: 70, sim.Prosper: 30}},
	{"guerrier", sim.Pie{sim.Arm: 50, sim.Prosper: 30, sim.Fortify: 20}},
	{"érudit", sim.Pie{sim.Study: 60, sim.Prosper: 40}},
	{"équilibré", sim.Pie{sim.Prosper: 40, sim.Fortify: 30, sim.Study: 30}},
	{"expansion", sim.Pie{sim.Expand: 50, sim.Prosper: 50}},
	{"bâtisseur", sim.Pie{sim.Prosper: 50, sim.Expand: 30, sim.Study: 20}},
}

// life is one civilisation's fate.
type life struct {
	strategy string
	days     float64
	peak     float64
	tiers    int
	byBoss   bool
	fell     bool
}

func main() {
	skies := flag.Int("skies", 16, "skies to simulate")
	days := flag.Float64("days", 8, "simulated days per sky")
	parallel := flag.Int("parallel", runtime.NumCPU(), "skies simulated at once")
	seed := flag.Uint64("seed", 1, "seed of the first sky")
	raiders := flag.Int("raiders", 0, "warlike AI gods added to each sky, to test the strategies under raids")
	flag.Parse()

	start := time.Now()
	results := make(chan []life)
	var wg sync.WaitGroup
	sem := make(chan struct{}, *parallel)
	for i := range *skies {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results <- runSky(*seed+uint64(i), *days, *raiders)
		}()
	}
	go func() {
		wg.Wait()
		close(results)
	}()
	var all []life
	done := 0
	for r := range results {
		all = append(all, r...)
		done++
		fmt.Fprintf(os.Stderr, "\rciels simulés : %d/%d", done, *skies)
	}
	fmt.Fprintf(os.Stderr, " en %s\n\n", time.Since(start).Round(time.Second))
	report(all, *skies, *days)
}

// runSky simulates one sky and returns every life that ended or was still
// running at the end.
func runSky(seed uint64, days float64, raiders int) []life {
	var s sim.Sky
	s.Seed = seed
	r := rand.New(rand.NewPCG(seed, 99))
	order := r.Perm(len(strategies))
	for k, i := range order {
		st := strategies[i]
		s.Found("bench/"+st.name, st.name, r.Uint64(), float64(k)/float64(len(order))*2*math.Pi)
	}
	for i := range raiders {
		s.FoundRival(fmt.Sprintf("Pillard %d", i+1), sim.Warlike, r.Uint64(), r.Float64()*2*math.Pi)
	}
	perHour := int(3600 / sim.TickDuration.Seconds())
	for range int(days * 24) {
		// A reborn people hears the same god: reapply each strategy.
		for _, st := range strategies {
			if id, ok := s.IslandOf("bench/" + st.name); ok {
				if is := s.Island(id); is.Civ.Pie != st.pie {
					is.Civ.SetPie(st.pie)
				}
			}
		}
		for range perHour {
			s.Step()
		}
	}
	var out []life
	for _, a := range s.Annals {
		if !strings.HasPrefix(a.Owner, "bench/") {
			continue
		}
		out = append(out, life{
			strategy: strings.TrimPrefix(a.Owner, "bench/"),
			days:     float64(a.Fell-a.Born) * sim.TickDuration.Seconds() / 86400,
			peak:     a.PeakPower, tiers: a.Tiers,
			byBoss: strings.Contains(a.Cause, "colosse"), fell: true,
		})
	}
	return out
}

func report(all []life, skies int, days float64) {
	type stats struct {
		n, boss           int
		days, peak, tiers []float64
	}
	by := map[string]*stats{}
	for _, st := range strategies {
		by[st.name] = &stats{}
	}
	for _, l := range all {
		s := by[l.strategy]
		s.n++
		if l.byBoss {
			s.boss++
		}
		s.days = append(s.days, l.days)
		s.peak = append(s.peak, l.peak)
		s.tiers = append(s.tiers, float64(l.tiers))
	}
	fmt.Printf("%d ciels de %.0f jours, %d civilisations tombées.\n", skies, days, len(all))
	fmt.Println("Une civilisation encore debout à la fin n'est pas comptée.")
	fmt.Println()
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "stratégie\tchutes\tvie médiane (j)\tvie min–max\tapogée médiane\tpaliers moyens\tpar le colosse")
	names := make([]string, 0, len(by))
	for n := range by {
		names = append(names, n)
	}
	sort.Slice(names, func(i, j int) bool { return median(by[names[i]].days) > median(by[names[j]].days) })
	for _, n := range names {
		s := by[n]
		if s.n == 0 {
			fmt.Fprintf(w, "%s\t0\t—\t—\t—\t—\t—\n", n)
			continue
		}
		lo, hi := minMax(s.days)
		fmt.Fprintf(w, "%s\t%d\t%.1f\t%.1f–%.1f\t%.0f\t%.1f\t%d %%\n", n, s.n, median(s.days), lo, hi,
			median(s.peak), mean(s.tiers), 100*s.boss/s.n)
	}
	w.Flush()
}

func median(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	c := append([]float64(nil), v...)
	sort.Float64s(c)
	if len(c)%2 == 1 {
		return c[len(c)/2]
	}
	return (c[len(c)/2-1] + c[len(c)/2]) / 2
}

func mean(v []float64) float64 {
	t := 0.0
	for _, x := range v {
		t += x
	}
	return t / float64(max(len(v), 1))
}

func minMax(v []float64) (float64, float64) {
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, x := range v {
		lo, hi = math.Min(lo, x), math.Max(hi, x)
	}
	return lo, hi
}
