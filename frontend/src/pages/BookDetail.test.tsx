import { act } from "react";
import { createRoot } from "react-dom/client";
import { MemoryRouter, Route, Routes, useLocation, useNavigate } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { BookDetail as BookDetailData, Comment, RatingSummary, User } from "../api/types";

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

vi.mock("../api/client", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../api/client")>()),
  api: {},
}));
const h = vi.hoisted(() => ({ user: null as unknown }));
vi.mock("../auth/AuthContext", () => ({ useAuth: () => ({ user: h.user, ready: true }) }));

import { api, ApiError } from "../api/client";
import { ToastProvider } from "../components/toast";
import { BookDetail } from "./BookDetail";

const calls: Record<string, unknown[][]> = {};
function stub(impls: Record<string, (...a: any[]) => unknown>) {
  for (const [k, f] of Object.entries(impls)) {
    (api as unknown as Record<string, unknown>)[k] = (...args: unknown[]) => {
      (calls[k] ??= []).push(args);
      return f(...args);
    };
  }
}

const USER: User = { id: 7, email: "ali@example.com", display_name: "Ali", role: "USER", created_at: "" };
const ADMIN: User = { ...USER, id: 1, role: "ADMIN" };
const bookOf = (id: number): BookDetailData => ({
  id, title: `Kitap ${id}`, slug: `kitap-${id}`, author: "Yazar", description: "", cover_url: "",
  publication_year: null, page_count: null, language: "tr", genres: [], average_rating: null, rating_count: 0,
  created_at: "", updated_at: "", editions: [],
});
const summaryOf = (count: number): RatingSummary => ({
  average_rating: count ? 8 : null,
  rating_count: count,
  distribution: Array.from({ length: 10 }, (_, i) => ({ score: i + 1, votes: i === 7 ? count : 0 })),
});
const comment = (id: number, author: Comment["author"]): Comment => ({
  id, book_id: 1, author, content: `yorum ${id}`, created_at: "2026-09-01T00:00:00Z",
});
const notRated = () => Promise.reject(new ApiError(404, "not_rated", "bu kitaba henüz puan vermedin"));

const cleanup: (() => void)[] = [];
afterEach(() => cleanup.splice(0).forEach((f) => f()));
// jsdom scrollTo'yu uygulamıyor; çağrıldığını görmek için yerine casus.
const scrollTo = vi.fn();
beforeEach(() => {
  window.scrollTo = scrollTo as unknown as typeof window.scrollTo;
  scrollTo.mockClear();
  for (const k of Object.keys(calls)) delete calls[k];
  h.user = USER;
  stub({
    book: async (id: number) => bookOf(id),
    ratingSummary: async () => summaryOf(0),
    comments: async (_id: number, offset: number) => ({ items: [comment(offset + 1, { id: 7, display_name: "Ali" })], total: 1, limit: 10, offset }),
    myRating: notRated,
    books: async () => ({ items: [], total: 0, limit: 7, offset: 0 }),
    libraryStatuses: async () => ({ statuses: {} }),
  });
});

const flush = async () => {
  for (let i = 0; i < 4; i++) await act(async () => {});
};

function FromProbe() {
  const from = (useLocation().state as { from?: { pathname: string } } | null)?.from?.pathname;
  return <p data-from={from}>login</p>;
}

async function render(path = "/books/1") {
  const nav: { go?: (to: string) => void } = {};
  function Nav() {
    nav.go = useNavigate();
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
            <Route path="/books/:id" element={<><BookDetail /><Nav /></>} />
            <Route path="/login" element={<FromProbe />} />
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
  return { el, nav };
}
const button = (el: ParentNode, text: string) =>
  Array.from(el.querySelectorAll("button")).find((b) => b.textContent === text);
const alertText = (el: HTMLElement) => el.querySelector("[role=alert]")?.textContent ?? null;

function type(el: HTMLElement, text: string) {
  const ta = el.querySelector("textarea")!;
  act(() => {
    Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, "value")!.set!.call(ta, text);
    ta.dispatchEvent(new Event("input", { bubbles: true }));
  });
}

async function postComment(el: HTMLElement, text: string) {
  type(el, text);
  await act(async () => {
    el.querySelector("textarea")!.form!.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
  });
  await flush();
}

