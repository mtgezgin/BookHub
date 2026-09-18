import { act } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, describe, expect, it } from "vitest";
import type { Page } from "../api/types";
import { usePaged } from "./usePaged";

// React'e "test ortamındayız" de: act() dışı güncelleme uyarısı basmasın.
(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

type Hook = ReturnType<typeof usePaged<number>>;
const page = (items: number[], total: number): Page<number> => ({ items, total, limit: 2, offset: 0 });

// Hook'u gerçek bir bileşende çalıştırıp son değerini dışarı verir (ek kütüphane gerekmez).
function mount(fetch: (offset: number) => Promise<Page<number>>) {
  const box: { current?: Hook } = {};
  function Harness() {
    box.current = usePaged(fetch, "k", () => {});
    return null;
  }
  const el = document.createElement("div");
  const root = createRoot(el);
  act(() => root.render(<Harness />));
  cleanup.push(() => act(() => root.unmount()));
  return box;
}
const cleanup: (() => void)[] = [];
afterEach(() => cleanup.splice(0).forEach((f) => f()));
const flush = () => act(async () => {});

describe("usePaged", () => {
  // Review bulgusu: son sayfadaki son öğe kaybolunca boş sayfada mahsur kalınmamalı.
  it("boş sayfada bir sayfa geri gider", async () => {
    let total = 3;
    const fetch = async (offset: number) => (offset === 0 ? page([1, 2], total) : page(total > 2 ? [3] : [], total));
    const h = mount(fetch);
    await flush();
    await act(async () => h.current!.setOffset(2));
    expect(h.current!.page!.items).toEqual([3]);

    total = 2; // 3. öğe silindi/onaylandı
    await act(async () => h.current!.reload());
    await flush();
    expect(h.current!.offset).toBe(0);
    expect(h.current!.page!.items).toEqual([1, 2]);
  });

  it("geç gelen ESKİ cevap yenisinin üzerine yazmaz", async () => {
    const resolvers: Record<number, (p: Page<number>) => void> = {};
    const fetch = (offset: number) => new Promise<Page<number>>((r) => (resolvers[offset] = r));
    const h = mount(fetch);
    await act(async () => h.current!.setOffset(2)); // offset 0 isteği hâlâ uçuşta
    await act(async () => resolvers[2](page([3], 3)));
    await act(async () => resolvers[0](page([1, 2], 3))); // bayat cevap geç geldi
    expect(h.current!.page!.items).toEqual([3]);
  });

  it("offset > 0 iken toplam limitten küçük olsa da pager görünür", async () => {
    const h = mount(async (offset) => (offset === 0 ? page([1, 2], 2) : page([9], 2)));
    await flush();
    expect(h.current!.showPager).toBe(false); // tek sayfa, ilk sayfadayız
    await act(async () => h.current!.setOffset(2));
    expect(h.current!.showPager).toBe(true); // geri dönüş yolu açık kalmalı
  });
});
