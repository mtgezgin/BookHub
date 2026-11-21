import { act } from "react";
import { notifyManager, QueryClient, QueryClientProvider } from "@tanstack/react-query";
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

// TanStack, önbellekteki değişiklikleri bileşenlere toplu ve setTimeout(0) ile bildirir
// (query-core notifyManager.js: defaultScheduler = systemSetTimeoutZero). Testlerin `act(async () => {})`
// beklemesi mikro görevleri boşaltır ama zamanlayıcıları BEKLEMEZ: cevap önbelleğe gelir, ekrana gelmez.
// Testte bildirimi mikro göreve alıyoruz; davranış aynı, yalnız zamanlama act'in gördüğü yere taşınıyor.
notifyManager.setScheduler(queueMicrotask);

/**
 * withQuery: bileşeni TAZE bir önbellekle sarar (Faz 16d). Her test kendi önbelleğini alır; yoksa bir
 * testin çektiği veri ötekinde "önbellekten" görünür ve testler sıraya bağımlı olurdu. Testte yeniden
 * deneme kapalı: hata senaryosu beklemeden hata ekranını göstersin.
 */
export function withQuery(node: React.ReactNode, client = testQueryClient()): React.ReactNode {
  return <QueryClientProvider client={client}>{node}</QueryClientProvider>;
}

/**
 * mountHook: bir kancayı, ekrana bir şey çizmeyen bir bileşenin içinde verilen önbellekle çalıştırır; son
 * dönüş değeri `box.current`'ta. Değişiklik kancalarının "neyi geçersiz kılar" kurallarını sınayan testler
 * (api/*Queries.test.tsx) için. Söküm, render'ınki gibi testin sonunda.
 */
export function mountHook<T>(useHook: () => T, client: QueryClient): { current?: T } {
  const box: { current?: T } = {};
  function Harness() {
    box.current = useHook();
    return null;
  }
  render(withQuery(<Harness />, client));
  return box;
}

/** Testin kendi önbelleği. Aynı ağacı yeniden çizen testler (rerender) bunu bir kez kurup withQuery'ye verir. */
export function testQueryClient(): QueryClient {
  return new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
}