describe("BookDetail — kendi puanım", () => {
  it("myRating → not_rated hata göstermez", async () => {
    const { el } = await render();
    expect(calls.myRating).toEqual([[1]]);
    expect(alertText(el)).toBeNull();
  });

  it("not_rated dışındaki myRating hatası gösterilir", async () => {
    stub({ myRating: () => Promise.reject(new ApiError(500, "internal", "beklenmeyen bir hata oluştu")) });
    const { el } = await render();
    expect(alertText(el)).toBe("beklenmeyen bir hata oluştu");
  });

  it("girişsiz kullanıcı myRating isteği atmaz", async () => {
    h.user = null;
    await render();
    expect(calls.myRating).toBeUndefined();
  });
});

describe("BookDetail — yorum gönderme", () => {
  it("ilk sayfadayken gönderince listeyi yeniden yükler", async () => {
    stub({ addComment: async () => comment(99, null) });
    const { el } = await render();
    const before = calls.comments.length;
    await postComment(el, "harika");
    expect(calls.addComment).toEqual([[1, "harika"]]);
    expect(calls.comments.length).toBe(before + 1);
    expect(calls.comments.at(-1)).toEqual([1, 0]);
  });

  it("sonraki bir sayfadayken gönderince offset 0'a döner (fbbc532)", async () => {
    stub({
      addComment: async () => comment(99, null),
      comments: async (_id: number, offset: number) => ({ items: [comment(offset + 1, null)], total: 25, limit: 10, offset }),
    });
    const { el } = await render();
    await act(async () => button(el, "Eskiler →")!.click());
    await flush();
    expect(calls.comments.at(-1)).toEqual([1, 10]);
    await postComment(el, "yeni yorum");
    expect(calls.comments.at(-1)).toEqual([1, 0]);
  });
});

describe("BookDetail — başarısız yorum", () => {
  it("addComment reddedilirse hata gösterilir, metin korunur, yorumlar yeniden yüklenmez", async () => {
    stub({ addComment: () => Promise.reject(new ApiError(429, "rate_limited", "çok fazla istek")) });
    const { el } = await render();
    const before = calls.comments.length;
    await postComment(el, "kaybolmasın");
    expect(alertText(el)).toBe("çok fazla istek");
    expect(el.querySelector("textarea")!.value).toBe("kaybolmasın");
    expect(calls.comments.length).toBe(before);
  });
});

describe("BookDetail — admin yorum silme", () => {
  it("\"sil\" düğmesi yalnız ADMIN'e görünür", async () => {
    const asUser = await render();
    expect(button(asUser.el, "sil")).toBeUndefined();
    cleanup.splice(0).forEach((f) => f());
    h.user = ADMIN;
    const asAdmin = await render();
    expect(button(asAdmin.el, "sil")).toBeDefined();
  });

  it("başarısız silme gösterilir ve liste yine yeniden yüklenir", async () => {
    h.user = ADMIN;
    stub({ deleteComment: () => Promise.reject(new ApiError(500, "internal", "silinemedi")) });
    const { el } = await render();
    const before = calls.comments.length;
    await act(async () => button(el, "sil")!.click());
    await flush();
    expect(calls.deleteComment).toEqual([[1]]);
    expect(alertText(el)).toBe("silinemedi");
    expect(calls.comments.length).toBe(before + 1);
  });
});

describe("BookDetail — puan verme/kaldırma", () => {
  it("başarılı puan skoru vurgular ve özeti yeniden yükler", async () => {
    stub({ rate: async (_id: number, score: number) => ({ score }) });
    const { el } = await render();
    const before = calls.ratingSummary.length;
    await act(async () => button(el, "7")!.click());
    await flush();
    expect(calls.rate).toEqual([[1, 7]]);
    expect(button(el, "7")!.getAttribute("aria-pressed")).toBe("true");
    expect(calls.ratingSummary.length).toBe(before + 1);
  });

  it("başarılı puan kısa bir bildirimle onaylanır", async () => {
    stub({ rate: async (_id: number, score: number) => ({ score }) });
    const { el } = await render();
    await act(async () => button(el, "7")!.click());
    await flush();
    expect(el.querySelector('[aria-label="Bildirimler"]')!.textContent).toContain("Puanın kaydedildi: 7/10");
  });

  it("başarısız puan hatasını gösterir", async () => {
    stub({ rate: () => Promise.reject(new ApiError(404, "not_found", "kitap bulunamadı")) });
    const { el } = await render();
    await act(async () => button(el, "7")!.click());
    await flush();
    expect(alertText(el)).toBe("kitap bulunamadı");
    expect(button(el, "7")!.getAttribute("aria-pressed")).toBe("false");
  });

  it("unrate 'kaldır'ı gizler ve özeti yeniden yükler", async () => {
    stub({ myRating: async () => ({ score: 8, created_at: "", updated_at: "" }), removeRating: async () => ({}) });
    const { el } = await render();
    expect(button(el, "kaldır")).toBeDefined();
    const before = calls.ratingSummary.length;
    await act(async () => button(el, "kaldır")!.click());
    await flush();
    expect(button(el, "kaldır")).toBeUndefined();
    expect(calls.ratingSummary.length).toBe(before + 1);
  });

  it("başarısız unrate gösterilir", async () => {
    stub({
      myRating: async () => ({ score: 8, created_at: "", updated_at: "" }),
      removeRating: () => Promise.reject(new ApiError(500, "internal", "kaldırılamadı")),
    });
    const { el } = await render();
    await act(async () => button(el, "kaldır")!.click());
    await flush();
    expect(alertText(el)).toBe("kaldırılamadı");
    expect(button(el, "kaldır")).toBeDefined();
  });
});

