package richtext

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
)

// para, tek paragraflı belge JSON'u üretir; içerik paragrafın content dizisinin içidir.
func para(content string) string {
	return `{"type":"doc","content":[{"type":"paragraph","content":[` + content + `]}]}`
}

// TestVectors: geçerli belgeler ve onlardan türetilenler. Dosya ortak: arayüzdeki düz metin ve sayaç
// hesabı (adım 5) aynı örneklerle sınanır; iki taraf ayrışırsa okur sayaçta yer varken 400 alır.
func TestVectors(t *testing.T) {
	raw, err := os.ReadFile("testdata/vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	var vectors []struct {
		Name       string          `json:"name"`
		Doc        json.RawMessage `json:"doc"`
		Text       string          `json:"text"`
		HasSpoiler bool            `json:"has_spoiler"`
	}
	if err := json.Unmarshal(raw, &vectors); err != nil {
		t.Fatal(err)
	}
	for _, v := range vectors {
		t.Run(v.Name, func(t *testing.T) {
			d, err := Parse(v.Doc)
			if err != nil {
				t.Fatalf("geçerli belge reddedildi: %v", err)
			}
			if got := d.Text(); got != v.Text {
				t.Errorf("düz metin = %q, beklenen %q", got, v.Text)
			}
			if got := d.HasSpoiler(); got != v.HasSpoiler {
				t.Errorf("spoiler = %v, beklenen %v", got, v.HasSpoiler)
			}
		})
	}
}

// TestParseRejects: izin listesinin dışındaki her şey reddedilir. XSS denemeleri de burada: belge HTML
// olmadığı için "etiket" bir metin parçasıdır, ama etiket taşıyabilecek her alan (tür, işaret,
// öznitelik) ayrı ayrı kapalıdır.
func TestParseRejects(t *testing.T) {
	// Tam sınırın bir üstü: doc (1) + MaxDepth-1 alıntı + paragraf = MaxDepth+1 kat.
	deep := strings.Repeat(`{"type":"blockquote","content":[`, MaxDepth-1) + `{"type":"paragraph"}` + strings.Repeat(`]}`, MaxDepth-1)
	manyNodes := `{"type":"doc","content":[` + strings.TrimSuffix(strings.Repeat(`{"type":"paragraph"},`, MaxNodes), ",") + `]}`
	for _, c := range []struct {
		name, doc string
		want      error
	}{
		{"JSON değil", `<p>merhaba</p>`, ErrInvalid},
		{"JSON null", `null`, ErrInvalid},
		{"dizi", `[]`, ErrInvalid},
		{"metin", `"merhaba"`, ErrInvalid},
		{"arkasında ikinci değer", para(`{"type":"text","text":"a"}`) + `{}`, ErrInvalid},
		{"kök doc değil", `{"type":"paragraph","content":[{"type":"text","text":"a"}]}`, ErrInvalid},
		{"type yok", `{"content":[{"type":"paragraph"}]}`, ErrInvalid},
		{"type metin değil", `{"type":1,"content":[]}`, ErrInvalid},
		{"alan adı büyük harfle (Type)", `{"Type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"a"}]}]}`, ErrInvalid},
		{"bilinmeyen düğüm: başlık", `{"type":"doc","content":[{"type":"heading","content":[{"type":"text","text":"a"}]}]}`, ErrInvalid},
		{"bilinmeyen düğüm: resim", `{"type":"doc","content":[{"type":"image"},{"type":"paragraph","content":[{"type":"text","text":"a"}]}]}`, ErrInvalid},
		{"bilinmeyen düğüm: satır sonu", para(`{"type":"text","text":"a"},{"type":"hardBreak"}`), ErrInvalid},
		{"bilinmeyen işaret: bağlantı", para(`{"type":"text","text":"a","marks":[{"type":"link"}]}`), ErrInvalid},
		{"işarette öznitelik", para(`{"type":"text","text":"a","marks":[{"type":"bold","attrs":{"href":"javascript:alert(1)"}}]}`), ErrInvalid},
		{"aynı işaret iki kez", para(`{"type":"text","text":"a","marks":[{"type":"bold"},{"type":"bold"}]}`), ErrInvalid},
		{"işaret nesne değil", para(`{"type":"text","text":"a","marks":["bold"]}`), ErrInvalid},
		{"düğümde attrs", `{"type":"doc","content":[{"type":"orderedList","attrs":{"start":1,"type":null},"content":[{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"a"}]}]}]}]}`, ErrInvalid},
		{"düğümde bilinmeyen alan", `{"type":"doc","content":[{"type":"paragraph","style":"color:red","content":[{"type":"text","text":"a"}]}]}`, ErrInvalid},
		{"metinde bilinmeyen alan", para(`{"type":"text","text":"a","html":"<script>alert(1)</script>"}`), ErrInvalid},
		{"paragrafta marks", `{"type":"doc","content":[{"type":"paragraph","marks":[{"type":"bold"}],"content":[{"type":"text","text":"a"}]}]}`, ErrInvalid},
		{"content dizi değil", `{"type":"doc","content":{"type":"paragraph"}}`, ErrInvalid},
		{"boş metin düğümü", para(`{"type":"text","text":""}`), ErrInvalid},
		{"text metin değil", para(`{"type":"text","text":5}`), ErrInvalid},
		{"metinde NUL", para(`{"type":"text","text":"a\u0000b"}`), ErrInvalid},
		{"metinde satır sonu", para(`{"type":"text","text":"a\nb"}`), ErrInvalid},
		{"metinde C1 denetim karakteri", para(`{"type":"text","text":"a\u0085b"}`), ErrInvalid},
		{"doc içinde doğrudan metin", `{"type":"doc","content":[{"type":"text","text":"a"}]}`, ErrInvalid},
		{"paragraf içinde paragraf", para(`{"type":"paragraph"}`), ErrInvalid},
		{"doc içinde doc", `{"type":"doc","content":[{"type":"doc","content":[{"type":"paragraph"}]}]}`, ErrInvalid},
		{"boş doc", `{"type":"doc"}`, ErrInvalid},
		{"boş alıntı", `{"type":"doc","content":[{"type":"blockquote"}]}`, ErrInvalid},
		{"boş liste", `{"type":"doc","content":[{"type":"bulletList","content":[]}]}`, ErrInvalid},
		{"listede madde dışı çocuk", `{"type":"doc","content":[{"type":"bulletList","content":[{"type":"paragraph"}]}]}`, ErrInvalid},
		{"madde paragrafla başlamıyor", `{"type":"doc","content":[{"type":"bulletList","content":[{"type":"listItem","content":[{"type":"blockquote","content":[{"type":"paragraph"}]}]}]}]}`, ErrInvalid},
		{"liste dışında madde", `{"type":"doc","content":[{"type":"listItem","content":[{"type":"paragraph"}]}]}`, ErrInvalid},

		{"metin yok: boş paragraf", `{"type":"doc","content":[{"type":"paragraph"}]}`, ErrEmpty},
		{"metin yok: yalnız boşluk", para(`{"type":"text","text":" \t "}`), ErrEmpty},

		{"5001 karakter", para(`{"type":"text","text":"` + strings.Repeat("ş", MaxTextLen+1) + `"}`), ErrLimit},
		{"satır sonları da sayılır", `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"` + strings.Repeat("a", MaxTextLen) + `"}]},{"type":"paragraph","content":[{"type":"text","text":"b"}]}]}`, ErrLimit},
		{"fazla derin", `{"type":"doc","content":[` + deep + `]}`, ErrLimit},
		{"fazla düğüm", manyNodes, ErrLimit},
		// Başka hiçbir sınıra takılmayan küçük bir belge, arkasında boşluk: yalnız boyut sınırı keser.
		{"fazla büyük", para(`{"type":"text","text":"a"}`) + strings.Repeat(" ", MaxBytes), ErrLimit},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, err := Parse([]byte(c.doc))
			if !errors.Is(err, c.want) {
				t.Fatalf("hata = %v, beklenen %v", err, c.want)
			}
			if !errors.Is(err, ErrInvalid) {
				t.Errorf("her hata ErrInvalid'i sarmalı: %v", err)
			}
			// Yapı hatası sınır ya da boşluk hatası gibi görünmemeli (handler mesajı ona göre seçer).
			if c.want == ErrInvalid && (errors.Is(err, ErrLimit) || errors.Is(err, ErrEmpty)) {
				t.Errorf("yapı hatası bekleniyordu: %v", err)
			}
		})
	}
}

