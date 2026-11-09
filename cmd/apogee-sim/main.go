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
	flag.Int("every", 1, "print one line every that many hours")
	flag.Parse()

	pie, err := parsePie(*pieFlag)
	if err != nil {
		log.Fatal(err)
	}
	var sky sim.Sky
	sky.Seed = *seed
	owner := "banc"
	sky.Found(owner, "Banc d'essai", *seed, 0)
	is := &sky.Islands[0]
	if err := is.Civ.SetPie(pie); err != nil {
		log.Fatal(err)
	}

	every := flag.Lookup("every").Value.(flag.Getter).Get().(int)
	fmt.Println("heure  pop  déf  ass  bois pierre vivres aether  bât  paliers  puiss. orbite cœur  culture")
	perHour := int(3600 / sim.TickDuration.Seconds())
	for h := 1; h <= *hours; h++ {
		for range perHour {
			sky.Step()
		}
		id, alive := sky.IslandOf(owner)
		if !alive {
			fmt.Printf("%5d  (l'île est tombée, le peuple attend l'aube)\n", h)
			continue
		}
		is = sky.Island(id)
		if is.Civ.Pie != pie {
			is.Civ.SetPie(pie) // a reborn people hears the same god
		}
		if h%every != 0 {
			continue
		}
		c := &is.Civ
		fmt.Printf("%5d %4.0f %4.0f %4.0f %5.0f %6.0f %6.0f %6.0f %4d  %d/%d/%d/%d %7.0f %6.0f %4.0f%%  %s gén. %d\n",
			h, c.Population, c.Defenders, c.Assailants,
			c.Stock[sim.Wood], c.Stock[sim.Stone], c.Stock[sim.Food], c.Stock[sim.Aether],
			len(c.Buildings),
			c.Tech.Tier(sim.Weapons), c.Tech.Tier(sim.Walls), c.Tech.Tier(sim.Industry), c.Tech.Tier(sim.Arcana),
			c.Power(), is.Radius, c.CoreHealth*100, c.Dominant().Name(), is.Generation)
	}
	for _, a := range sky.Annals {
		fmt.Printf("Annales : %s (gén. %d), née à %s, tombée à %s, apogée %.0f, %d paliers, %s — %s\n",
			a.Name, a.Generation, clock(a.Born), clock(a.Fell), a.PeakPower, a.Tiers, a.Culture.Name(), a.Cause)
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
	return fmt.Sprintf("j%d %02d:%02d", s/86400, s/3600%24, s/60%60)
}
