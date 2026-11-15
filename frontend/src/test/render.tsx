import { act } from "react";
import { createRoot } from "react-dom/client";
import { afterEach } from "vitest";

// Bileşen testlerinin ortak çizim yardımcısı: React'i act ortamına alır, bileşeni ayrık bir
// <div>'e çizer ve test bitince söker. Yeni testler bunu kullanır; eski test dosyalarındaki
// kopyalar ayrı bir temizlikte buraya taşınabilir.
(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

const cleanup: (() => void)[] = [];
afterEach(() => cleanup.splice(0).forEach((f) => f()));

export function render(node: React.ReactNode): HTMLDivElement {
  const el = document.createElement("div");
  const root = createRoot(el);
  act(() => root.render(node));
  cleanup.push(() => act(() => root.unmount()));
  return el;
}
