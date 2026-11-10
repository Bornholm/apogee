package geo

import (
	"math"
	"testing"
)

func TestRiversRunFromInlandToTheFrontShore(t *testing.T) {
	for seed := uint64(1); seed <= 20; seed++ {
		rivers := Rivers(seed, 40)
		if len(rivers) == 0 {
			t.Fatalf("seed %d: no stream", seed)
		}
		outline := Outline(seed)
		for _, path := range rivers {
			mouth := path[len(path)-1]
			if e := EdgeDistance(outline, 40, mouth); math.Abs(e) > 0.01 {
				t.Fatalf("seed %d: the mouth is %.2f from the shore", seed, e)
			}
			if mouth.Y <= 0 {
				t.Fatalf("seed %d: the stream falls behind the island", seed)
			}
			if path[0].Len() < 12 {
				t.Fatalf("seed %d: the spring is by the heart", seed)
			}
		}
	}
}

func TestRiversKeepTheirCourseAsTheIslandGrows(t *testing.T) {
	small, big := Rivers(7, 35), Rivers(7, 55)
	for i := range small {
		inner := small[i][:len(small[i])-1]
		for j, p := range inner {
			if q := big[i][j]; p.Dist(q) > 1e-9 {
				t.Fatalf("stream %d moved at point %d: %v then %v", i, j, p, q)
			}
		}
		if len(big[i]) <= len(small[i]) {
			t.Fatal("the stream does not reach the new shore")
		}
	}
}