// TestParseErrorHidesInput: hata mesajı istemcinin gönderdiği değeri geri yazmaz (cevaba girer).
func TestParseErrorHidesInput(t *testing.T) {
	_, err := Parse([]byte(`{"type":"doc","content":[{"type":"<img src=x onerror=alert(1)>"}]}`))
	if err == nil || strings.Contains(err.Error(), "onerror") {
		t.Fatalf("mesaj girdiyi taşımamalı: %v", err)
	}
	if !strings.Contains(err.Error(), "doc.content[0]") {
		t.Errorf("mesaj sorunun yerini söylemeli: %v", err)
	}
}

// TestParseLimitsAccepted: sınırların tam üstündeki belgeler kabul edilir.
func TestParseLimitsAccepted(t *testing.T) {
	deep := strings.Repeat(`{"type":"blockquote","content":[`, MaxDepth-3) + `{"type":"paragraph","content":[{"type":"text","text":"a"}]}` + strings.Repeat(`]}`, MaxDepth-3)
	for name, doc := range map[string]string{
		"5000 karakter (çok baytlı)": para(`{"type":"text","text":"` + strings.Repeat("ş", MaxTextLen) + `"}`),
		"en derin":                   `{"type":"doc","content":[` + deep + `]}`,
		"en çok düğüm":               `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"a"}]},` + strings.TrimSuffix(strings.Repeat(`{"type":"paragraph"},`, MaxNodes-3), ",") + `]}`,
	} {
		if _, err := Parse([]byte(doc)); err != nil {
			t.Errorf("%s: kabul edilmeliydi: %v", name, err)
		}
	}
}

