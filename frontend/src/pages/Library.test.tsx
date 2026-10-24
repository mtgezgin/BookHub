import { act } from "react";
import { createRoot } from "react-dom/client";
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { Book, LibraryItem, LibraryStatus, Page } from "../api/types";

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

vi.mock("../api/client", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../api/client")>()),
  api: {},
}));

import { api, ApiError } from "../api/client";
import { ToastProvider } from "../components/toast";
import { Library } from "./Library";

const calls: Record<string, unknown[][]> = {};
function stub(impls: Record<string, (...a: any[]) => unknown>) {
  for (const [k, f] of Object.entries(impls)) {
    (api as unknown as Record<string, unknown>)[k] = (...args: unknown[]) => {
      (calls[k] ??= []).push(args);
      return f(...args);
    };
  }
}
const bookOf = (id: number): Book => ({
  id, title: `Kitap ${id}`, slug: `kitap-${id}`, author: "Yazar", description: "", cover_url: "",
  publication_year: null, page_count: null, language: "tr", genres: [], average_rating: null, rating_count: 0,
  created_at: "", updated_at: "",
});
const itemOf = (id: number, extra: Partial<LibraryItem> = {}): LibraryItem => ({
  book: bookOf(id), status: "READ" as LibraryStatus, added_at: "", my_rating: null, in_catalog: true, ...extra,
});
const pageOf = (items: LibraryItem[], total = items.length, offset = 0): Page<LibraryItem> => ({ items, total, limit: 12, offset });

let reply: (p: { offset?: number }) => Page<LibraryItem>;
const cleanup: (() => void)[] = [];
afterEach(() => cleanup.splice(0).forEach((f) => f()));
beforeEach(() => {
  for (const k of Object.keys(calls)) delete calls[k];
  reply = () => pageOf([itemOf(1)]);
  stub({
    genres: async () => [{ id: 1, name: "Roman", slug: "roman" }],
    library: async (p: { offset?: number }) => reply(p),
    removeFromLibrary: async () => undefined,
  });
});

const flush = async () => {
  for (let i = 0; i < 4; i++) await act(async () => {});
};

async function render(path: string) {
  const loc = { search: "" };
  function Probe() {
    loc.search = useLocation().search;
    return null;
  }
  const el = document.createElement("div");
  document.body.appendChild(el);
  const root = createRoot(el);
  act(() =>
    root.render(
      <ToastProvider>
        <MemoryRouter initialEntries={[path]}>
          <Routes>
            <Route path="/library" element={<><Library /><Probe /></>} />
            <Route path="/" element={<p>katalog</p>} />
          </Routes>
        </MemoryRouter>
      </ToastProvider>,
    ),
  );
  cleanup.push(() => {
    act(() => root.unmount());
    el.remove();
  });
  await flush();
  return { el, params: () => new URLSearchParams(loc.search) };
}

const buttonIn = (root: ParentNode, text: string) =>
  Array.from(root.querySelectorAll<HTMLButtonElement>("button")).find((b) => b.textContent?.trim() === text);
const group = (el: HTMLElement, label: string) => el.querySelector<HTMLElement>(`[role=group][aria-label="${label}"]`)!;
const errorToast = () =>
  document.querySelector("section[aria-label=Bildirimler] .text-destructive")?.textContent;
async function removeFirst(el: HTMLElement) {
  await click(buttonIn(el.querySelector('[aria-label="Kitaplığım"] li')!, "Çıkar"));
  await click(buttonIn(document.querySelector<HTMLElement>("[role=dialog]")!, "Çıkar"));
}
async function click(target: HTMLElement | undefined) {
  expect(target, "tıklanacak öğe bulunamadı").toBeTruthy();
  await act(async () => target!.click());
  await flush();
}

