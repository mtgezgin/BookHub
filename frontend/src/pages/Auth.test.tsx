import { act } from "react";
import { createRoot } from "react-dom/client";
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

vi.mock("../api/client", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../api/client")>()),
  api: {},
}));
const h = vi.hoisted(() => ({ login: (() => Promise.resolve()) as (e: string, p: string) => Promise<void> }));
vi.mock("../auth/AuthContext", () => ({ useAuth: () => ({ login: h.login }) }));

import { api, ApiError } from "../api/client";
import { Login, Register } from "./Auth";

let order: string[] = [];
const cleanup: (() => void)[] = [];
afterEach(() => cleanup.splice(0).forEach((f) => f()));
beforeEach(() => {
  order = [];
  h.login = async (e, p) => {
    order.push(`login:${e}:${p}`);
  };
  (api as unknown as Record<string, unknown>).register = async (e: string, n: string) => {
    order.push(`register:${e}:${n}`);
  };
});

type Entry = string | { pathname: string; state: unknown };

async function render(path: Entry) {
  const loc = { pathname: "" };
  function Probe() {
    loc.pathname = useLocation().pathname;
    return null;
  }
  const el = document.createElement("div");
  document.body.appendChild(el);
  const root = createRoot(el);
  act(() =>
    root.render(
      <MemoryRouter initialEntries={[path]}>
        <Routes>
          <Route path="/login" element={<Login />} />
          <Route path="/register" element={<Register />} />
          <Route path="/" element={<p>anasayfa</p>} />
          <Route path="/books/:id" element={<p>kitap</p>} />
        </Routes>
        <Probe />
      </MemoryRouter>,
    ),
  );
  cleanup.push(() => {
    act(() => root.unmount());
    el.remove();
  });
  return { el, loc };
}

async function fillAndSubmit(el: HTMLElement, values: string[]) {
  Array.from(el.querySelectorAll("input")).forEach((input, i) =>
    act(() => {
      Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")!.set!.call(input, values[i]);
      input.dispatchEvent(new Event("input", { bubbles: true }));
    }),
  );
  await act(async () => {
    el.querySelector("form")!.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
  });
  await act(async () => {});
}

async function submit(path: Entry, values: string[]) {
  const { el, loc } = await render(path);
  await fillAndSubmit(el, values);
  return { el, loc };
}

const submitButton = (el: HTMLElement) => Array.from(el.querySelectorAll("button")).find((b) => b.type === "submit")!;

async function followLink(el: HTMLElement, text: string) {
  await act(async () => Array.from(el.querySelectorAll("a")).find((a) => a.textContent === text)!.click());
}

const fromBook = { pathname: "/login", state: { from: { pathname: "/books/5", search: "?yorum=1" } } };

describe("Login", () => {
  it("korumalı sayfadan ya da \"giriş yap\" bağlantısından geldiyse oraya döner", async () => {
    const { loc } = await submit(fromBook, ["ali@example.com", "parola123"]);
    expect(loc.pathname).toBe("/books/5");
  });

  it("uygulama dışı bir adrese (//site) asla yönlendirmez", async () => {
    const { loc } = await submit({ pathname: "/login", state: { from: { pathname: "//kotu.example", search: "" } } }, ["a@b.co", "parola123"]);
    expect(loc.pathname).toBe("/");
  });

  it("ters bölü ile başlayan yola (/\\site) da yönlendirmez", async () => {
    const { loc } = await submit({ pathname: "/login", state: { from: { pathname: "/\\kotu.example", search: "" } } }, ["a@b.co", "parola123"]);
    expect(loc.pathname).toBe("/");
  });

  it("\"Kayıt ol\" bağlantısı dönüş adresini taşır: kayıttan sonra geldiği sayfaya döner", async () => {
    const { el, loc } = await render(fromBook);
    await followLink(el, "Kayıt ol");
    expect(loc.pathname).toBe("/register");
    await fillAndSubmit(el, ["ali@example.com", "Ali", "parola123"]);
    expect(loc.pathname).toBe("/books/5");
  });

  it("başarısız girişten sonra düğme yeniden kullanılabilir ve \"Giriş\" yazar", async () => {
    h.login = () => Promise.reject(new ApiError(401, "invalid_credentials", "e-posta ya da parola hatalı"));
    const { el } = await submit("/login", ["ali@example.com", "yanlis"]);
    expect(submitButton(el).disabled).toBe(false);
    expect(submitButton(el).textContent).toBe("Giriş");
  });

  it("gönderilirken düğme kilitli: ikinci tıklama ikinci isteği atmaz", async () => {
    h.login = () => new Promise(() => {});
    const { el } = await submit("/login", ["ali@example.com", "parola123"]);
    const btn = Array.from(el.querySelectorAll("button")).find((b) => b.type === "submit")!;
    expect(btn.disabled).toBe(true);
    expect(btn.textContent).toBe("Giriş yapılıyor…");
  });

  it("parola alanı \"Parolayı göster\" düğmesiyle görünür olur", async () => {
    const { el } = await render("/login");
    const input = el.querySelector<HTMLInputElement>("input[autocomplete=current-password]")!;
    const toggle = el.querySelector<HTMLButtonElement>('button[aria-label="Parolayı göster"]')!;
    expect(input.type).toBe("password");
    act(() => toggle.click());
    expect(input.type).toBe("text");
    expect(toggle.getAttribute("aria-pressed")).toBe("true");
  });

  it("parola alanının etiketi alana bağlı (ekran okuyucu \"Parola\" duyar)", async () => {
    const { el } = await render("/login");
    const input = el.querySelector<HTMLInputElement>("input[autocomplete=current-password]")!;
    expect(el.querySelector(`label[for="${input.id}"]`)!.textContent).toBe("Parola");
  });

  it("başarıda / adresine gider", async () => {
    const { loc } = await submit("/login", ["ali@example.com", "parola123"]);
    expect(order).toEqual(["login:ali@example.com:parola123"]);
    expect(loc.pathname).toBe("/");
  });

  it("başarısızlıkta API mesajını gösterir ve sayfada kalır", async () => {
    h.login = () => Promise.reject(new ApiError(401, "invalid_credentials", "e-posta ya da parola hatalı"));
    const { el, loc } = await submit("/login", ["ali@example.com", "yanlis"]);
    expect(el.querySelector("[role=alert]")!.textContent).toBe("e-posta ya da parola hatalı");
    expect(loc.pathname).toBe("/login");
  });
});

