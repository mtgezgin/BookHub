import { act } from "react";
import { createRoot } from "react-dom/client";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { Page, PublicReview, Review, User } from "../api/types";
import { testQueryClient, withQuery } from "../test/render";

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

vi.mock("../api/client", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../api/client")>()),
  api: {},
}));
const h = vi.hoisted(() => ({ user: null as unknown }));
vi.mock("../auth/AuthContext", () => ({ useAuth: () => ({ user: h.user, ready: true }) }));

import { api, ApiError } from "../api/client";
import { ReviewSection } from "./ReviewSection";
import { ToastProvider } from "./toast";

/*
 * Kitap sayfasındaki İncelemeler bölümü (Faz 17B adım 5). Sahte sunucu: `list` herkese açık liste, `mine`
 * okurun incelemesi (null = yok). Her test hangi isteğin gittiğine ve ekranda ne kaldığına bakar. Önbellek
 * kuralları api/reviewQueries.test.tsx'te.
 */

const USER: User = { id: 7, email: "ali@example.com", display_name: "Ali", role: "USER", created_at: "" };
const ADMIN: User = { ...USER, id: 1, display_name: "Yönetici", role: "ADMIN" };
const BOOK = 12;

const publicOf = (id: number, over: Partial<PublicReview> = {}): PublicReview => ({
  id, author: { id: 100 + id, display_name: `Okur ${id}` }, author_score: null, body: `İnceleme ${id}`,
  has_spoiler: false, created_at: "2026-11-20T10:00:00Z", edited_at: null, ...over,
});
const mineOf = (over: Partial<Review> = {}): Review => ({
  id: 50, book_id: BOOK, body: "Benim incelemem", has_spoiler: false, version: 2,
  created_at: "2026-11-20T10:00:00Z", edited_at: null, hidden: null, ...over,
});
const pageOf = (items: PublicReview[]): Page<PublicReview> => ({ items, total: items.length, limit: 20, offset: 0 });
const noReview = () => Promise.reject(new ApiError(404, "no_review", "bu kitaba henüz inceleme yazmadın"));

let list: PublicReview[];
let mine: Review | null;
const calls: Record<string, unknown[][]> = {};
function stub(impls: Record<string, (...a: any[]) => unknown>) {
  for (const [k, f] of Object.entries(impls)) {
    (api as unknown as Record<string, unknown>)[k] = (...args: unknown[]) => {
      (calls[k] ??= []).push(args);
      return f(...args);
    };
  }
}

beforeEach(() => {
  for (const k of Object.keys(calls)) delete calls[k];
  h.user = USER;
  list = [publicOf(1), publicOf(2)];
  mine = null;
  stub({
    reviews: async () => pageOf(list),
    myReview: async () => mine ?? noReview(),
    saveReview: async (_book: number, d: { body: string; has_spoiler: boolean; version?: number }) => {
      mine = mineOf({ body: d.body, has_spoiler: d.has_spoiler, version: (d.version ?? 0) + 1 });
      return mine;
    },
    deleteReview: async () => {
      mine = null;
    },
    hideReview: async (id: number, reason: string) => {
      list = list.filter((r) => r.id !== id);
      return mineOf({ id, hidden: { at: "", reason } });
    },
    unhideReview: async (id: number) => {
      list = [publicOf(id), ...list];
      return mineOf({ id });
    },
  });
});

const cleanup: (() => void)[] = [];
afterEach(() => cleanup.splice(0).forEach((f) => f()));
const flush = async () => {
  for (let i = 0; i < 4; i++) await act(async () => {});
};

async function render() {
  const el = document.createElement("div");
  document.body.appendChild(el);
  const root = createRoot(el);
  act(() => root.render(withQuery(
    <ToastProvider><MemoryRouter><ReviewSection bookId={BOOK} /></MemoryRouter></ToastProvider>,
    testQueryClient(),
  )));
  cleanup.push(() => {
    act(() => root.unmount());
    el.remove();
  });
  await flush();
  return el;
}

const button = (text: string) =>
  Array.from(document.querySelectorAll<HTMLElement>("button, a")).find((b) => b.textContent?.trim() === text);
const listed = () => Array.from(document.querySelectorAll("ul[aria-label=İncelemeler] > li")).map((li) => li.textContent);
const dialog = () => document.querySelector("[role=dialog]");
const alertText = () => document.querySelector("[role=alert]")?.textContent ?? null;
async function click(target: Element | null | undefined) {
  expect(target, "tıklanacak öğe bulunamadı").toBeTruthy();
  await act(async () => (target as HTMLElement).click());
  await flush();
}
// React kontrollü alanı: değer prototipteki setter'la yazılır, yoksa React değişikliği görmez.
async function type(el: Element | null, value: string) {
  expect(el, "alan bulunamadı").toBeTruthy();
  await act(async () => {
    Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, "value")!.set!.call(el, value);
    el!.dispatchEvent(new Event("input", { bubbles: true }));
  });
}
const editor = () => document.querySelector("form[aria-label='İncelemeni yaz'] textarea");
const lastSave = () => calls.saveReview?.at(-1)?.[1];

