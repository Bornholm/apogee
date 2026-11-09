package client

import (
	"testing"

	"apogee/internal/geo"
)

func TestParticlesRiseFallAndVanish(t *testing.T) {
	var p particles
	for j := building; j <= channel; j++ {
		w := walker{x: 5, y: 5, state: working, work: spot{job: j, centre: geo.Vec2{X: 5, Y: 2}, radius: 6}}
		for range 600 {
			p.emit(&w, 1.0/60)
		}
	}
	if len(p.ps) == 0 {
		t.Fatal("ten seconds of work raised no particle")
	}
	if len(p.ps) > maxParticles {
		t.Fatalf("%d particles, cap is %d", len(p.ps), maxParticles)
	}
	for range 60 * 15 { // fifteen seconds later
		p.step(1.0 / 60)
		for _, q := range p.ps {
			if q.z < 0 {
				t.Fatalf("a particle sank below the ground: z = %v", q.z)
			}
		}
	}
	if len(p.ps) != 0 {
		t.Fatalf("%d particles still alive long after the work stopped", len(p.ps))
	}
}

func TestIdleVillagersRaiseNothing(t *testing.T) {
	var p particles
	w := walker{state: working, work: spot{job: idle}}
	for range 600 {
		p.emit(&w, 1.0/60)
	}
	if len(p.ps) != 0 {
		t.Fatalf("an idle villager raised %d particles", len(p.ps))
	}
}