describe("Register", () => {
  it("demo uyarısı formdan önce görünür ve parola alanına bağlı (ekran okuyucu alanda da duyar)", async () => {
    const { el } = await render("/register");
    const note = el.querySelector<HTMLElement>("[role=note]")!;
    expect(note.textContent).toContain("öğrenme projesinin demosu");
    expect(note.textContent).toContain("Başka bir yerde kullandığın parolayı kullanma");
    // Uyarı ilk alandan önce: DOM sırası = görme ve ekran okuyucu okuma sırası.
    const firstInput = el.querySelector("input")!;
    expect(note.compareDocumentPosition(firstInput) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    const password = el.querySelector<HTMLInputElement>("input[autocomplete=new-password]")!;
    expect(password.getAttribute("aria-describedby")).toBe(note.id);
  });

  it("giriş ekranında demo uyarısı yok (hesabı olan kayıtta görmüştür)", async () => {
    const { el } = await render("/login");
    expect(el.querySelector("[role=note]")).toBeNull();
  });

  it("kayıttan sonra da geldiği sayfaya döner", async () => {
    const { loc } = await submit({ pathname: "/register", state: fromBook.state }, ["ali@example.com", "Ali", "parola123"]);
    expect(loc.pathname).toBe("/books/5");
  });

  it("önce api.register, sonra login, sonra / adresine gider", async () => {
    const { loc } = await submit("/register", ["ali@example.com", "Ali", "parola123"]);
    expect(order).toEqual(["register:ali@example.com:Ali", "login:ali@example.com:parola123"]);
    expect(loc.pathname).toBe("/");
  });

  it("register hatası login'den önce durur", async () => {
    (api as unknown as Record<string, unknown>).register = () =>
      Promise.reject(new ApiError(409, "email_taken", "bu e-posta zaten kayıtlı"));
    const { el, loc } = await submit("/register", ["ali@example.com", "Ali", "parola123"]);
    expect(order).toEqual([]);
    expect(el.querySelector("[role=alert]")!.textContent).toBe("bu e-posta zaten kayıtlı");
    expect(loc.pathname).toBe("/register");
    expect(submitButton(el).disabled).toBe(false);
    expect(submitButton(el).textContent).toBe("Kayıt ol");
  });

  it("\"Giriş yap\" bağlantısı dönüş adresini taşır: girişten sonra geldiği sayfaya döner", async () => {
    const { el, loc } = await render({ pathname: "/register", state: fromBook.state });
    await followLink(el, "Giriş yap");
    expect(loc.pathname).toBe("/login");
    await fillAndSubmit(el, ["ali@example.com", "parola123"]);
    expect(loc.pathname).toBe("/books/5");
  });

  it("kayıt sürerken düğme kilitli: ikinci tıklama register'ı yeniden çağırmaz", async () => {
    let calls = 0;
    (api as unknown as Record<string, unknown>).register = () => {
      calls++;
      return new Promise(() => {});
    };
    const { el } = await submit("/register", ["ali@example.com", "Ali", "parola123"]);
    expect(submitButton(el).disabled).toBe(true);
    expect(submitButton(el).textContent).toBe("Kaydediliyor…");
    await act(async () => submitButton(el).click());
    expect(calls).toBe(1);
  });
});