describe("liste", () => {
  it("yazar, puanı ve 'düzenlendi'; başlıkta toplam", async () => {
    list = [publicOf(1, { author_score: 8, edited_at: "2026-11-21T10:00:00Z" }), publicOf(2)];
    await render();
    expect(document.querySelector("#reviews-heading")!.textContent).toBe("İncelemeler (2)");
    expect(listed()[0]).toContain("Okur 1");
    expect(listed()[0]).toContain("puanı 8/10");
    expect(listed()[0]).toContain("düzenlendi");
    expect(listed()[1]).not.toContain("/10");
  });

  it("spoiler metni açılana kadar DOM'da yok; düğme aria-expanded ile açar ve kapatır", async () => {
    list = [publicOf(1, { has_spoiler: true, body: "Katil uşak" })];
    await render();
    const toggle = button("Spoiler'ı göster")!;
    expect(document.body.textContent).not.toContain("Katil uşak");
    expect(toggle.getAttribute("aria-expanded")).toBe("false");
    expect(toggle.hasAttribute("aria-controls")).toBe(false);

    await click(toggle);
    const body = document.getElementById(button("Spoiler'ı gizle")!.getAttribute("aria-controls")!);
    expect(body?.textContent).toBe("Katil uşak");
    expect(button("Spoiler'ı gizle")!.getAttribute("aria-expanded")).toBe("true");

    await click(button("Spoiler'ı gizle"));
    expect(document.body.textContent).not.toContain("Katil uşak");
  });

  it("sunucunun belgesi çizilir: biçim görünür, yalnız spoiler parçası gizli (Faz 17C)", async () => {
    list = [publicOf(1, {
      body: "Sonu katil uşak", has_spoiler: true,
      doc: { type: "doc", content: [{ type: "paragraph", content: [
        { type: "text", text: "Sonu ", marks: [{ type: "bold" }] },
        { type: "text", text: "katil uşak", marks: [{ type: "spoiler" }] },
      ] }] },
    })];
    await render();
    expect(listed()[0]).toContain("Sonu ");
    expect(document.querySelector("ul[aria-label=İncelemeler] p strong")!.textContent).toBe("Sonu ");
    expect(document.body.textContent).not.toContain("katil uşak");
    expect(document.body.textContent).not.toContain("Spoiler içeriyor"); // tamamı spoiler değil: kart düğmesi yok
    await click(button("Spoiler'ı göster"));
    expect(listed()[0]).toContain("katil uşak");
  });

  it("girişsiz: giriş bağlantısı, yazma düğmesi yok, benim incelemem sorulmaz", async () => {
    h.user = null;
    await render();
    expect(button("giriş yap")).toBeTruthy();
    expect(button("İnceleme yaz")).toBeUndefined();
    expect(calls.myReview).toBeUndefined();
  });

  it("boş liste", async () => {
    list = [];
    await render();
    expect(document.body.textContent).toContain("Henüz inceleme yok.");
  });
});

