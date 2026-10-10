// Tasarım token'larının WCAG kontrastını ölçer (Faz 14): `node scripts/check-contrast.mjs`.
// styles.css'teki açık (:root) ve koyu (prefers-color-scheme: dark) bloklarını okur, OKLCH'yi
// sRGB'ye çevirir ve metin çiftlerinde 4.5:1, arayüz öğelerinde 3:1 arar. Bir çift kalırsa
// çıkış kodu 1.
import { readFileSync } from "node:fs";

const css = readFileSync(new URL("../src/styles.css", import.meta.url), "utf8");

function block(source, start) {
  const open = source.indexOf("{", source.indexOf(start));
  let depth = 0;
  for (let i = open; i < source.length; i++) {
    if (source[i] === "{") depth++;
    if (source[i] === "}" && --depth === 0) return source.slice(open + 1, i);
  }
  throw new Error(`blok bulunamadı: ${start}`);
}

function tokens(body) {
  const out = {};
  for (const m of body.matchAll(/--([\w-]+):([^;]+);/g)) out[m[1]] = m[2].trim();
  return out;
}

const light = tokens(block(css, ":root"));
const darkMedia = block(css, "@media (prefers-color-scheme: dark)");
const dark = { ...light, ...tokens(block(darkMedia, ":root")) };

function resolve(set, name) {
  let v = set[name];
  for (let i = 0; i < 5 && v?.startsWith("var("); i++) v = set[v.slice(6, -1)];
  if (!v) throw new Error(`token yok: ${name}`);
  return v;
}

// OKLCH → doğrusal sRGB (Björn Ottosson'un OKLab matrisleri) → göreli parlaklık.
function luminance(oklch) {
  const [L, C, h] = oklch.match(/oklch\(([^)]+)\)/)[1].split(/\s+/).map(Number);
  const a = C * Math.cos((h * Math.PI) / 180);
  const b = C * Math.sin((h * Math.PI) / 180);
  const l = (L + 0.3963377774 * a + 0.2158037573 * b) ** 3;
  const m = (L - 0.1055613458 * a - 0.0638541728 * b) ** 3;
  const s = (L - 0.0894841775 * a - 1.291485548 * b) ** 3;
  const clamp = (x) => Math.min(1, Math.max(0, x));
  const r = clamp(4.0767416621 * l - 3.3077115913 * m + 0.2309699292 * s);
  const g = clamp(-1.2684380046 * l + 2.6097574011 * m - 0.3413193965 * s);
  const bl = clamp(-0.0041960863 * l - 0.7034186147 * m + 1.707614701 * s);
  return 0.2126 * r + 0.7152 * g + 0.0722 * bl;
}

const ratio = (x, y) => {
  const [hi, lo] = [luminance(x), luminance(y)].sort((p, q) => q - p);
  return (hi + 0.05) / (lo + 0.05);
};

// [ön plan, arka plan, en az oran]
const PAIRS = [
  ["foreground", "background", 4.5],
  ["foreground", "card", 4.5],
  ["muted-foreground", "background", 4.5],
  ["muted-foreground", "card", 4.5],
  ["muted-foreground", "muted", 4.5],
  ["primary", "background", 4.5],
  ["primary", "card", 4.5],
  ["primary-foreground", "primary", 4.5],
  ["secondary-foreground", "secondary", 4.5],
  ["accent-foreground", "accent", 4.5],
  ["destructive", "background", 4.5],
  ["destructive", "card", 4.5],
  ["destructive-foreground", "destructive", 4.5],
  ["success", "background", 4.5],
  ["success", "card", 4.5],
  ["ring", "background", 3],
  ["star", "card", 3],
  ["input", "card", 1.5],
];

let failed = 0;
for (const [theme, set] of [["açık", light], ["koyu", dark]]) {
  for (const [fg, bg, min] of PAIRS) {
    const r = ratio(resolve(set, fg), resolve(set, bg));
    const ok = r >= min;
    if (!ok) failed++;
    const pair = `${fg} / ${bg}`;
    console.log(`${ok ? "✓" : "✗"} ${theme.padEnd(4)} ${pair.padEnd(42)} ${r.toFixed(2)} (≥ ${min})`);
  }
}
// Üretilmiş kapak paleti (BookCover.tsx): her zemin/yazı çifti ≥ 4.5:1.
const cover = readFileSync(new URL("../src/components/BookCover.tsx", import.meta.url), "utf8");
for (const m of cover.matchAll(/bg: "(oklch\([^)]+\))", fg: "(oklch\([^)]+\))" \}, \/\/ (\S+)/g)) {
  const r = ratio(m[2], m[1]);
  const ok = r >= 4.5;
  if (!ok) failed++;
  console.log(`${ok ? "✓" : "✗"} kapak ${m[3].padEnd(42)} ${r.toFixed(2)} (≥ 4.5)`);
}

process.exit(failed ? 1 : 0);
