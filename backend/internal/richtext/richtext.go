// Package richtext, incelemenin zengin metin belgesini doğrular ve ondan türetilen değerleri hesaplar
// (Faz 17C, REV-04).
//
// Belge HTML değildir: türü ve metni olan bir JSON ağacıdır (ProseMirror JSON'unun kısıtlı alt
// kümesi). Sayfaya hiçbir zaman HTML olarak basılmaz; arayüz ağacı kendi bileşenleriyle çizer.
//
// NEDEN SUNUCUDA: editör şemaya uysa da isteği herkes elle gönderebilir. Asıl kapı buradaki izin
// listesidir (spec karar 3): listede olmayan tür, işaret ya da alan → ErrInvalid. Sessizce ayıklama
// yok; neyin silindiği belirsiz kalmasın.
//
// BİÇİM (spec karar 2 ve adım 1'in kararları):
//
//	doc         → blok+
//	paragraph   → text*
//	blockquote  → blok+
//	bulletList  → listItem+
//	orderedList → listItem+
//	listItem    → paragraph blok*
//	text        → {"type":"text","text":"…","marks":[{"type":"bold"|"italic"|"spoiler"}]}
//
// "blok" = paragraph, blockquote, bulletList, orderedList. Hiçbir düğümde attrs yoktur.
//
// KANONİK HÂL: Parse'ın döndürdüğü Doc tek biçimlidir: işaretler sabit sırada, aynı işaretli komşu
// metinler birleşmiş, boş content ve marks alanları yok. Veritabanına Doc.JSON() yazılır; yani
// saklanan şey doğrulanan şeyin kendisidir, istemcinin gönderdiği baytlar değil.
package richtext

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Sınırlar (spec karar 4). Metin sınırı incelemenin bugünkü sınırıyla aynı (reviews.body CHECK'i);
// ötekiler doğrulamayı ve çizimi yoran belgeleri keser. Gerekçeleri ve ölçümleri spec adım 3'te.
const (
	// MaxTextLen, düz metnin (Doc.Text) en çok karakter sayısı; karakter = Unicode kod noktası.
	MaxTextLen = 5000
	// MaxBytes, ham JSON'un en çok boyutu. 5000 karakterlik yoğun biçimli bir belge 30 KiB'ı geçmez.
	MaxBytes = 128 << 10
	// MaxNodes, metin düğümleri dahil en çok düğüm sayısı.
	MaxNodes = 2000
	// MaxDepth, doc'tan metne en çok kat. 12 = bir alıntının içinde dört kat iç içe liste.
	MaxDepth = 12
)

var (
	// ErrInvalid: belge biçime uymuyor. Handler 400 validation_error der. Mesajda istemcinin
	// gönderdiği değerler yer almaz; yalnız sorunun yeri ve türü.
	ErrInvalid = errors.New("geçersiz belge")
	// ErrLimit: biçim doğru ama bir sınır aşıldı (boyut, düğüm sayısı, derinlik, metin uzunluğu).
	ErrLimit = fmt.Errorf("%w: sınır aşıldı", ErrInvalid)
	// ErrEmpty: belgede görünen metin yok (yalnız boşluk ya da boş paragraflar).
	ErrEmpty = fmt.Errorf("%w: metin boş", ErrInvalid)
)

// Düğüm türleri ve işaretler. İzin listesi bunlardan ibarettir.
const (
	typeDoc         = "doc"
	typeParagraph   = "paragraph"
	typeBlockquote  = "blockquote"
	typeBulletList  = "bulletList"
	typeOrderedList = "orderedList"
	typeListItem    = "listItem"
	typeText        = "text"

	MarkBold    = "bold"
	MarkItalic  = "italic"
	MarkSpoiler = "spoiler"
)

// markOrder, işaretlerin kanonik sırası.
var markOrder = []string{MarkBold, MarkItalic, MarkSpoiler}

// node, belge ağacının bir düğümü. JSON etiketleri kanonik çıktıyı tanımlar.
type node struct {
	Type    string `json:"type"`
	Content []node `json:"content,omitempty"`
	Marks   []mark `json:"marks,omitempty"`
	Text    string `json:"text,omitempty"`
}

type mark struct {
	Type string `json:"type"`
}

// Doc, doğrulanmış bir belge. Sıfır değeri boş belgedir; geçerli bir Doc yalnız Parse ya da
// FromPlain ile üretilir.
type Doc struct {
	root node
}

// Parse, ham JSON'u doğrular ve kanonik belgeyi döndürür. Hata her zaman ErrInvalid'i sarar
// (errors.Is); sınır aşımı ayrıca ErrLimit, görünen metnin olmaması ErrEmpty'dir.
func Parse(raw []byte) (Doc, error) {
	if len(raw) > MaxBytes {
		return Doc{}, fmt.Errorf("%w: belge %d bayttan büyük", ErrLimit, MaxBytes)
	}
	p := parser{}
	root, err := p.node(raw, "doc", 1)
	if err != nil {
		return Doc{}, err
	}
	if root.Type != typeDoc {
		return Doc{}, invalid("doc", "kök düğümün türü doc olmalı")
	}
	d := Doc{root: root}
	text := d.Text()
	if strings.TrimSpace(text) == "" {
		return Doc{}, ErrEmpty
	}
	if utf8.RuneCountInString(text) > MaxTextLen {
		return Doc{}, fmt.Errorf("%w: metin %d karakterden uzun", ErrLimit, MaxTextLen)
	}
	return d, nil
}

// parser, bir Parse çağrısının sayacını taşır.
type parser struct {
	nodes int
}

// allowedKeys, türüne göre bir düğümde bulunabilecek alanlar. "attrs" hiçbirinde yok.
var (
	textKeys  = []string{"type", "text", "marks"}
	blockKeys = []string{"type", "content"}
)

// node, tek bir düğümü ve altını çözer. path hata mesajı içindir ("doc.content[2]").
func (p *parser) node(raw json.RawMessage, path string, depth int) (node, error) {
	if depth > MaxDepth {
		return node{}, fmt.Errorf("%w: %s: iç içe en çok %d kat olabilir", ErrLimit, path, MaxDepth)
	}
	if p.nodes++; p.nodes > MaxNodes {
		return node{}, fmt.Errorf("%w: en çok %d düğüm olabilir", ErrLimit, MaxNodes)
	}
	fields, err := object(raw, path)
	if err != nil {
		return node{}, err
	}
	var typ string
	if err := str(fields["type"], &typ); err != nil {
		return node{}, invalid(path, "type bir metin olmalı")
	}
	if typ == typeText {
		return p.text(fields, path)
	}
	if !slices.Contains([]string{typeDoc, typeParagraph, typeBlockquote, typeBulletList, typeOrderedList, typeListItem}, typ) {
		return node{}, invalid(path, "bilinmeyen düğüm türü")
	}
	if err := onlyKeys(fields, blockKeys, path); err != nil {
		return node{}, err
	}
	var items []json.RawMessage
	if c, ok := fields["content"]; ok {
		if err := json.Unmarshal(c, &items); err != nil {
			return node{}, invalid(path, "content bir dizi olmalı")
		}
	}
	n := node{Type: typ}
	for i, item := range items {
		child, err := p.node(item, path+".content["+strconv.Itoa(i)+"]", depth+1)
		if err != nil {
			return node{}, err
		}
		n.Content = appendChild(n.Content, child)
	}
	if err := checkContent(n, path); err != nil {
		return node{}, err
	}
	return n, nil
}

// text, bir metin düğümünü çözer: boş olmayan metin, denetim karakteri yok, yalnız bilinen işaretler.
func (p *parser) text(fields map[string]json.RawMessage, path string) (node, error) {
	if err := onlyKeys(fields, textKeys, path); err != nil {
		return node{}, err
	}
	var s string
	if err := str(fields["text"], &s); err != nil {
		return node{}, invalid(path, "text bir metin olmalı")
	}
	if s == "" {
		return node{}, invalid(path, "metin düğümü boş olamaz")
	}
	if strings.ContainsFunc(s, forbiddenRune) {
		// Satır sonu da buraya düşer: paragraf içinde satır sonu yoktur, yeni satır yeni paragraftır.
		// NUL ayrıca Postgres'in metinde ve jsonb'de kabul etmediği tek karakter.
		return node{}, invalid(path, "metin denetim karakteri içeremez")
	}
	n := node{Type: typeText, Text: s}
	var raws []json.RawMessage
	if m, ok := fields["marks"]; ok {
		if err := json.Unmarshal(m, &raws); err != nil {
			return node{}, invalid(path, "marks bir dizi olmalı")
		}
	}
	seen := map[string]bool{}
	for i, raw := range raws {
		mpath := path + ".marks[" + strconv.Itoa(i) + "]"
		mf, err := object(raw, mpath)
		if err != nil {
			return node{}, err
		}
		if err := onlyKeys(mf, []string{"type"}, mpath); err != nil {
			return node{}, err
		}
		var mt string
		if err := str(mf["type"], &mt); err != nil || !slices.Contains(markOrder, mt) {
			return node{}, invalid(mpath, "bilinmeyen işaret")
		}
		if seen[mt] {
			return node{}, invalid(mpath, "aynı işaret iki kez verilmiş")
		}
		seen[mt] = true
	}
	for _, mt := range markOrder { // kanonik sıra
		if seen[mt] {
			n.Marks = append(n.Marks, mark{Type: mt})
		}
	}
	return n, nil
}

// checkContent, bir düğümün çocuklarının biçimdeki kurala uyduğunu denetler.
func checkContent(n node, path string) error {
	isBlock := func(c node) bool {
		return c.Type == typeParagraph || c.Type == typeBlockquote || c.Type == typeBulletList || c.Type == typeOrderedList
	}
	all := func(ok func(node) bool) bool {
		return !slices.ContainsFunc(n.Content, func(c node) bool { return !ok(c) })
	}
	switch n.Type {
	case typeParagraph:
		if !all(func(c node) bool { return c.Type == typeText }) {
			return invalid(path, "paragraf yalnız metin içerebilir")
		}
	case typeDoc, typeBlockquote:
		if len(n.Content) == 0 || !all(isBlock) {
			return invalid(path, "en az bir blok içermeli ve yalnız blok içerebilir")
		}
	case typeBulletList, typeOrderedList:
		if len(n.Content) == 0 || !all(func(c node) bool { return c.Type == typeListItem }) {
			return invalid(path, "liste en az bir madde içermeli ve yalnız madde içerebilir")
		}
	case typeListItem:
		if len(n.Content) == 0 || n.Content[0].Type != typeParagraph || !all(isBlock) {
			return invalid(path, "liste maddesi paragrafla başlamalı ve yalnız blok içerebilir")
		}
	}
	return nil
}

// appendChild, çocuğu ekler; aynı işaretli komşu metinleri tek düğümde birleştirir (kanonik hâl).
func appendChild(siblings []node, child node) []node {
	if last := len(siblings) - 1; last >= 0 && child.Type == typeText && siblings[last].Type == typeText &&
		slices.Equal(siblings[last].Marks, child.Marks) {
		siblings[last].Text += child.Text
		return siblings
	}
	return append(siblings, child)
}

// forbiddenRune: sekme dışındaki denetim karakterleri (NUL, satır sonu, DEL, C1 dahil).
func forbiddenRune(r rune) bool {
	return r != '\t' && unicode.IsControl(r)
}

// object, ham değeri JSON nesnesi olarak açar. Alan adları harfi harfine karşılaştırılır
// (encoding/json'ın struct eşleştirmesi büyük/küçük harfe bakmaz; burada "Type" bilinmeyen alandır).
func object(raw json.RawMessage, path string) (map[string]json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return nil, invalid(path, "bir JSON nesnesi olmalı")
	}
	return fields, nil
}

// str, ham değeri JSON metni olarak okur; alan yoksa ya da metin değilse hata.
func str(raw json.RawMessage, into *string) error {
	if len(raw) == 0 || raw[0] != '"' {
		return errors.New("metin değil")
	}
	return json.Unmarshal(raw, into)
}

func onlyKeys(fields map[string]json.RawMessage, allowed []string, path string) error {
	for k := range fields {
		if !slices.Contains(allowed, k) {
			return invalid(path, "bilinmeyen alan")
		}
	}
	return nil
}

func invalid(path, reason string) error {
	return fmt.Errorf("%w: %s: %s", ErrInvalid, path, reason)
}