describe("BookDetail — yükleme ve şablon", () => {
  it("404 kitap \"Kitap bulunamadı\" sayfasını ve kataloğa dönüş bağlantısını çizer", async () => {
    stub({ book: () => Promise.reject(new ApiError(404, "not_found", "kitap bulunamadı")) });
    const { el } = await render();
    expect(el.querySelector("h1")!.textContent).toBe("Kitap bulunamadı");
    const back = Array.from(el.querySelectorAll("a")).find((a) => a.textContent === "Kataloğa dön")!;
    expect(back.getAttribute("href")).toBe("/");
    expect(el.querySelector("textarea")).toBeNull();
  });

  it("404 dışındaki yükleme hatası ErrorBox'ta API mesajıyla gösterilir", async () => {
    stub({ book: () => Promise.reject(new ApiError(500, "internal", "kitap yüklenemedi")) });
    const { el } = await render();
    expect(alertText(el)).toBe("kitap yüklenemedi");
  });

  it("404 dışındaki yükleme hatasında sekme başlığı \"Kitap yüklenemedi\" olur, önceki sayfanınki kalmaz", async () => {
    document.title = "Önceki kitap — BookHub";
    stub({ book: () => Promise.reject(new ApiError(500, "internal", "kitap yüklenemedi")) });
    await render();
    expect(document.title).toBe("Kitap yüklenemedi — BookHub");
  });

  it("kitap yüklenirken iskelet gösterir", async () => {
    stub({ book: () => new Promise<BookDetailData>(() => {}) });
    const { el } = await render();
    expect(el.querySelector('[role=status][aria-label="Kitap yükleniyor"]')).not.toBeNull();
  });

  it("girişsiz \"giriş yap\" bağlantısı kitabın adresini taşır (girişten sonra geri dönülsün)", async () => {
    h.user = null;
    const { el } = await render();
    const link = Array.from(el.querySelectorAll("a")).find((a) => a.textContent === "giriş yap")!;
    await act(async () => link.dispatchEvent(new MouseEvent("click", { bubbles: true, cancelable: true, button: 0 })));
    expect(el.querySelector("[data-from]")!.getAttribute("data-from")).toBe("/books/1");
  });

  it("kitap yüklenirken sekme başlığı önceki sayfanınki kalmaz", async () => {
    document.title = "Önceki kitap — BookHub";
    stub({ book: () => new Promise(() => {}) });
    await render();
    expect(document.title).toBe("Yükleniyor — BookHub");
  });

  it("sekme başlığı kitabın adı", async () => {
    await render();
    expect(document.title).toBe("Kitap 1 — BookHub");
  });

  it("dil kodu Türkçe dil adıyla gösterilir", async () => {
    stub({ book: async (id: number) => ({ ...bookOf(id), language: "en" }) });
    const { el } = await render();
    expect(el.querySelector("header")!.textContent).toContain("İngilizce");
  });

  it("yeniden yükleme (başka kitap) eski hatayı temizler", async () => {
    stub({ book: (id: number) => (id === 1 ? Promise.reject(new ApiError(500, "internal", "kitap yüklenemedi")) : Promise.resolve(bookOf(id))) });
    const { el, nav } = await render();
    expect(alertText(el)).toBe("kitap yüklenemedi");
    await act(async () => nav.go!("/books/2"));
    await flush();
    expect(alertText(el)).toBeNull();
    expect(el.querySelector("h1")!.textContent).toBe("Kitap 2");
  });

  it("başka kitaba geçince öncekinin içeriği yeni kitap yüklenirken kalmaz, sayfa en üstten açılır", async () => {
    const { el, nav } = await render();
    expect(el.querySelector("h1")!.textContent).toBe("Kitap 1");
    let release!: () => void;
    stub({
      book: (id: number) =>
        id === 2 ? new Promise((res) => { release = () => res(bookOf(2)); }) : Promise.resolve(bookOf(id)),
    });
    scrollTo.mockClear();
    await act(async () => nav.go!("/books/2"));
    await flush();
    expect(el.textContent).not.toContain("Kitap 1");
    expect(scrollTo).toHaveBeenCalledWith(0, 0);
    await act(async () => release());
    await flush();
    expect(el.querySelector("h1")!.textContent).toBe("Kitap 2");
  });

  // Chromium 156'da window.scrollTo() bir Promise döndürüyor. Etki onu döndürseydi React
  // ayrılırken Promise'i temizlik fonksiyonu sanıp çağırır, uygulama boş ekranla çökerdi.
  it("scrollTo Promise döndüren tarayıcıda sayfadan ayrılmak çökmez", async () => {
    scrollTo.mockReturnValue(Promise.resolve());
    const { el, nav } = await render();
    expect(el.querySelector("h1")!.textContent).toBe("Kitap 1");
    await act(async () => nav.go!("/login"));
    await flush();
    expect(el.textContent).toContain("login");
    scrollTo.mockReturnValue(undefined);
  });

  it("rating_count 0 iken histogram gizli, oy varken görünür", async () => {
    const { el } = await render();
    expect(el.querySelector('[aria-label="Puan dağılımı"]')).toBeNull();
    cleanup.splice(0).forEach((f) => f());
    stub({ ratingSummary: async () => summaryOf(3) });
    const second = await render();
    expect(second.el.querySelector('[aria-label="Puan dağılımı"]')).not.toBeNull();
  });

  it("puan özeti hatası sayfayı düşürmez: kitap çizilir, hata ErrorBox'ta", async () => {
    stub({ ratingSummary: () => Promise.reject(new ApiError(500, "internal", "özet yüklenemedi")) });
    const { el } = await render();
    expect(el.querySelector("h1")!.textContent).toBe("Kitap 1");
    expect(alertText(el)).toBe("özet yüklenemedi");
  });

  it("yorum yoksa bunu söyler; giriş yapmış kullanıcıyı ilk yorumu yazmaya çağırır", async () => {
    stub({ comments: async () => ({ items: [], total: 0, limit: 10, offset: 0 }) });
    const { el } = await render();
    expect(el.textContent).toContain("Henüz yorum yok. İlk yorumu sen yaz.");
  });

  it("yazarı olmayan yorum \"silinmiş kullanıcı\" gösterir", async () => {
    stub({ comments: async () => ({ items: [comment(5, null)], total: 1, limit: 10, offset: 0 }) });
    const { el } = await render();
    expect(el.querySelector('[aria-label="Yorumlar"] strong')!.textContent).toBe("silinmiş kullanıcı");
  });
});

