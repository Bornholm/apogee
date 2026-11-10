package client

import (
	"testing"
	"time"

	"apogee/internal/sim"
)

func TestWeatherIsSharedAndVaried(t *testing.T) {
	ticks := uint64(spell / sim.TickDuration)
	rainy := 0
	const spells = 400
	for n := range uint64(spells) {
		w := weatherAt(n*ticks+ticks/2, "day", 5*time.Minute)
		if w != weatherAt(n*ticks+ticks/2, "day", 5*time.Minute) {
			t.Fatal("the same moment gave two weathers")
		}
		if w.rain > 0 {
			rainy++
		}
		if w.wind <= 0 || w.wind > 1 {
			t.Fatalf("wind %v", w.wind)
		}
	}
	if share := float64(rainy) / spells; share < 0.15 || share > 0.35 {
		t.Fatalf("rain in %.0f %% of the spells", share*100)
	}
}

func TestWeatherChangesSmoothly(t *testing.T) {
	prev := weatherAt(0, "day", 10*time.Minute)
	for tick := uint64(1); tick < 50000; tick++ {
		w := weatherAt(tick, "day", 10*time.Minute)
		if d := w.rain - prev.rain; d > 0.01 || d < -0.01 {
			t.Fatalf("the rain jumps by %.2f at tick %d", d, tick)
		}
		prev = w
	}
}

func TestFogLiesOnlyInTheMorning(t *testing.T) {
	foggy := false
	for day := range uint64(20) {
		tick := day * uint64(30*time.Minute/sim.TickDuration)
		if weatherAt(tick, "night", time.Minute).fog > 0 || weatherAt(tick, "day", time.Minute).fog > 0.01 {
			t.Fatal("fog at night or late in the day")
		}
		foggy = foggy || weatherAt(tick, "dawn", time.Minute).fog > 0
	}
	if !foggy {
		t.Fatal("no foggy dawn in twenty days")
	}
}
