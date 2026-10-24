import { act } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { LibraryStatus, User } from "../api/types";

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

vi.mock("../api/client", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../api/client")>()),
  api: {},
}));
const h = vi.hoisted(() => ({ user: null as unknown }));
vi.mock("../auth/AuthContext", () => ({ useAuth: () => ({ user: h.user, ready: true }) }));

import { api } from "../api/client";
import { useLibraryStatuses } from "./useLibraryStatuses";

const USER: User = { id: 7, email: "ali@example.com", display_name: "Ali", role: "USER", created_at: "" };
type Reply = { statuses: Record<string, LibraryStatus> };
let calls: number[][] = [];
let replies: ((r: Reply) => void)[] = [];

beforeEach(() => {
  calls = [];
  replies = [];
  h.user = USER;
  (api as unknown as Record<string, unknown>).libraryStatuses = (ids: number[]) => {
    calls.push(ids);
    return new Promise<Reply>((r) => replies.push(r));
  };
});
const cleanup: (() => void)[] = [];
afterEach(() => cleanup.splice(0).forEach((f) => f()));

function Probe({ ids }: { ids: number[] }) {
  const statuses = useLibraryStatuses(ids);
  return <p>{ids.map((id) => `${id}:${statuses[id] ?? "-"}`).join(" ")}</p>;
}

function mount(ids: number[]) {
  const el = document.createElement("div");
  document.body.appendChild(el);
  const root = createRoot(el);
  act(() => root.render(<Probe ids={ids} />));
  cleanup.push(() => {
    act(() => root.unmount());
    el.remove();
  });
  return { el, rerender: (next: number[]) => act(() => root.render(<Probe ids={next} />)) };
}

describe("useLibraryStatuses", () => {
  it("giriş yoksa istek yok", () => {
    h.user = null;
    mount([3, 4]);
    expect(calls).toEqual([]);
  });

  it("kimlik listesi boşsa istek yok", () => {
    mount([]);
    expect(calls).toEqual([]);
  });

  it("sayfadaki kimlikler için TEK istek; tekrarlar bir kez gider", async () => {
    const { el } = mount([4, 3, 4]);
    expect(calls).toEqual([[3, 4]]);
    await act(async () => replies[0]({ statuses: { "3": "READ" } }));
    expect(el.textContent).toBe("4:- 3:READ 4:-");
  });

  it("token yenilenince (aynı kullanıcı, yeni nesne) yeniden sormaz", () => {
    const { rerender } = mount([3]);
    h.user = { ...USER };
    rerender([3]);
    expect(calls).toEqual([[3]]);
  });

  it("kimlikler değişince yeni istek; bayat yanıt yok sayılır", async () => {
    const { el, rerender } = mount([3]);
    rerender([5]);
    expect(calls).toEqual([[3], [5]]);
    await act(async () => replies[1]({ statuses: { "5": "READING" } }));
    await act(async () => replies[0]({ statuses: { "3": "READ" } }));
    expect(el.textContent).toBe("5:READING");
  });

  it("okuma başarısızsa rozet yok, yakalanmamış ret yok", async () => {
    const unhandled = vi.fn();
    process.on("unhandledRejection", unhandled);
    cleanup.push(() => process.off("unhandledRejection", unhandled));
    (api as unknown as Record<string, unknown>).libraryStatuses = () => Promise.reject(new Error("db"));
    const { el } = mount([3]);
    await act(async () => {});
    await new Promise((r) => setTimeout(r, 0));
    expect(el.textContent).toBe("3:-");
    expect(unhandled).not.toHaveBeenCalled();
  });

  it("yeni kimliklerin okuması başarısızsa önceki rozetler kalmaz", async () => {
    const { el, rerender } = mount([3]);
    await act(async () => replies[0]({ statuses: { "3": "READ" } }));
    (api as unknown as Record<string, unknown>).libraryStatuses = () => Promise.reject(new Error("db"));
    rerender([3, 5]);
    await act(async () => {});
    expect(el.textContent).toBe("3:- 5:-");
  });

  it("kullanıcı değişince önceki kullanıcının durumları gösterilmez", async () => {
    const { el, rerender } = mount([3]);
    await act(async () => replies[0]({ statuses: { "3": "READ" } }));
    h.user = { ...USER, id: 8 };
    rerender([3]);
    expect(calls).toEqual([[3], [3]]);
    expect(el.textContent).toBe("3:-");
  });

  it("çıkış yapılınca durumlar temizlenir", async () => {
    const { el, rerender } = mount([3]);
    await act(async () => replies[0]({ statuses: { "3": "READ" } }));
    expect(el.textContent).toBe("3:READ");
    h.user = null;
    rerender([3]);
    expect(el.textContent).toBe("3:-");
  });
});