describe("BookDetail — baskılar (Faz 16)", () => {
  const edition = (id: number, language: string, is_primary: boolean, publisher = "") => ({
    id, title: `Baskı ${id}`, language, translator: "", publisher, publication_year: 2012, isbn: "",
    page_count: null, cover_url: "", is_primary,
  });

  it("birden çok baskıda: kapak altında varsayılan baskı notu, açıklamanın altında şerit (yorumlardan önce)", async () => {
    stub({ book: async (id: number) => ({ ...bookOf(id), description: "Açıklama", editions: [edition(1, "tr", true, "Yediveren Yayınları"), edition(2, "ru", false)] }) });
    const { el } = await render();
    expect(el.textContent).toContain("Türkçe baskı · Yediveren Yayınları");
    const strip = el.querySelector('section[aria-labelledby="editions-heading"]')!;
    expect(strip.querySelectorAll("li")).toHaveLength(1);
    expect(strip.textContent).toContain("Rusça · 2012");
    const comments = el.querySelector('section[aria-labelledby="comments-heading"]')!;
    expect(strip.compareDocumentPosition(comments) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });

  it("tek baskıda ne not ne şerit", async () => {
    stub({ book: async (id: number) => ({ ...bookOf(id), editions: [edition(1, "tr", true, "Yediveren Yayınları")] }) });
    const { el } = await render();
    expect(el.querySelector("#editions-heading")).toBeNull();
    expect(el.textContent).not.toContain("Türkçe baskı");
  });
});

describe("BookDetail — aynı türden kitaplar", () => {
  const withGenre = (id: number): BookDetailData => ({ ...bookOf(id), genres: [{ id: 1, name: "Roman", slug: "roman" }] });
  const SIMILAR = '[aria-labelledby="similar-heading"]';

  it("ilk türden en fazla 6 kitap gösterir, mevcut kitabı göstermez", async () => {
    stub({
      book: async (id: number) => withGenre(id),
      books: async () => ({ items: [1, 2, 3, 4, 5, 6, 7, 8].map(withGenre), total: 8, limit: 7, offset: 0 }),
    });
    const { el } = await render();
    expect(calls.books).toEqual([[{ genre: "roman", limit: 7 }]]);
    const hrefs = Array.from(el.querySelectorAll(`${SIMILAR} h3 a`)).map((a) => a.getAttribute("href"));
    expect(hrefs).toEqual(["/books/2", "/books/3", "/books/4", "/books/5", "/books/6", "/books/7"]);
  });

  it("türü olmayan kitapta istek atılmaz, bölüm çizilmez", async () => {
    const { el } = await render();
    expect(calls.books).toBeUndefined();
    expect(el.querySelector(SIMILAR)).toBeNull();
  });

  it("istek başarısızsa hata gösterilmez, bölüm çizilmez (yardımcı içerik)", async () => {
    stub({
      book: async (id: number) => withGenre(id),
      books: () => Promise.reject(new ApiError(500, "internal", "liste yüklenemedi")),
    });
    const { el } = await render();
    expect(alertText(el)).toBeNull();
    expect(el.querySelector(SIMILAR)).toBeNull();
  });
  it("giriş yapmış okurda benzer kitabın kartında kitaplık durumu", async () => {
    stub({
      book: async (id: number) => withGenre(id),
      books: async () => ({ items: [1, 2, 3].map(withGenre), total: 3, limit: 7, offset: 0 }),
      libraryStatuses: async () => ({ statuses: { "3": "READ" } }),
    });
    const { el } = await render();
    expect(calls.libraryStatuses).toContainEqual([[2, 3]]);
    const cards = Array.from(el.querySelectorAll<HTMLElement>(`${SIMILAR} li`));
    const card = (id: number) => cards.find((c) => c.querySelector(`a[href="/books/${id}"]`));
    expect(card(3)!.textContent).toContain("Okudum");
    expect(card(2)!.textContent).not.toContain("Okudum");
  });
});

describe("BookDetail — yorum deneyimi", () => {
  it("gönderilirken düğme kilitli: çift tıklama iki yorum yazmaz", async () => {
    stub({ addComment: () => new Promise(() => {}) });
    const { el } = await render();
    type(el, "merhaba");
    await act(async () => {
      el.querySelector("textarea")!.form!.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
    });
    expect(button(el, "Gönderiliyor…")!.disabled).toBe(true);
  });

  it("karakter sayacı yazılanı ve sınırı gösterir", async () => {
    const { el } = await render();
    type(el, "merhaba");
    expect(el.textContent).toContain("7/2000");
  });

  it("yorum zamanı göreli yazılır, makine okunur tarih <time>'da", async () => {
    const iso = new Date(Date.now() - 3 * 24 * 60 * 60 * 1000).toISOString();
    stub({ comments: async () => ({ items: [{ ...comment(9, { id: 7, display_name: "Ali" }), created_at: iso }], total: 1, limit: 10, offset: 0 }) });
    const { el } = await render();
    const time = el.querySelector('[aria-label="Yorumlar"] time')!;
    expect(time.textContent).toBe("3 gün önce");
    expect(time.getAttribute("dateTime")).toBe(iso);
  });

  it("yorumda yazarın baş harfi avatar olarak görünür (dekoratif)", async () => {
    const { el } = await render();
    const avatar = el.querySelector('[aria-label="Yorumlar"] li [aria-hidden="true"]')!;
    expect(avatar.textContent).toBe("A");
  });
});
