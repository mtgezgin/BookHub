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

async function submit(path: string, values: string[]) {
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
        </Routes>
        <Probe />
      </MemoryRouter>,
    ),
  );
  cleanup.push(() => {
    act(() => root.unmount());
    el.remove();
  });
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
  return { el, loc };
}

describe("Login", () => {
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
  });
});
