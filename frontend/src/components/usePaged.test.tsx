import { keepPreviousData, QueryClient, QueryClientProvider, useQuery } from "@tanstack/react-query";
import { act, useState } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, describe, expect, it } from "vitest";
import type { Page } from "../api/types";
import "../test/render"; // notifyManager.setScheduler(queueMicrotask): bildirimler act içinde gelsin
import { Pager, usePageView } from "./usePaged";

// React'e "test ortamındayız" de: act() dışı güncelleme uyarısı basmasın.
(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

const page = (items: number[], total: number): Page<number> => ({ items, total, limit: 2, offset: 0 });
const cleanup: (() => void)[] = [];
afterEach(() => cleanup.splice(0).forEach((f) => f()));
const flush = () => act(async () => {});

type View = ReturnType<typeof usePageView<number>>;

// usePageView'ı sahte bir sorgu sonucuyla çalıştırır: kancanın KENDİ kuralları (geri adım, gizleme,
// pager) ağdan ve TanStack'ten bağımsız sınanır.
function mountView(data: Page<number> | undefined, isPlaceholderData: boolean, offset: number) {
  const box: { current?: View; setTo: number[] } = { setTo: [] };
  function Harness() {
    box.current = usePageView({ data, isPlaceholderData }, offset, (o) => box.setTo.push(o));
    return null;
  }
  const el = document.createElement("div");
  const root = createRoot(el);
  act(() => root.render(<Harness />));
  cleanup.push(() => act(() => root.unmount()));
  return box;
}

describe("usePageView", () => {
  // Review bulgusu: son sayfadaki son öğe kaybolunca boş sayfada mahsur kalınmamalı.
  it("boş sayfada bir sayfa geri gider ve boş sayfayı çizmez", () => {
    const v = mountView(page([], 2), false, 2);
    expect(v.setTo).toEqual([0]);
    expect(v.current!.page).toBeUndefined(); // eski satırlar ya da "liste boş" yanıp sönmesin
  });

  it("boş YER TUTUCUDA geri adım atılmaz (gerçek cevap beklenir) ve o da çizilmez", () => {
    const v = mountView(page([], 2), true, 2);
    expect(v.setTo).toEqual([]);
    expect(v.current!.page).toBeUndefined();
  });

  it("geri adımdan sonra ilk sayfada boş yer tutucu \"liste boş\" diye gösterilmez", () => {
    const v = mountView(page([], 2), true, 0);
    expect(v.current!.page).toBeUndefined();
  });

  it("ilk sayfada gerçekten boş liste gösterilir (boş ekran için)", () => {
    const v = mountView(page([], 0), false, 0);
    expect(v.setTo).toEqual([]);
    expect(v.current!.page!.total).toBe(0);
  });

  it("offset > 0 iken toplam limitten küçük olsa da pager görünür", () => {
    expect(mountView(page([1, 2], 2), false, 0).current!.showPager).toBe(false); // tek sayfa, ilk sayfadayız
    expect(mountView(page([9], 2), false, 2).current!.showPager).toBe(true); // geri dönüş yolu açık kalmalı
  });
});

// Eski usePaged'in sıra numarasıyla yaptığı iş artık sorgu anahtarından geliyor. Bu test kuralın hâlâ
// geçerli olduğunu GERÇEK bir sorguyla gösterir (sayfalar useComments/useAdminBooks ile aynı biçimde).
describe("usePageView + useQuery", () => {
  it("geç gelen ESKİ sayfa cevabı yenisinin üzerine yazmaz", async () => {
    const resolvers: Record<number, (p: Page<number>) => void> = {};
    const box: { view?: View; setOffset?: (o: number) => void } = {};
    function Harness() {
      const [offset, setOffset] = useState(0);
      const q = useQuery({
        queryKey: ["test", offset],
        queryFn: () => new Promise<Page<number>>((r) => (resolvers[offset] = r)),
        placeholderData: keepPreviousData,
      });
      box.view = usePageView(q, offset, setOffset);
      box.setOffset = setOffset;
      return null;
    }
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const el = document.createElement("div");
    const root = createRoot(el);
    act(() => root.render(<QueryClientProvider client={client}><Harness /></QueryClientProvider>));
    cleanup.push(() => act(() => root.unmount()));
    await flush();
    await act(async () => box.setOffset!(2)); // offset 0 isteği hâlâ uçuşta
    await act(async () => resolvers[2](page([3], 3)));
    await act(async () => resolvers[0](page([1, 2], 3))); // bayat cevap geç geldi
    expect(box.view!.page!.items).toEqual([3]);
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

  // REVIEW_FEEDBACK F25: sunucu offset 10.000'den ötesini 400 ile reddeder. Toplam daha büyükse "Sonraki"
  // sınırda kapanır ve nedeni yazılır; düğme açık kalıp ilk sayfaya dönmez.
  it("10.000 sınırında sonraki kapanır ve nedeni yazılır", () => {
    const atLimit = render({ items: [1, 2], total: 50_000, limit: 20, offset: 9_990 }, 9_990, () => {});
    expect(atLimit.next.disabled).toBe(true);
    expect(atLimit.el.textContent).toContain("En fazla ilk 10.000 sonuç");

    const before = render({ items: [1, 2], total: 50_000, limit: 20, offset: 9_960 }, 9_960, () => {});
    expect(before.next.disabled).toBe(false); // bir sonraki offset (9.980) hâlâ geçerli
    expect(before.el.textContent).not.toContain("En fazla");

    const small = render({ items: [5], total: 5, limit: 2, offset: 4 }, 4, () => {});
    expect(small.el.textContent).not.toContain("En fazla"); // olağan son sayfada not yok
  });

  it("tıklamalar onChange(offset ± limit) çağırır", () => {
    const calls: number[] = [];
    const { prev, next } = render({ items: [3, 4], total: 9, limit: 2, offset: 2 }, 2, (o) => calls.push(o));
    act(() => prev.click());
    act(() => next.click());
    expect(calls).toEqual([0, 4]);
  });
});
