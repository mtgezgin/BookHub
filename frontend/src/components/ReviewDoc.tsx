import { type ReactNode, useId, useState } from "react";
import { type DocNode, isAllSpoiler, isBold, isItalic, isSpoiler } from "../lib/reviewDoc";
import { Button } from "./ui/button";

/**
 * - "reader": başkasının incelemesi. Spoiler parçalarının metni açılana kadar DOM'da HİÇ yok.
 * - "author": okurun kendi incelemesi. Her şey görünür; spoiler parçaları işaretli (neyi gizlediğini görsün).
 */
type Mode = "reader" | "author";

/**
 * İnceleme belgesini çizer (Faz 17C). Belge HTML değil, bir düğüm ağacıdır; burada React öğelerine
 * çevrilir. `dangerouslySetInnerHTML` yok: metin her zaman metin olarak basılır, "<script>" yazan bir
 * inceleme ekranda "<script>" diye görünür.
 *
 * Tanınmayan düğüm türü çizilmez (çökmez). Güvenlik buna dayanmaz: asıl kapı sunucudaki doğrulayıcı.
 *
 * SPOILER (spec karar 9): bir paragraftaki art arda spoiler işaretli metinler tek parçadır ve kendi
 * düğmesiyle açılır; öteki parçalar kapalı kalır. Metnin TAMAMI spoiler ise (eski "spoiler içeriyor"
 * kutusu) inceleme tek düğmeyle açılır.
 */
export function ReviewDoc({ doc, mode = "reader" }: Readonly<{ doc: DocNode; mode?: Mode }>) {
  if (mode === "reader" && isAllSpoiler(doc)) return <WholeSpoiler doc={doc} />;
  return <div className="mt-1 flex flex-col gap-2 leading-relaxed">{blocks(doc.content, mode === "author" ? "marked" : "pieces")}</div>;
}

/*
 * React anahtarı: sıra numarası değil, düğüm nesnesinin kimliği. Belge sunucudan gelen değişmez bir
 * ağaçtır; aynı düğüm her çizimde aynı anahtarı alır, araya düğüm girse de anahtarlar kaymaz (Sonar S6479).
 */
const nodeKeys = new WeakMap<DocNode, number>();
let nextKey = 0;
function keyOf(node: DocNode): number {
  let key = nodeKeys.get(node);
  if (key === undefined) {
    key = nextKey++;
    nodeKeys.set(node, key);
  }
  return key;
}

/** Spoiler metninin nasıl basılacağı: parça parça gizli, işaretli ama görünür, ya da düz. */
type SpoilerStyle = "pieces" | "marked" | "plain";

/** Tamamı spoiler olan inceleme: 17B'deki tek düğme. Metin açılana kadar DOM'da yok. */
function WholeSpoiler({ doc }: Readonly<{ doc: DocNode }>) {
  const [revealed, setRevealed] = useState(false);
  const bodyId = useId();
  return (
    <>
      <p className="mt-2 flex flex-wrap items-center gap-2 text-sm text-muted-foreground">
        Spoiler içeriyor
        <Button variant="outline" size="sm" aria-expanded={revealed} aria-controls={revealed ? bodyId : undefined} onClick={() => setRevealed(!revealed)}>
          {revealed ? "Spoiler'ı gizle" : "Spoiler'ı göster"}
        </Button>
      </p>
      {revealed && <div id={bodyId} className="mt-1 flex flex-col gap-2 leading-relaxed">{blocks(doc.content, "plain")}</div>}
    </>
  );
}

function blocks(nodes: DocNode[] | undefined, style: SpoilerStyle): ReactNode {
  return (nodes ?? []).map((n) => <Block key={keyOf(n)} node={n} style={style} />);
}

function Block({ node, style }: Readonly<{ node: DocNode; style: SpoilerStyle }>) {
  switch (node.type) {
    case "paragraph":
      // Boş paragraf bir satır yer tutar (okurun bıraktığı boşluk). pre-wrap: art arda boşluk ve sekme korunur.
      return <p className="whitespace-pre-wrap">{node.content?.length ? inline(node.content, style) : <br />}</p>;
    case "blockquote":
      return <blockquote className="flex flex-col gap-2 border-l-2 pl-3 text-muted-foreground">{blocks(node.content, style)}</blockquote>;
    case "bulletList":
      return <ul className="flex list-disc flex-col gap-1 pl-6">{blocks(node.content, style)}</ul>;
    case "orderedList":
      return <ol className="flex list-decimal flex-col gap-1 pl-6">{blocks(node.content, style)}</ol>;
    case "listItem":
      return <li><div className="flex flex-col gap-1">{blocks(node.content, style)}</div></li>;
    default:
      return null;
  }
}

/** Paragrafın içi: art arda gelen spoiler metinleri tek parçada toplanır (aradaki kalın onları bölmesin). */
function inline(nodes: DocNode[], style: SpoilerStyle): ReactNode {
  const runs: { spoiler: boolean; nodes: DocNode[] }[] = [];
  for (const n of nodes) {
    if (n.type !== "text") continue;
    const spoiler = style !== "plain" && isSpoiler(n);
    const last = runs.at(-1);
    if (last?.spoiler === spoiler) last.nodes.push(n);
    else runs.push({ spoiler, nodes: [n] });
  }
  return runs.map((run) => {
    const key = keyOf(run.nodes[0]); // parçanın anahtarı ilk metin düğümünün kimliği
    const text = run.nodes.map((n) => <Text key={keyOf(n)} node={n} />);
    if (!run.spoiler) return <span key={key}>{text}</span>;
    if (style === "marked") return <mark key={key} className="rounded bg-muted px-0.5 text-inherit" title="Spoiler olarak işaretli">{text}</mark>;
    return <SpoilerPiece key={key}>{text}</SpoilerPiece>;
  });
}

function Text({ node }: Readonly<{ node: DocNode }>) {
  let out: ReactNode = node.text ?? "";
  if (isItalic(node)) out = <em>{out}</em>;
  if (isBold(node)) out = <strong className="font-semibold">{out}</strong>;
  return out;
}

/**
 * Bir spoiler parçası. Kapalıyken metin DOM'da yok (CSS ile gizlemek ekran okuyucuya ve sayfa içi
 * aramaya sızdırırdı; 17B'nin kuralı, parça başına). Düğme yerinde kalır: açınca odak kaybolmaz, aynı
 * düğme kapatır.
 */
function SpoilerPiece({ children }: Readonly<{ children: ReactNode }>) {
  const [revealed, setRevealed] = useState(false);
  const id = useId();
  return (
    <>
      <Button
        variant="outline" size="sm" className="mx-0.5 h-auto px-1.5 py-0 align-baseline text-xs"
        aria-expanded={revealed} aria-controls={revealed ? id : undefined} onClick={() => setRevealed(!revealed)}
      >
        {revealed ? "Spoiler'ı gizle" : "Spoiler'ı göster"}
      </Button>
      {revealed && <span id={id} className="rounded bg-muted px-0.5">{children}</span>}
    </>
  );
}
