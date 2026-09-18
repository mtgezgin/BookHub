import { act } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { User } from "../api/types";

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

// API istemcisini sahtele: restore/logout'u test kontrol eder, subscribeSession kaydedilen
// fonksiyonu yakalar (istemci oturumu düşürdüğünde çağrılan kanca).
const h = vi.hoisted(() => ({ session: null as null | ((u: unknown) => void) }));
vi.mock("../api/client", () => ({
  api: { restore: vi.fn(), logout: vi.fn(), login: vi.fn() },
  subscribeSession: (fn: (u: unknown) => void) => {
    h.session = fn;
  },
}));

import { api } from "../api/client";
import { AuthProvider, useAuth } from "./AuthContext";

const ali: User = { id: 7, email: "ali@example.com", display_name: "Ali", role: "USER", created_at: "" };
type State = ReturnType<typeof useAuth>;

const cleanup: (() => void)[] = [];
afterEach(() => cleanup.splice(0).forEach((f) => f()));
beforeEach(() => vi.mocked(api.restore).mockReset());

function mount() {
  const box: { current?: State } = {};
  function Probe() {
    box.current = useAuth();
    return null;
  }
  const root = createRoot(document.createElement("div"));
  act(() => root.render(<AuthProvider><Probe /></AuthProvider>));
  cleanup.push(() => act(() => root.unmount()));
  return box;
}

function deferred<T>() {
  let resolve!: (v: T) => void;
  let reject!: (e: unknown) => void;
  const promise = new Promise<T>((res, rej) => ((resolve = res), (reject = rej)));
  return { promise, resolve, reject };
}

describe("AuthProvider", () => {
  it("açılışta api.restore çağırır; bitene kadar ready=false, sonra user + ready=true", async () => {
    const d = deferred<User | null>();
    vi.mocked(api.restore).mockReturnValue(d.promise);
    const box = mount();
    expect(api.restore).toHaveBeenCalledTimes(1);
    expect(box.current!).toMatchObject({ user: null, ready: false });
    await act(async () => d.resolve(ali));
    expect(box.current!).toMatchObject({ user: ali, ready: true });
  });

  it("reddedilen restore user=null ve ready=true bırakır", async () => {
    // vi.fn yerine düz fonksiyon: vitest'in mock sonuç takibi reddedilen Promise'i testin
    // hatası sayıyor; AuthProvider'ın kendi .catch'i ise reddi zaten işliyor.
    const original = api.restore;
    api.restore = () => Promise.reject(new Error("ağ"));
    cleanup.push(() => (api.restore = original));
    const box = mount();
    await act(async () => {});
    expect(box.current!).toMatchObject({ user: null, ready: true });
  });

  it("istemcinin düşürdüğü oturum (subscribeSession(null)) context'teki user'ı temizler", async () => {
    vi.mocked(api.restore).mockResolvedValue(ali);
    const box = mount();
    await act(async () => {});
    expect(box.current!.user).toEqual(ali);
    act(() => h.session!(null));
    expect(box.current!.user).toBeNull();
  });

  it("logout api.logout'u BEKLER, sonra user=null yapar", async () => {
    vi.mocked(api.restore).mockResolvedValue(ali);
    const d = deferred<void>();
    vi.mocked(api.logout).mockReturnValue(d.promise);
    const box = mount();
    await act(async () => {});

    let done!: Promise<void>;
    act(() => {
      done = box.current!.logout();
    });
    expect(api.logout).toHaveBeenCalledTimes(1);
    expect(box.current!.user).toEqual(ali); // api.logout henüz bitmedi
    await act(async () => {
      d.resolve();
      await done;
    });
    expect(box.current!.user).toBeNull();
  });
});

describe("AuthProvider.login", () => {
  it("api.login'i çağırır ve dönen kullanıcıyı context'e yazar", async () => {
    vi.mocked(api.restore).mockResolvedValue(null);
    vi.mocked(api.login).mockResolvedValue(ali);
    const box = mount();
    await act(async () => {});
    expect(box.current!.user).toBeNull();
    await act(async () => box.current!.login("ali@example.com", "parola"));
    expect(api.login).toHaveBeenCalledWith("ali@example.com", "parola");
    expect(box.current!.user).toEqual(ali);
  });
});

describe("useAuth", () => {
  it("AuthProvider dışında fırlatır", () => {
    let err: unknown;
    function Probe() {
      try {
        useAuth();
      } catch (e) {
        err = e;
      }
      return null;
    }
    const root = createRoot(document.createElement("div"));
    act(() => root.render(<Probe />));
    cleanup.push(() => act(() => root.unmount()));
    expect(err).toBeInstanceOf(Error);
    expect((err as Error).message).toMatch(/AuthProvider/);
  });
});
