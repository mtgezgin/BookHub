import { describe, expect, it, vi } from "vitest";
import { reloadOnceForStaleChunk } from "./chunkReload";

function memoryStorage(): Pick<Storage, "getItem" | "setItem"> {
  const data = new Map<string, string>();
  return { getItem: (k) => data.get(k) ?? null, setItem: (k, v) => void data.set(k, v) };
}

describe("reloadOnceForStaleChunk", () => {
  it("ilk hatada sayfayı yeniden yükler", () => {
    const reload = vi.fn();
    expect(reloadOnceForStaleChunk(memoryStorage(), 1_000_000, reload)).toBe(true);
    expect(reload).toHaveBeenCalledTimes(1);
  });

  it("10 sn içinde ikinci hata yeniden yüklemez (döngü yok)", () => {
    const storage = memoryStorage();
    const reload = vi.fn();
    reloadOnceForStaleChunk(storage, 1_000_000, reload);
    expect(reloadOnceForStaleChunk(storage, 1_009_999, reload)).toBe(false);
    expect(reload).toHaveBeenCalledTimes(1);
  });

  it("10 sn sonra yine yeniden yükleyebilir", () => {
    const storage = memoryStorage();
    const reload = vi.fn();
    reloadOnceForStaleChunk(storage, 1_000_000, reload);
    expect(reloadOnceForStaleChunk(storage, 1_010_000, reload)).toBe(true);
    expect(reload).toHaveBeenCalledTimes(2);
  });

  it("depolama kullanılamıyorsa yeniden yüklemez (döngü riski)", () => {
    const reload = vi.fn();
    const broken = { getItem: () => { throw new Error("kapalı"); }, setItem: () => {} };
    expect(reloadOnceForStaleChunk(broken, 1_000_000, reload)).toBe(false);
    expect(reload).not.toHaveBeenCalled();
  });
});
