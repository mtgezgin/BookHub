package ratings

import (
	"math"
	"testing"
)

// TestWeightedScore — projenin bayrak tablo testi (PROJECT_BRIEF §6.7'deki örnekler dahil).
func TestWeightedScore(t *testing.T) {
	tests := []struct {
		name string
		r    float64
		v, m int64
		c    float64
		want float64
	}{
		// Brief'in örneği (m=100): az oylu 10'luk kitap C'ye çekilir, çok oylu 8.5 öne geçer.
		{"brief A: 1 oy, 10 puan, m=100", 10, 1, 100, 7.0, 7.0297},
		{"brief B: 2000 oy, 8.5 puan, m=100", 8.5, 2000, 100, 7.0, 8.4286},
		// Bizim ayarımız (m=10): küçük veri setinde her skor C'ye çökmesin diye.
		{"A, m=10", 10, 1, 10, 7.0, 7.2727},
		{"B, m=10", 8.5, 2000, 10, 7.0, 8.4925},
		// Kenarlar.
		{"hiç oy yok → C", 9, 0, 10, 6.5, 6.5},
		{"negatif v → C", 9, -3, 10, 6.5, 6.5},
		{"m=0 → saf ortalama", 9, 3, 0, 6.5, 9},
		{"negatif m → 0 sayılır", 9, 3, -5, 6.5, 9},
		{"v = m → tam ortası", 9, 10, 10, 5, 7},
		{"R = C → her zaman C", 7, 42, 10, 7, 7},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := WeightedScore(tt.r, tt.v, tt.m, tt.c)
			if math.Abs(got-tt.want) > 0.0001 {
				t.Errorf("WeightedScore(%v, %d, %d, %v) = %.4f, istenen %.4f", tt.r, tt.v, tt.m, tt.c, got, tt.want)
			}
		})
	}
}

// TestWeightedScoreRanking: asıl iddia sıralama — brief'in iki kitabında B, A'nın önünde.
func TestWeightedScoreRanking(t *testing.T) {
	for _, m := range []int64{10, 100} {
		a := WeightedScore(10, 1, m, 7)
		b := WeightedScore(8.5, 2000, m, 7)
		if b <= a {
			t.Errorf("m=%d: 2000 oyla 8.5 (%.3f), 1 oyla 10'un (%.3f) önünde olmalı", m, b, a)
		}
	}
}

// TestWeightedScoreConvergence: oy sayısı arttıkça skor kitabın kendi ortalamasına yaklaşır
// ve hep R ile C arasında kalır (tekdüze/monoton davranış).
func TestWeightedScoreConvergence(t *testing.T) {
	const r, c, m = 9.0, 6.0, 10
	prev := c
	for _, v := range []int64{1, 5, 10, 100, 1000, 100000} {
		got := WeightedScore(r, v, m, c)
		if got < prev || got < c || got > r {
			t.Errorf("v=%d: %.4f monoton artmalı ve [%v, %v] içinde kalmalı (önceki %.4f)", v, got, c, r, prev)
		}
		prev = got
	}
	if math.Abs(prev-r) > 0.01 {
		t.Errorf("çok oyda R'ye yakınsamalı: %.4f", prev)
	}
}