describe("benim incelemem", () => {
  it("ilk yazım: boş metin gönderilmez; metin kırpılır, spoiler işaretlenir, sürüm gönderilmez", async () => {
    await render();
    await click(button("İnceleme yaz"));
    await click(button("Kaydet"));
    expect(alertText()).toBe("İncelemen boş olamaz.");
    expect(calls.saveReview).toBeUndefined();

    await type(editor(), "  Çok iyi  ");
    await click(document.querySelector("form[aria-label='İncelemeni yaz'] input[type=checkbox]"));
    await click(button("Kaydet"));
    expect(lastSave()).toEqual({ body: "Çok iyi", has_spoiler: true, version: undefined });
    expect(document.querySelector("[aria-labelledby=my-review-heading]")?.textContent).toContain("Çok iyi");
  });

  it("kendi incelemem listede ikinci kez görünmez", async () => {
    mine = mineOf();
    list = [publicOf(1, { author: { id: USER.id, display_name: "Ali" }, body: "Benim incelemem" }), publicOf(2)];
    await render();
    expect(listed()).toHaveLength(1);
    expect(listed()[0]).toContain("Okur 2");
  });

  it("kendi incelememde spoiler açık ve işaretli görünür, düğme yok (Faz 17C)", async () => {
    mine = mineOf({
      body: "Sonu katil uşak", has_spoiler: true,
      doc: { type: "doc", content: [{ type: "paragraph", content: [
        { type: "text", text: "Sonu " }, { type: "text", text: "katil uşak", marks: [{ type: "spoiler" }] },
      ] }] },
    });
    await render();
    const card = document.querySelector("section[aria-labelledby=my-review-heading]")!;
    expect(card.querySelector("mark")!.textContent).toBe("katil uşak");
    expect(card.textContent).toContain("spoiler içeriyor olarak işaretli");
    expect(Array.from(card.querySelectorAll("button")).map((x) => x.textContent)).toEqual(["Düzenle", "Sil"]);
  });

  it("gizlenmiş incelemede gerekçe görünür", async () => {
    mine = mineOf({ hidden: { at: "2026-11-22T10:00:00Z", reason: "Kişisel bilgi" } });
    await render();
    expect(document.querySelector("[aria-labelledby=my-review-heading]")?.textContent)
      .toContain("Bu inceleme yönetici tarafından gizlendi: Kişisel bilgi");
  });

  it("düzenleme: alan dolu açılır, okunan sürüm gönderilir", async () => {
    mine = mineOf({ version: 4 });
    await render();
    await click(button("Düzenle"));
    expect((editor() as HTMLTextAreaElement).value).toBe("Benim incelemem");
    await type(editor(), "Yeni metin");
    await click(button("Kaydet"));
    expect(lastSave()).toEqual({ body: "Yeni metin", has_spoiler: false, version: 4 });
  });

  it("silme onayı puanın silinmeyeceğini söyler; silince yazma düğmesi döner", async () => {
    mine = mineOf();
    await render();
    await click(button("Sil"));
    expect(dialog()?.textContent).toContain("Puanın silinmez.");
    await click(Array.from(dialog()!.querySelectorAll("button")).find((b) => b.textContent === "Sil"));
    expect(calls.deleteReview).toHaveLength(1);
    expect(button("İnceleme yaz")).toBeTruthy();
  });
});

describe("409: başka bir yerde değişmiş", () => {
  async function conflictAfter(change: () => void) {
    mine = mineOf({ version: 2 });
    await render();
    await click(button("Düzenle"));
    await type(editor(), "Benim yeni metnim");
    stub({ saveReview: async () => { change(); throw new ApiError(409, "version_conflict", "x"); } });
    await click(button("Kaydet"));
  }

  it("değişmişse şimdiki metin görünür, yazılan korunur; 'Benimkini kaydet' güncel sürümü gönderir", async () => {
    await conflictAfter(() => (mine = mineOf({ version: 3, body: "Öteki sekmenin metni" })));
    expect(alertText()).toContain("Öteki sekmenin metni");
    expect((editor() as HTMLTextAreaElement).value).toBe("Benim yeni metnim");
    stub({ saveReview: async (_b: number, d: Review) => mineOf(d) });
    await click(button("Benimkini kaydet"));
    expect(lastSave()).toMatchObject({ body: "Benim yeni metnim", version: 3 });
  });

  it("silinmişse 'silinmiş' der; kaydedince sürümsüz (yeni inceleme) gider", async () => {
    await conflictAfter(() => (mine = null));
    expect(alertText()).toContain("İncelemen başka bir yerde silinmiş");
    stub({ saveReview: async (_b: number, d: Review) => mineOf(d) });
    await click(button("Benimkini kaydet"));
    expect(lastSave()).toEqual({ body: "Benim yeni metnim", has_spoiler: false, version: undefined });
  });
});

describe("yönetici", () => {
  it("okur gizle düğmesi görmez", async () => {
    await render();
    expect(button("gizle")).toBeUndefined();
  });

  it("gerekçesiz gizlenmez; gizlenen listeden kalkar, aynı ziyarette geri açılır", async () => {
    h.user = ADMIN;
    await render();
    await click(Array.from(document.querySelectorAll("ul[aria-label=İncelemeler] button")).find((b) => b.textContent === "gizle"));
    await click(Array.from(dialog()!.querySelectorAll("button")).find((b) => b.textContent === "Gizle"));
    expect(dialog()?.querySelector("[role=alert]")?.textContent).toBe("Gerekçe yaz; incelemenin sahibine gösterilir.");
    expect(calls.hideReview).toBeUndefined();

    await type(dialog()!.querySelector("textarea"), " Kişisel bilgi ");
    await click(Array.from(dialog()!.querySelectorAll("button")).find((b) => b.textContent === "Gizle"));
    expect(calls.hideReview).toEqual([[1, "Kişisel bilgi"]]);
    expect(dialog()).toBeNull();
    expect(listed().join()).not.toContain("Okur 1");
    expect(document.body.textContent).toContain("Okur 1 tarafından yazılan inceleme gizlendi.");

    await click(button("Geri aç"));
    expect(calls.unhideReview).toEqual([[1]]);
    expect(listed().join()).toContain("Okur 1");
  });
});

it("liste yüklenemezse hata kutusu", async () => {
  stub({ reviews: async () => { throw new ApiError(500, "internal", "Sunucu hatası"); } });
  await render();
  expect(alertText()).toBe("Sunucu hatası");
});
