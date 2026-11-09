// Command apogee-sim runs a civilisation without rendering and prints how it
// fares, hour by hour. It is the balancing workbench of GDD 14.4.
package main

import (
	"flag"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"

	"apogee/internal/sim"
)

func main() {
	seed := flag.Uint64("seed", 7, "island seed")
	hours := flag.Int("hours", 8, "simulated hours")
	pieFlag := flag.String("pie", "prosper=60,fortify=40", "intentions, e.g. prosper=40,study=60")
	chronicle := flag.Int("chronicle", 6, "chronicle lines to print at the end")
	flag.Parse()

	pie, err := parsePie(*pieFlag)
	if err != nil {
		log.Fatal(err)
	}
	var sky sim.Sky
	sky.AddIsland("banc d'essai", *seed, 0)
	is := &sky.Islands[0]
	if err := is.Civ.SetPie(pie); err != nil {
		log.Fatal(err)
	}

	fmt.Println("heure  pop  déf  ass  bois pierre vivres aether  savoir  bât  culture dominante")
	perHour := int(3600 / sim.TickDuration.Seconds())
	for h := 1; h <= *hours; h++ {
		for range perHour {
			sky.Step()
		}
		c := &is.Civ
		fmt.Printf("%5d %4.0f %4.0f %4.0f %5.0f %6.0f %6.0f %6.0f %7.1f %4d  %s\n",
			h, c.Population, c.Defenders, c.Assailants,
			c.Stock[sim.Wood], c.Stock[sim.Stone], c.Stock[sim.Food], c.Stock[sim.Aether],
			c.Knowledge, len(c.Buildings), c.Dominant().Name())
	}

	c := &is.Civ
	counts := map[string]int{}
	for _, b := range c.Buildings {
		counts[sim.Specs[b.Kind].Name]++
	}
	names := make([]string, 0, len(counts))
	for n := range counts {
		names = append(names, n)
	}
	sort.Strings(names)
	fmt.Println("\nbâtiments :")
	for _, n := range names {
		fmt.Printf("  %2d × %s\n", counts[n], n)
	}
	fmt.Printf("taille de l'île : %.0f\n\nchronique :\n", c.Size)
	start := max(0, len(c.Chronicle)-*chronicle)
	for _, e := range c.Chronicle[start:] {
		fmt.Printf("  [%s] %s\n", clock(e.Tick), e.Text)
	}
}

func parsePie(s string) (sim.Pie, error) {
	var p sim.Pie
	names := map[string]sim.Intention{"prosper": sim.Prosper, "expand": sim.Expand,
		"fortify": sim.Fortify, "arm": sim.Arm, "study": sim.Study}
	for _, part := range strings.Split(s, ",") {
		k, v, ok := strings.Cut(strings.TrimSpace(part), "=")
		i, known := names[k]
		n, err := strconv.Atoi(v)
		if !ok || !known || err != nil {
			return p, fmt.Errorf("bad slice %q: want name=percent", part)
		}
		p[i] = n
	}
	return p, p.Validate()
}

func clock(tick uint64) string {
	s := int(float64(tick) * sim.TickDuration.Seconds())
	return fmt.Sprintf("%02d:%02d", s/3600, s/60%60)
}
