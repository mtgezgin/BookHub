/*
 * İnceleme belgesi (Faz 17C): sunucunun doğruladığı JSON ağacının arayüzdeki karşılığı. Buradaki
 * fonksiyonlar saf; çizim components/ReviewDoc.tsx'te.
 *
 * KURALLARIN SAHİBİ SUNUCU (backend/internal/richtext). Düz metin ve uzunluk hesabı onunkiyle birebir
 * aynı olmalı: ayrışırlarsa okur sayaçta yer varken 400 alır. İki taraf aynı örnek dosyasıyla sınanır
 * (backend/internal/richtext/testdata/vectors.json).
 */

/** Sunucudaki sınır (richtext.MaxTextLen): düz metnin karakter (kod noktası) sayısı. */
export const MAX_REVIEW = 5000;

/**
 * Belge düğümü. Tür adları sunucunun izin listesi: doc, paragraph, blockquote, bulletList,
 * orderedList, listItem, text; işaretler bold, italic, spoiler. Tip bilerek gevşek (string): veri
 * ağdan gelir; çizen kod tanımadığı türü atlar, tipe güvenmez.
 */
export interface DocNode {
  type: string;
  content?: DocNode[];
  text?: string;
  marks?: { type: string }[];
}

const hasMark = (n: DocNode, mark: string) => n.marks?.some((m) => m.type === mark) ?? false;

/** Metin düğümü spoiler işaretli mi. */
export const isSpoiler = (n: DocNode) => hasMark(n, "spoiler");
export const isBold = (n: DocNode) => hasMark(n, "bold");
export const isItalic = (n: DocNode) => hasMark(n, "italic");

/** Ağaçtaki bütün metin düğümleri, belge sırasıyla. */
function textNodes(n: DocNode, out: DocNode[] = []): DocNode[] {
  if (n.type === "text") out.push(n);
  for (const c of n.content ?? []) textNodes(c, out);
  return out;
}

/**
 * Düz metin: her paragraf bir satır, satırlar "\n" ile ayrılır; madde imi ve numara yazılmaz.
 * Sunucudaki Doc.Text ile aynı.
 */
export function docText(doc: DocNode): string {
  const lines: string[] = [];
  const walk = (n: DocNode) => {
    if (n.type === "paragraph") lines.push((n.content ?? []).map((t) => t.text ?? "").join(""));
    else for (const c of n.content ?? []) walk(c);
  };
  walk(doc);
  return lines.join("\n");
}

/**
 * Sayaçtaki uzunluk: kod noktası sayısı (sunucu ve veritabanı böyle sayar). `string.length` UTF-16
 * birimi sayar; bir emoji 2 çıkar ve sayaç sunucudan erken dolardı.
 */
export function docLength(doc: DocNode): number {
  let n = 0;
  for (const _ of docText(doc)) n++;
  return n;
}

/** Belgede spoiler işaretli metin var mı. */
export const docHasSpoiler = (doc: DocNode) => textNodes(doc).some(isSpoiler);

/**
 * Metnin TAMAMI spoiler mi (en az bir metin var ve hepsi işaretli). Eski "inceleme spoiler içeriyor"
 * kutusunun karşılığı: böyle bir inceleme tek düğmeyle açılır, paragraf başına düğme çıkmaz.
 */
export function isAllSpoiler(doc: DocNode): boolean {
  const texts = textNodes(doc);
  return texts.length > 0 && texts.every(isSpoiler);
}

/**
 * Düz metinden belge: her satır bir paragraf; spoiler ise bütün metin işaretli. Sunucudaki FromPlain
 * ile aynı: satır sonları "\n"e iner, sekme dışındaki denetim karakterleri atılır.
 */
export function plainToDoc(body: string, spoiler: boolean): DocNode {
  const marks = spoiler ? [{ type: "spoiler" }] : undefined;
  const content = body.replace(/\r\n?/g, "\n").split("\n").map((raw): DocNode => {
    const line = raw.replace(/[^\P{Cc}\t]/gu, "");
    return line === "" ? { type: "paragraph" } : { type: "paragraph", content: [{ type: "text", text: line, ...(marks && { marks }) }] };
  });
  return { type: "doc", content };
}

/**
 * Bir incelemenin belgesi. Sunucu `doc`'u her zaman gönderir; alan yoksa (yayın sırasında birkaç
 * dakika eski API ile konuşan yeni arayüz) düz metinden çevrilir.
 */
export function docOf(review: { doc?: DocNode | null; body: string; has_spoiler: boolean }): DocNode {
  return review.doc ?? plainToDoc(review.body, review.has_spoiler);
}
