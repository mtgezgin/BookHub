import { act } from "react";
import { createRoot } from "react-dom/client";
import { MemoryRouter } from "react-router-dom";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { User } from "./api/types";

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

const h = vi.hoisted(() => ({
  auth: { user: null as unknown, ready: true, logout: () => Promise.resolve(), login: () => Promise.resolve() },
}));
vi.mock("./auth/AuthContext", () => ({ useAuth: () => h.auth }));
// Sayfalar kendi API çağrılarını yapar; burada yalnız App'in yönlendirmesi ve nav'ı test ediliyor.
vi.mock("./pages/Home", () => ({ Home: () => <p>home-page</p> }));
vi.mock("./pages/BookDetail", () => ({ BookDetail: () => <p>detail-page</p> }));
vi.mock("./pages/Suggest", () => ({ Suggest: () => <p>suggest-page</p> }));
vi.mock("./pages/Auth", () => ({ Login: () => <p>login-page</p>, Register: () => <p>register-page</p> }));
vi.mock("./pages/admin/Moderation", () => ({ Moderation: () => <p>moderation-page</p> }));
vi.mock("./pages/Account", () => ({ Account: () => <p>account-page</p> }));

import { App } from "./App";

const USER: User = { id: 7, email: "ali@example.com", display_name: "Ali", role: "USER", created_at: "" };
const ADMIN: User = { ...USER, id: 1, role: "ADMIN" };

const cleanup: (() => void)[] = [];
afterEach(() => cleanup.splice(0).forEach((f) => f()));

function render(path: string, user: User | null, ready = true) {
  h.auth = { ...h.auth, user, ready };
  const el = document.createElement("div");
  document.body.appendChild(el);
  const root = createRoot(el);
  act(() => root.render(<MemoryRouter initialEntries={[path]}><App /></MemoryRouter>));
  cleanup.push(() => {
    act(() => root.unmount());
    el.remove();
  });
  return el;
}
const main = (el: HTMLElement) => el.querySelector("main")!.textContent;
const navLinks = (el: HTMLElement) => Array.from(el.querySelectorAll("nav a")).map((a) => a.textContent);

describe("RequireRole", () => {
  it("hazır değilken \"Yükleniyor…\" gösterir", () => {
    expect(main(render("/suggest", null, false))).toBe("Yükleniyor…");
  });

  it("kullanıcı yoksa /login'e yönlendirir", () => {
    expect(main(render("/suggest", null))).toBe("login-page");
  });

  it("USER /admin'de yetki mesajı görür", () => {
    expect(main(render("/admin", USER))).toBe("Bu sayfa için yetkin yok.");
  });

  it("ADMIN /admin'de içeriği görür", () => {
    expect(main(render("/admin", ADMIN))).toBe("moderation-page");
  });

  // /account ROL İSTEMEZ ama GİRİŞ ister: sayfa parola değiştirmeyi ve (ileride) oturum
  // yönetimini barındırıyor. Girişsiz bir ziyaret sayfayı ÇİZMEMELİ — arayüzdeki bu koruma
  // yalnız deneyim içindir (asıl kontrol backend'de requireAuth), ama boş bir "Hesabım"
  // formu göstermek kullanıcıyı parolasını girmeye davet etmek olurdu.
  it("girişsiz /account login'e yönlendirir", () => {
    expect(main(render("/account", null))).toBe("login-page");
  });

  it("girişli kullanıcı /account sayfasını görür (rol şartı YOK)", () => {
    expect(main(render("/account", USER))).toBe("account-page");
  });

  it("ADMIN de /account sayfasını görür", () => {
    expect(main(render("/account", ADMIN))).toBe("account-page");
  });
});

describe("Bilinmeyen adres", () => {
  it("\"Sayfa bulunamadı\" başlığını ve ana sayfaya dönüş bağlantısını gösterir", () => {
    const el = render("/yok-boyle-sayfa", null);
    expect(el.querySelector("main h1")!.textContent).toBe("Sayfa bulunamadı");
    const back = Array.from(el.querySelectorAll("main a")).find((a) => a.textContent === "Kataloğa dön")!;
    expect(back.getAttribute("href")).toBe("/");
  });
});

describe("App nav", () => {
  it("girişsiz: \"Kitap öner\" ve \"Moderasyon\" yok", () => {
    const links = navLinks(render("/", null));
    expect(links).not.toContain("Kitap öner");
    expect(links).not.toContain("Moderasyon");
  });

  it("USER: \"Kitap öner\" var, \"Moderasyon\" yok", () => {
    const links = navLinks(render("/", USER));
    expect(links).toContain("Kitap öner");
    expect(links).not.toContain("Moderasyon");
  });

  it("ADMIN: ikisi de var", () => {
    const links = navLinks(render("/", ADMIN));
    expect(links).toContain("Kitap öner");
    expect(links).toContain("Moderasyon");
  });

  it("girişsiz: \"Hesabım\" yok", () => {
    expect(navLinks(render("/", null))).not.toContain("Hesabım");
  });

  it("USER: \"Hesabım\" var", () => {
    expect(navLinks(render("/", USER))).toContain("Hesabım");
  });

  it("ADMIN: \"Hesabım\" var", () => {
    expect(navLinks(render("/", ADMIN))).toContain("Hesabım");
  });

  it("ana menü adıyla bulunur (ekran okuyucu ve e2e aynı adı kullanır)", () => {
    expect(render("/", null).querySelector('nav[aria-label="Ana menü"]')).not.toBeNull();
  });

  it("bulunulan sayfanın bağlantısı aria-current=page taşır, diğerleri taşımaz", () => {
    const el = render("/suggest", USER);
    const link = (text: string) => Array.from(el.querySelectorAll("nav a")).find((a) => a.textContent === text)!;
    expect(link("Kitap öner").getAttribute("aria-current")).toBe("page");
    expect(link("Hesabım").getAttribute("aria-current")).toBeNull();
  });

  it("\"Çıkış\" logout'u çağırır", () => {
    let calls = 0;
    h.auth = { ...h.auth, logout: async () => void calls++ };
    const el = render("/", USER);
    const out = Array.from(el.querySelectorAll("nav button")).find((b) => b.textContent === "Çıkış") as HTMLButtonElement;
    act(() => out.click());
    expect(calls).toBe(1);
  });
});
