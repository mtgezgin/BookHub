package ratings

// WeightedScore, IMDb tarzı Bayes ağırlıklı puan:
//
//	WR = (v / (v + m)) · R  +  (m / (v + m)) · C
//
//	R = kitabın ortalaması, v = kitabın oy sayısı,
//	m = "güvenilir sayılmak için gereken" oy sayısı (ayar), C = tüm kitapların ortalaması.
//
// SEZGİ: her kitap, sanki m tane "ortalama (C)" oyla başlıyormuş gibi davranır. 1 oyla 10
// alan kitap: 1 gerçek 10 + m sanal C → C'ye yakın kalır. 2000 oyla 8.5 alan kitap: sanal
// oylar gerçeklerin yanında kaybolur → 8.5'e yakın. Az veri = ihtiyatlı tahmin.
//
// SAF FONKSİYON: DB'yi, HTTP'yi, saati bilmez. Bu yüzden projenin "bayrak" tablo testi
// burada: formülün her kenarı (v=0, m=0, v→∞) DB'siz, milisaniyede doğrulanır. Top-rated
// sorgusu aynı formülü SQL'de hesaplıyor (sıralama DB'de, LIMIT ile); entegrasyon testi
// SQL sonucunun bu fonksiyonla AYNI olduğunu kontrol ediyor — spesifikasyon burası.
func WeightedScore(r float64, v, m int64, c float64) float64 {
	if v <= 0 {
		return c // hiç oy yok: tek bilgimiz genel ortalama
	}
	if m < 0 {
		m = 0
	}
	vf, mf := float64(v), float64(m)
	return (vf/(vf+mf))*r + (mf/(vf+mf))*c
}
