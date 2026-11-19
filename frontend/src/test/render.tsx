import { act } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
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

/**
 * withQuery: bileşeni TAZE bir önbellekle sarar (Faz 16d). Her test kendi önbelleğini alır; yoksa bir
 * testin çektiği veri ötekinde "önbellekten" görünür ve testler sıraya bağımlı olurdu. Testte yeniden
 * deneme kapalı: hata senaryosu beklemeden hata ekranını göstersin.
 */
export function withQuery(node: React.ReactNode): React.ReactNode {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  return <QueryClientProvider client={client}>{node}</QueryClientProvider>;
}