describe("Library", () => {
  it("filtreler adresle eşleşir", async () => {
    const { el } = await render("/library?status=READ&sort=title&q=dune");
    expect(calls.library!.at(-1)).toEqual([{ status: "READ", sort: "title", q: "dune", limit: 12, offset: 0 }]);
    expect(buttonIn(group(el, "Durum"), "Okudum")!.getAttribute("aria-pressed")).toBe("true");
    expect(buttonIn(group(el, "Sıralama"), "Başlık")!.getAttribute("aria-pressed")).toBe("true");
    expect(el.querySelector<HTMLInputElement>("input[type=search]")!.value).toBe("dune");
  });

  it("elle değiştirilmiş adres: sunucunun reddedeceği tür, arama ve sayfa API'ye gitmez", async () => {
    const { el } = await render(`/library?genre=Bilim%20Kurgu&q=${"a".repeat(101)}&offset=99999999`);
    expect(calls.library!.at(-1)).toEqual([{ limit: 12, offset: 0 }]);
    expect(el.querySelector('[aria-label="Kitaplığım"] li')).not.toBeNull();
  });

  it("sınırdaki tür, arama ve sayfa olduğu gibi gider", async () => {
    await render(`/library?genre=bilim-kurgu&q=${"ç".repeat(100)}&offset=10000`);
    expect(calls.library!.at(-1)).toEqual([{ genre: "bilim-kurgu", q: "ç".repeat(100), limit: 12, offset: 10000 }]);
  });

  it("filtre değişince ilk sayfa", async () => {
    reply = () => pageOf([itemOf(13)], 13, 12);
    const { el, params } = await render("/library?offset=12");
    await click(buttonIn(group(el, "Türler"), "Roman"));
    expect(params().get("genre")).toBe("roman");
    expect(params().has("offset")).toBe(false);
    expect(calls.library!.at(-1)).toEqual([{ genre: "roman", limit: 12, offset: 0 }]);
  });

  it("boş kitaplık: kataloğa yönlendirir", async () => {
    reply = () => pageOf([], 0);
    const { el } = await render("/library");
    const link = Array.from(el.querySelectorAll("a")).find((a) => a.textContent === "Kataloğa göz at");
    expect(link?.getAttribute("href")).toBe("/");
  });

  it("boş sonuç: filtreleri temizler", async () => {
    reply = () => pageOf([], 0);
    const { el, params } = await render("/library?status=READING&q=yok");
    await click(buttonIn(el, "Filtreleri temizle"));
    expect(params().toString()).toBe("");
  });

  it("sonuç sayısı filtreyle eşleşir", async () => {
    reply = () => pageOf([itemOf(1)], 7);
    const { el } = await render("/library?status=READ");
    expect(el.textContent).toContain("7 kitap");
  });

  it("kartta durum ve puanım", async () => {
    reply = () => pageOf([itemOf(1, { status: "READING", my_rating: 8 })]);
    const { el } = await render("/library");
    const card = el.querySelector('[aria-label="Kitaplığım"] li')!;
    expect(card.textContent).toContain("Şu an okuyorum");
    expect(card.textContent).toContain("Puanım: 8");
  });

  it("katalogdan kaldırılan kart: bağlantı yok, yalnız çıkar", async () => {
    reply = () => pageOf([itemOf(5, { in_catalog: false })]);
    const { el } = await render("/library");
    const card = el.querySelector('[aria-label="Kitaplığım"] li')!;
    expect(card.textContent).toContain("Katalogdan kaldırıldı");
    expect(Array.from(card.querySelectorAll("a")).some((a) => a.textContent?.includes("Kitap 5"))).toBe(false);
    expect(buttonIn(card, "Çıkar")).toBeTruthy();
  });

  it("geri tuşuyla boşalmış bir sayfaya gelince son gerçek sayfaya döner", async () => {
    // Örn. 2. sayfadaki tek kitap, kitabın kendi sayfasından çıkarıldı; sonra "geri".
    reply = (p) => (p.offset === 12 ? pageOf([], 12, 12) : pageOf([itemOf(1)], 12, 0));
    const { el, params } = await render("/library?offset=12");
    expect(params().has("offset")).toBe(false);
    expect(el.querySelector('[aria-label="Kitaplığım"]')!.textContent).toContain("Kitap 1");
    expect(el.textContent).not.toContain("Kitaplığın boş");
  });

  it("son sayfadaki son kitap çıkınca önceki sayfa", async () => {
    reply = (p) => (p.offset === 12 ? pageOf([itemOf(13, { in_catalog: false })], 13, 12) : pageOf([itemOf(1)], 12, 0));
    const { el, params } = await render("/library?offset=12");
    await click(buttonIn(el.querySelector('[aria-label="Kitaplığım"] li')!, "Çıkar"));
    const dialog = document.querySelector<HTMLElement>("[role=dialog]")!;
    expect(dialog.textContent).toContain("Puanın ve yorumların silinmez");
    await click(buttonIn(dialog, "Çıkar"));
    expect(calls.removeFromLibrary).toEqual([[13]]);
    expect(params().has("offset")).toBe(false);
    expect(calls.library!.at(-1)).toEqual([{ limit: 12, offset: 0 }]);
  });

  it("liste yüklenemezse hata kutusu, liste yok", async () => {
    stub({ library: () => Promise.reject(new ApiError(500, "internal", "Kitaplık yüklenemedi")) });
    const { el } = await render("/library");
    expect(el.querySelector("[role=alert]")?.textContent).toBe("Kitaplık yüklenemedi");
    expect(el.querySelector('[aria-label="Kitaplığım"]')).toBeNull();
  });

  it("çıkarma başarısızsa hata bildirimi, kitap kalır, yeniden yüklenmez", async () => {
    reply = () => pageOf([itemOf(5, { in_catalog: false })]);
    stub({ removeFromLibrary: () => Promise.reject(new ApiError(500, "internal", "Çıkarılamadı")) });
    const { el, params } = await render("/library?status=READ");
    await removeFirst(el);
    expect(calls.removeFromLibrary).toEqual([[5]]);
    expect(errorToast()).toContain("Çıkarılamadı");
    expect(el.querySelector('[aria-label="Kitaplığım"]')!.textContent).toContain("Kitap 5");
    expect(calls.library).toHaveLength(1);
    expect(params().toString()).toBe("status=READ");
  });

  it("ilk sayfada çıkarınca aynı filtreyle yeniden yüklenir, adres değişmez", async () => {
    reply = () => pageOf([itemOf(5, { in_catalog: false }), itemOf(6, { in_catalog: false })]);
    const { el, params } = await render("/library?status=READ");
    await removeFirst(el);
    expect(calls.removeFromLibrary).toEqual([[5]]);
    expect(calls.library).toEqual([
      [{ status: "READ", limit: 12, offset: 0 }],
      [{ status: "READ", limit: 12, offset: 0 }],
    ]);
    expect(params().toString()).toBe("status=READ");
  });

  it("arama kırpılarak adrese yazılır, sayfa başa döner", async () => {
    reply = () => pageOf([itemOf(13)], 13, 12);
    const { el, params } = await render("/library?offset=12");
    const input = el.querySelector<HTMLInputElement>("input[type=search]")!;
    act(() => {
      Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")!.set!.call(input, " dune ");
      input.dispatchEvent(new Event("input", { bubbles: true }));
    });
    await act(async () => {
      input.form!.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
    });
    await flush();
    expect(params().get("q")).toBe("dune");
    expect(params().has("offset")).toBe(false);
    expect(calls.library!.at(-1)).toEqual([{ q: "dune", limit: 12, offset: 0 }]);
  });

  it("Puanladıklarım rated=yes gönderir", async () => {
    const { el, params } = await render("/library");
    await click(buttonIn(group(el, "Puanım"), "Puanladıklarım"));
    expect(params().get("rated")).toBe("yes");
    expect(calls.library!.at(-1)).toEqual([{ rated: "yes", limit: 12, offset: 0 }]);
  });

  it("adresteki tanınmayan değerler API'ye gitmez", async () => {
    const { el } = await render("/library?status=DONE&rated=maybe&sort=year");
    expect(calls.library).toEqual([[{ limit: 12, offset: 0 }]]);
    expect(buttonIn(group(el, "Durum"), "Tümü")!.getAttribute("aria-pressed")).toBe("true");
  });
});
