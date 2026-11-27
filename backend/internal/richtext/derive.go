package richtext

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
)

// Text, belgenin düz metni: her paragraf bir satır, satırlar "\n" ile ayrılır. Liste maddeleri ve
// alıntılar da paragraflarının sırasıyla girer; biçim işaretleri (madde imi, numara) metne yazılmaz.
// reviews.body kolonuna bu yazılır (uzunluk sınırı, eski arayüz, ileride arama).
func (d Doc) Text() string {
	var lines []string
	walk(d.root, func(n node) {
		if n.Type != typeParagraph {
			return
		}
		var b strings.Builder
		for _, t := range n.Content {
			b.WriteString(t.Text)
		}
		lines = append(lines, b.String())
	})
	return strings.Join(lines, "\n")
}

// HasSpoiler, belgede spoiler işaretli metin var mı. reviews.has_spoiler kolonuna bu yazılır;
// istemcinin söylediğine bakılmaz.
func (d Doc) HasSpoiler() bool {
	found := false
	walk(d.root, func(n node) {
		if slices.Contains(n.Marks, mark{Type: MarkSpoiler}) {
			found = true
		}
	})
	return found
}

// JSON, belgenin kanonik JSON'u; veritabanına ve cevaba bu yazılır. Sıfır değerli Doc için nil
// döner (SQL NULL: "belgesi yok").
func (d Doc) JSON() json.RawMessage {
	if d.root.Type == "" {
		return nil
	}
	out, err := json.Marshal(d.root)
	if err != nil {
		// node yalnız metin ve dilim içerir; Marshal'ın hata verebileceği bir alan yok.
		panic("richtext: belge JSON'a çevrilemedi: " + err.Error())
	}
	return out
}

// Equal, iki belge aynı mı. Belgeler kanonik olduğu için ağaç karşılaştırması yeter: işaret sırası ve
// metnin düğümlere nasıl bölündüğü fark yaratmaz. "Kaydet'e iki kez basmak düzenleme değildir"
// kuralı bununla işler.
func Equal(a, b Doc) bool {
	return reflect.DeepEqual(a.root, b.root)
}

// FromPlain, düz metinden belge üretir: her satır bir paragraf; spoiler ise bütün metin spoiler
// işareti alır. Belgesi olmayan eski incelemeleri okurken (spec karar 6) ve eski gövdeyle gelen
// isteklerde (karar 7) kullanılır.
//
// Metin Parse'ın kurallarına uydurulur: satır sonları "\n"e iner, öteki denetim karakterleri atılır.
// Sonuç yine de Parse'ın sınırlarını aşabilir ya da boş olabilir; yazmadan önce
// Parse(doc.JSON()) ile doğrulanır.
func FromPlain(body string, spoiler bool) Doc {
	body = strings.NewReplacer("\r\n", "\n", "\r", "\n").Replace(body)
	var marks []mark
	if spoiler {
		marks = []mark{{Type: MarkSpoiler}}
	}
	root := node{Type: typeDoc}
	for line := range strings.SplitSeq(body, "\n") {
		line = strings.Map(func(r rune) rune {
			if forbiddenRune(r) {
				return -1
			}
			return r
		}, line)
		p := node{Type: typeParagraph}
		if line != "" {
			p.Content = []node{{Type: typeText, Text: line, Marks: marks}}
		}
		root.Content = append(root.Content, p)
	}
	return Doc{root: root}
}

// walk, ağacı belge sırasıyla gezer.
func walk(n node, visit func(node)) {
	visit(n)
	for _, c := range n.Content {
		walk(c, visit)
	}
}
