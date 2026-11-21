import { act } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { User } from "../api/types";
import { testQueryClient, withQuery } from "../test/render";

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

// API istemcisini sahtele: restore/logout'u test kontrol eder, subscribeSession kaydedilen
// fonksiyonu yakalar (istemci oturumu düşürdüğünde çağrılan kanca).
const h = vi.hoisted(() => ({ session: null as null | ((u: unknown) => void) }));
vi.mock("../api/client", () => ({
  api: { restore: vi.fn(), logout: vi.fn(), login: vi.fn(), sessionUncertain: vi.fn(() => false) },
  subscribeSession: (fn: (u: unknown) => void) => {
    h.session = fn;
  },
}));

import { api } from "../api/client";
import { AuthProvider, RESTORE_RETRY_DELAYS_MS, useAuth } from "./AuthContext";

const ali: User = { id: 7, email: "ali@example.com", display_name: "Ali", role: "USER", created_at: "" };
type State = ReturnType<typeof useAuth>;

const cleanup: (() => void)[] = [];
afterEach(() => cleanup.splice(0).forEach((f) => f()));
beforeEach(() => {
  vi.mocked(api.restore).mockReset();
  vi.mocked(api.sessionUncertain).mockReset().mockReturnValue(false);
});

function mount(client = testQueryClient()) {
  const box: { current?: State } = {};
  function Probe() {
    box.current = useAuth();
    return null;
  }
  const root = createRoot(document.createElement("div"));
  act(() => root.render(withQuery(<AuthProvider><Probe /></AuthProvider>, client)));
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

// REVIEW_FEEDBACK F36: açılışta yenileme GEÇİCİ bir hatayla dönerse (ağ, 429, 502) oturum "yok" değil
// "belirsiz"dir. ready false kalır (korumalı sayfalar giriş sayfasına yönlendirmez) ve yeniden denenir.
describe("AuthProvider — belirsiz oturum", () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it("doğrulanamayan oturum anonim sayılmaz; yeniden denenir ve oturum kendiliğinden görünür", async () => {
    vi.mocked(api.restore).mockResolvedValue(null);
    vi.mocked(api.sessionUncertain).mockReturnValue(true);
    const box = mount();
    await act(async () => {});
    expect(box.current!).toMatchObject({ user: null, ready: false, uncertain: true });
    expect(api.restore).toHaveBeenCalledTimes(1);

    // İlk yeniden deneme de başarısız: hâlâ belirsiz.
    await act(async () => vi.advanceTimersByTimeAsync(RESTORE_RETRY_DELAYS_MS[0]));
    expect(api.restore).toHaveBeenCalledTimes(2);
    expect(box.current!).toMatchObject({ ready: false, uncertain: true });

    // Sunucu geri geldi.
    vi.mocked(api.restore).mockResolvedValue(ali);
    await act(async () => vi.advanceTimersByTimeAsync(RESTORE_RETRY_DELAYS_MS[1]));
    expect(box.current!).toMatchObject({ user: ali, ready: true, uncertain: false });
  });

  it("yeniden denemede sunucu oturumu reddederse (401) anonim olarak kesinleşir", async () => {
    vi.mocked(api.restore).mockResolvedValue(null);
    vi.mocked(api.sessionUncertain).mockReturnValue(true);
    const box = mount();
    await act(async () => {});
    vi.mocked(api.sessionUncertain).mockReturnValue(false); // 401: istemci HINT'i sildi
    await act(async () => vi.advanceTimersByTimeAsync(RESTORE_RETRY_DELAYS_MS[0]));
    expect(box.current!).toMatchObject({ user: null, ready: true, uncertain: false });
  });

  it("ağ geri gelince (online) beklemeden dener", async () => {
    vi.mocked(api.restore).mockResolvedValue(null);
    vi.mocked(api.sessionUncertain).mockReturnValue(true);
    const box = mount();
    await act(async () => {});
    vi.mocked(api.restore).mockResolvedValue(ali);
    await act(async () => {
      window.dispatchEvent(new Event("online"));
    });
    expect(api.restore).toHaveBeenCalledTimes(2);
    expect(box.current!).toMatchObject({ user: ali, ready: true });
  });

  it("başka bir isteğin yenilediği oturum (subscribeSession(user)) belirsizliği bitirir", async () => {
    vi.mocked(api.restore).mockResolvedValue(null);
    vi.mocked(api.sessionUncertain).mockReturnValue(true);
    const box = mount();
    await act(async () => {});
    act(() => h.session!(ali));
    expect(box.current!).toMatchObject({ user: ali, ready: true, uncertain: false });
  });

  it("denemeler bitince zamanlayıcı kurmaz; kaldırılınca bekleyen deneme iptal olur", async () => {
    vi.mocked(api.restore).mockResolvedValue(null);
    vi.mocked(api.sessionUncertain).mockReturnValue(true);
    mount();
    await act(async () => {});
    for (const d of RESTORE_RETRY_DELAYS_MS) await act(async () => vi.advanceTimersByTimeAsync(d));
    expect(api.restore).toHaveBeenCalledTimes(1 + RESTORE_RETRY_DELAYS_MS.length);
    await act(async () => vi.advanceTimersByTimeAsync(120_000));
    expect(api.restore).toHaveBeenCalledTimes(1 + RESTORE_RETRY_DELAYS_MS.length);
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

// Karar 4 (Faz 16d adım 8): oturum kapanınca kullanıcıya özel veri önbellekte kalmaz.
describe("AuthProvider — önbellek", () => {
  const MY_LIBRARY = ["me", 7, "library", "list", { limit: 12, offset: 0 }];
  const ADMIN_QUEUE = ["admin", "books", "PENDING", 0];
  function seeded() {
    const client = testQueryClient();
    client.setQueryData(MY_LIBRARY, { items: [], total: 0, limit: 12, offset: 0 });
    client.setQueryData(ADMIN_QUEUE, { items: [], total: 0, limit: 20, offset: 0 });
    return client;
  }
  async function signedIn(client: ReturnType<typeof testQueryClient>) {
    vi.mocked(api.restore).mockResolvedValue(ali);
    const box = mount(client);
    await act(async () => {});
    expect(box.current!.user?.id).toBe(7);
    return box;
  }

  it("çıkış yapınca önbellek boşalır", async () => {
    const client = seeded();
    const box = await signedIn(client);
    vi.mocked(api.logout).mockResolvedValue(undefined);
    await act(async () => box.current!.logout());
    expect(box.current!.user).toBeNull();
    expect(client.getQueryCache().getAll()).toEqual([]);
  });

  it("oturum dışarıdan düşünce (başka sekmede çıkış, yenileme reddi) de önbellek boşalır", async () => {
    const client = seeded();
    const box = await signedIn(client);
    await act(async () => h.session!(null));
    expect(box.current!.user).toBeNull();
    expect(client.getQueryData(MY_LIBRARY)).toBeUndefined();
    expect(client.getQueryData(ADMIN_QUEUE)).toBeUndefined();
  });

  it("oturum yenilenince (aynı kullanıcı, yeni token) önbellek kalır", async () => {
    const client = seeded();
    await signedIn(client);
    await act(async () => h.session!({ ...ali }));
    expect(client.getQueryData(MY_LIBRARY)).toBeDefined();
  });
});
