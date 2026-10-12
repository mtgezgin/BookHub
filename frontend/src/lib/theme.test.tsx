import { act } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ThemeToggle } from "../components/ThemeToggle";

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

const cleanup: (() => void)[] = [];
afterEach(() => {
  cleanup.splice(0).forEach((f) => f());
  vi.unstubAllGlobals();
  localStorage.clear();
  delete document.documentElement.dataset.theme;
});

function stubSystem(dark: boolean) {
  const listeners = new Set<() => void>();
  const mq = {
    matches: dark,
    addEventListener: (_: string, f: () => void) => listeners.add(f),
    removeEventListener: (_: string, f: () => void) => listeners.delete(f),
  };
  vi.stubGlobal("matchMedia", vi.fn(() => mq));
  return (next: boolean) => {
    mq.matches = next;
    act(() => listeners.forEach((f) => f()));
  };
}

function render() {
  const el = document.createElement("div");
  document.body.appendChild(el);
  const root = createRoot(el);
  act(() => root.render(<ThemeToggle />));
  cleanup.push(() => {
    act(() => root.unmount());
    el.remove();
  });
  return el.querySelector<HTMLButtonElement>('button[aria-label="Koyu tema"]')!;
}

describe("useTheme", () => {
  it("seçim yokken sistem koyuya geçince tema koyu olur", () => {
    const changeSystem = stubSystem(false);
    const toggle = render();
    expect(toggle.getAttribute("aria-pressed")).toBe("false");
    changeSystem(true);
    expect(document.documentElement.dataset.theme).toBe("dark");
    expect(toggle.getAttribute("aria-pressed")).toBe("true");
  });

  it("kayıtlı açık tema varken sistem değişikliği temayı değiştirmez", () => {
    localStorage.setItem("bookhub.theme", "light");
    const changeSystem = stubSystem(false);
    const toggle = render();
    changeSystem(true);
    expect(document.documentElement.dataset.theme).not.toBe("dark");
    expect(toggle.getAttribute("aria-pressed")).toBe("false");
  });

  it("açılıştan sonra kullanıcı seçim yaptıysa sistem değişikliği onu ezmez", () => {
    const changeSystem = stubSystem(false);
    const toggle = render();
    act(() => toggle.click());
    changeSystem(false);
    expect(document.documentElement.dataset.theme).toBe("dark");
    expect(toggle.getAttribute("aria-pressed")).toBe("true");
  });

  it("data-theme yokken kayıtlı koyu tema geçerli olur", () => {
    localStorage.setItem("bookhub.theme", "dark");
    stubSystem(false);
    const toggle = render();
    expect(toggle.getAttribute("aria-pressed")).toBe("true");
  });
});
