import { act } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

vi.mock("../api/client", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../api/client")>()),
  api: {},
}));
const SIGNED_IN = { id: 7, email: "ali@example.com", display_name: "Ali", role: "USER", created_at: "" };
const h = vi.hoisted(() => ({ auth: { user: null as unknown, ready: true } }));
// Sayfa yalnız kullanıcı adını göstermek için useAuth kullanıyor; oturum mantığı client.ts'te.
vi.mock("../auth/AuthContext", () => ({ useAuth: () => h.auth }));

import { api, ApiError } from "../api/client";
import { Account } from "./Account";

/** changePassword çağrılarının argümanları: "istek hiç atılmadı" iddiasının kanıtı. */
let calls: [string, string][] = [];
let result: () => Promise<unknown>;

const cleanup: (() => void)[] = [];
afterEach(() => cleanup.splice(0).forEach((f) => f()));
beforeEach(() => {
  h.auth = { user: SIGNED_IN, ready: true };
  calls = [];
  result = async () => ({ id: 7 });
  (api as unknown as Record<string, unknown>).changePassword = (current: string, next: string) => {
    calls.push([current, next]);
    return result();
  };
});

function render() {
  const el = document.createElement("div");
  document.body.appendChild(el);
  const root = createRoot(el);
  act(() => root.render(<Account />));
  cleanup.push(() => {
    act(() => root.unmount());
    el.remove();
  });
  return el;
}

function fill(el: HTMLElement, values: { current: string; next: string; repeat: string }) {
  const [current, next, repeat] = Array.from(el.querySelectorAll("input"));
  for (const [input, v] of [[current, values.current], [next, values.next], [repeat, values.repeat]] as const) {
    act(() => {
      Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")!.set!.call(input, v);
      input.dispatchEvent(new Event("input", { bubbles: true }));
    });
  }
  return { current, next, repeat };
}

async function submit(el: HTMLElement) {
  await act(async () => {
    el.querySelector("form")!.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
  });
  await act(async () => {});
}

const alerts = (el: HTMLElement) => Array.from(el.querySelectorAll("[role=alert]")).map((n) => n.textContent);
const status = (el: HTMLElement) => el.querySelector("[role=status]")?.textContent ?? null;

/** identity: sayfanın üstündeki "ad · e-posta" satırı (yoksa null). */
const identity = (el: HTMLElement) => el.querySelector("p.muted.small")?.textContent ?? null;

describe("Account — kimlik satırı", () => {
  it("useAuth kullanıcısının adını ve e-postasını gösterir", () => {
    expect(identity(render())).toBe("Ali · ali@example.com");
  });

  it("kullanıcı yokken kimlik satırı hiç basılmaz", () => {
    h.auth = { user: null, ready: true };
    expect(identity(render())).toBeNull();
  });
});

describe("Account — parola değiştirme", () => {
  it("parolalar alanları type=password ve doğru autocomplete ile gelir", () => {
    const el = render();
    const inputs = Array.from(el.querySelectorAll("input"));
    expect(inputs.map((i) => i.type)).toEqual(["password", "password", "password"]);
    // Tarayıcının parola yöneticisi "mevcut" ile "yeni"yi ayırt edebilsin.
    expect(inputs.map((i) => i.autocomplete)).toEqual(["current-password", "new-password", "new-password"]);
  });

  // ASIL İDDİA: tekrar alanı uyuşmuyorsa İSTEK HİÇ ATILMAZ. Sunucuya gitseydi yanlış
  // yazılmış bir parola kaydedilir ve kullanıcı TÜM cihazlarından düşerdi (aileler iptal).
  it("yeni parolalar uyuşmazsa API çağrılmaz, uyarı gösterilir", async () => {
    const el = render();
    fill(el, { current: "eski-parola", next: "yeni-parola", repeat: "yeni-parolaa" });
    await submit(el);
    expect(calls).toEqual([]);
    expect(alerts(el)).toEqual(["Yeni parolalar birbiriyle eşleşmiyor."]);
    expect(status(el)).toBeNull();
  });

  it("uyuşmazlık düzeltilince istek gider ve uyarı kalkar", async () => {
    const el = render();
    fill(el, { current: "eski-parola", next: "yeni-parola", repeat: "yanlis" });
    await submit(el);
    expect(calls).toEqual([]);

    fill(el, { current: "eski-parola", next: "yeni-parola", repeat: "yeni-parola" });
    await submit(el);
    expect(calls).toEqual([["eski-parola", "yeni-parola"]]);
    expect(alerts(el)).toEqual([]);
  });

  it("başarıda form temizlenir ve diğer cihazların düştüğü söylenir", async () => {
    const el = render();
    const inputs = fill(el, { current: "eski-parola", next: "yeni-parola", repeat: "yeni-parola" });
    await submit(el);
    expect(calls).toEqual([["eski-parola", "yeni-parola"]]);
    // Parolalar ekranda kalmamalı (omuz sörfü / ekran paylaşımı).
    expect([inputs.current.value, inputs.next.value, inputs.repeat.value]).toEqual(["", "", ""]);
    expect(status(el)).toBe("Parolan değişti. Diğer tüm cihazlardaki oturumlar kapatıldı.");
  });

  it("yanlış mevcut parola (403) backend'in mesajını gösterir, form KORUNUR", async () => {
    result = () => Promise.reject(new ApiError(403, "invalid_current_password", "mevcut parola hatalı"));
    const el = render();
    const inputs = fill(el, { current: "yanlis", next: "yeni-parola", repeat: "yeni-parola" });
    await submit(el);
    expect(alerts(el)).toEqual(["mevcut parola hatalı"]);
    expect(status(el)).toBeNull();
    // Kullanıcı yalnız yanlış alanı düzeltsin; her şeyi yeniden yazmak zorunda kalmasın.
    expect(inputs.next.value).toBe("yeni-parola");
  });

  it("politika hatası (400) da gösterilir", async () => {
    result = () => Promise.reject(new ApiError(400, "validation_error", "password en az 8 karakter olmalı"));
    const el = render();
    fill(el, { current: "eski-parola", next: "kisa", repeat: "kisa" });
    await submit(el);
    expect(alerts(el)).toEqual(["password en az 8 karakter olmalı"]);
  });

  it("başarıdan sonra yeni bir hata eski başarı mesajını siler", async () => {
    const el = render();
    fill(el, { current: "eski-parola", next: "yeni-parola", repeat: "yeni-parola" });
    await submit(el);
    expect(status(el)).not.toBeNull();

    result = () => Promise.reject(new ApiError(403, "invalid_current_password", "mevcut parola hatalı"));
    fill(el, { current: "yine-yanlis", next: "baska-parola", repeat: "baska-parola" });
    await submit(el);
    expect(status(el)).toBeNull();
    expect(alerts(el)).toEqual(["mevcut parola hatalı"]);
  });

  it("istek sürerken düğme devre dışı (çift gönderim yok)", async () => {
    let release!: (v: unknown) => void;
    result = () => new Promise((r) => (release = r));
    const el = render();
    fill(el, { current: "eski-parola", next: "yeni-parola", repeat: "yeni-parola" });
    const button = el.querySelector("button")!;
    act(() => {
      el.querySelector("form")!.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
    });
    expect(button.disabled).toBe(true);
    await act(async () => release({ id: 7 }));
    expect(button.disabled).toBe(false);
    expect(calls).toHaveLength(1);
  });
});