// TestCanonical: aynı içeriğin farklı yazımları tek belgeye iner; saklanan JSON kanoniktir.
func TestCanonical(t *testing.T) {
	want := para(`{"type":"text","marks":[{"type":"bold"},{"type":"spoiler"}],"text":"uşak"}`)
	for name, in := range map[string]string{
		"işaretler ters sırada":       para(`{"type":"text","text":"uşak","marks":[{"type":"spoiler"},{"type":"bold"}]}`),
		"metin iki düğüme bölünmüş":   para(`{"type":"text","text":"uş","marks":[{"type":"bold"},{"type":"spoiler"}]},{"type":"text","text":"ak","marks":[{"type":"spoiler"},{"type":"bold"}]}`),
		"boşluklu ve boş dizili JSON": ` { "content" : [ { "content" : [ { "marks" : [ {"type":"bold"}, {"type":"spoiler"} ], "text" : "uşak", "type" : "text" } ], "type" : "paragraph" } ], "type" : "doc" } `,
	} {
		d, err := Parse([]byte(in))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got := string(d.JSON()); got != want {
			t.Errorf("%s:\n kanonik = %s\n beklenen = %s", name, got, want)
		}
	}

	a, _ := Parse([]byte(para(`{"type":"text","text":"a","marks":[{"type":"italic"},{"type":"bold"}]}`)))
	b, _ := Parse([]byte(para(`{"type":"text","text":"a","marks":[{"type":"bold"},{"type":"italic"}]}`)))
	c, _ := Parse([]byte(para(`{"type":"text","text":"a","marks":[{"type":"bold"}]}`)))
	if !Equal(a, b) {
		t.Error("işaret sırası farkı belgeyi değiştirmemeli")
	}
	if Equal(a, c) {
		t.Error("işareti farklı belgeler eşit sayılmamalı")
	}
	// Boş content dizisi ile alanın hiç olmaması aynı belgedir.
	e1, _ := Parse([]byte(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"a"}]},{"type":"paragraph","content":[]}]}`))
	e2, _ := Parse([]byte(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"a"}]},{"type":"paragraph"}]}`))
	if !Equal(e1, e2) || string(e1.JSON()) != string(e2.JSON()) {
		t.Error("boş content ile olmayan content aynı belge olmalı")
	}
}

// TestHTMLIsText: belgedeki "etiket" metindir; kanonik JSON'da da kaçışlı durur.
func TestHTMLIsText(t *testing.T) {
	d, err := Parse([]byte(para(`{"type":"text","text":"<script>alert(1)</script>"}`)))
	if err != nil {
		t.Fatal(err)
	}
	if d.Text() != "<script>alert(1)</script>" {
		t.Errorf("metin aynen korunmalı: %q", d.Text())
	}
	if strings.Contains(string(d.JSON()), "<script>") {
		t.Errorf("kanonik JSON'da < ve > kaçışlı olmalı: %s", d.JSON())
	}
}

func TestFromPlain(t *testing.T) {
	for _, c := range []struct {
		name, body string
		spoiler    bool
		wantText   string
		wantJSON   string
	}{
		{"tek satır", "Güzel.", false, "Güzel.", para(`{"type":"text","text":"Güzel."}`)},
		{"spoiler bütün metne", "Katil uşak.", true, "Katil uşak.", para(`{"type":"text","marks":[{"type":"spoiler"}],"text":"Katil uşak."}`)},
		{"satırlar paragraf olur", "bir\n\niki", false, "bir\n\niki",
			`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"bir"}]},{"type":"paragraph"},{"type":"paragraph","content":[{"type":"text","text":"iki"}]}]}`},
		{"Windows satır sonu", "bir\r\niki\rüç", false, "bir\niki\nüç", ""},
		{"denetim karakteri atılır, sekme kalır", "a\x00b\tc\x7f", false, "ab\tc", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			d := FromPlain(c.body, c.spoiler)
			if got := d.Text(); got != c.wantText {
				t.Errorf("düz metin = %q, beklenen %q", got, c.wantText)
			}
			if d.HasSpoiler() != c.spoiler {
				t.Errorf("spoiler = %v, beklenen %v", d.HasSpoiler(), c.spoiler)
			}
			if c.wantJSON != "" && string(d.JSON()) != c.wantJSON {
				t.Errorf("JSON = %s\nbeklenen %s", d.JSON(), c.wantJSON)
			}
			parsed, err := Parse(d.JSON())
			if err != nil {
				t.Fatalf("FromPlain'in ürettiği belge doğrulamadan geçmeli: %v", err)
			}
			if !Equal(parsed, d) {
				t.Error("FromPlain'in ürettiği belge kanonik olmalı")
			}
		})
	}
	if _, err := Parse(FromPlain("  \n ", false).JSON()); !errors.Is(err, ErrEmpty) {
		t.Errorf("boş metinden üretilen belge ErrEmpty vermeli: %v", err)
	}
	if (Doc{}).JSON() != nil {
		t.Error("sıfır değerli belgenin JSON'u nil olmalı (SQL NULL)")
	}
}
