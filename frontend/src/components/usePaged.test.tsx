import { act } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, describe, expect, it } from "vitest";
import type { Page } from "../api/types";
import { Pager, usePaged } from "./usePaged";

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

describe("usePaged — hata ve sıfırlama", () => {
  function mountWith(fetch: (offset: number) => Promise<Page<number>>, onError: (e: unknown) => void, resetKey = "k") {
    const box: { current?: Hook; key: string } = { key: resetKey };
    function Harness({ k }: { k: string }) {
      box.current = usePaged(fetch, k, onError);
      return null;
    }
    const el = document.createElement("div");
    const root = createRoot(el);
    act(() => root.render(<Harness k={resetKey} />));
    cleanup.push(() => act(() => root.unmount()));
    return { box, rerender: (k: string) => act(() => root.render(<Harness k={k} />)) };
  }

  it("reddedilen istek onError'u çağırır", async () => {
    const errors: unknown[] = [];
    const boom = new Error("ağ hatası");
    mountWith(() => Promise.reject(boom), (e) => errors.push(e));
    await flush();
    expect(errors).toEqual([boom]);
  });

  it("yerini yenisine bırakmış (bayat) isteğin reddi onError'u ÇAĞIRMAZ", async () => {
    const errors: unknown[] = [];
    const rejecters: Record<number, (e: unknown) => void> = {};
    const resolvers: Record<number, (p: Page<number>) => void> = {};
    const fetch = (offset: number) =>
      new Promise<Page<number>>((res, rej) => {
        resolvers[offset] = res;
        rejecters[offset] = rej;
      });
    const { box } = mountWith(fetch, (e) => errors.push(e));
    await act(async () => box.current!.setOffset(2)); // offset 0 isteği bayatladı
    await act(async () => resolvers[2](page([3], 3)));
    await act(async () => rejecters[0](new Error("bayat")));
    expect(errors).toEqual([]);
  });

  it("resetKey değişince offset 0'a döner", async () => {
    const { box, rerender } = mountWith(async () => page([1, 2], 10), () => {}, "PENDING");
    await flush();
    await act(async () => box.current!.setOffset(4));
    expect(box.current!.offset).toBe(4);
    rerender("APPROVED");
    await flush();
    expect(box.current!.offset).toBe(0);
  });
});

describe("Pager", () => {
  function render(p: Page<number>, offset: number, onChange: (o: number) => void) {
    const el = document.createElement("div");
    const root = createRoot(el);
    act(() => root.render(<Pager page={p} offset={offset} onChange={onChange} />));
    cleanup.push(() => act(() => root.unmount()));
    const [prev, next] = Array.from(el.querySelectorAll("button"));
    return { el, prev, next };
  }

  it("offset 0'da önceki devre dışı, etiket a–b / toplam", () => {
    const { el, prev, next } = render({ items: [1, 2], total: 5, limit: 2, offset: 0 }, 0, () => {});
    expect(prev.disabled).toBe(true);
    expect(next.disabled).toBe(false);
    expect(el.querySelector("span")!.textContent).toBe("1–2 / 5");
  });

  it("son sayfada sonraki devre dışı, etiket toplamda biter", () => {
    const { el, prev, next } = render({ items: [5], total: 5, limit: 2, offset: 4 }, 4, () => {});
    expect(next.disabled).toBe(true);
    expect(prev.disabled).toBe(false);
    expect(el.querySelector("span")!.textContent).toBe("5–5 / 5");
  });

  it("tıklamalar onChange(offset ± limit) çağırır", () => {
    const calls: number[] = [];
    const { prev, next } = render({ items: [3, 4], total: 9, limit: 2, offset: 2 }, 2, (o) => calls.push(o));
    act(() => prev.click());
    act(() => next.click());
    expect(calls).toEqual([0, 4]);
  });

  it("boş sayfadan geri adım atarken eski satırlar ekranda kalmaz", async () => {
    let total = 3;
    let back!: () => void;
    const fetch = (offset: number) =>
      offset === 0 && total === 2
        ? new Promise<Page<number>>((res) => { back = () => res(page([1, 2], total)); })
        : Promise.resolve(offset === 0 ? page([1, 2], total) : page(total > 2 ? [3] : [], total));
    const h = mount(fetch);
    await flush();
    await act(async () => h.current!.setOffset(2));
    expect(h.current!.page!.items).toEqual([3]);
    total = 2;
    await act(async () => h.current!.reload());
    await flush();
    expect(h.current!.page).toBeNull();
    await act(async () => back());
    expect(h.current!.page!.items).toEqual([1, 2]);
  });
});
