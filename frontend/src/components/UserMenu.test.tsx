import { act } from "react";
import { createRoot } from "react-dom/client";
import { MemoryRouter } from "react-router-dom";
import { afterEach, describe, expect, it } from "vitest";
import type { User } from "../api/types";
import { UserMenu } from "./UserMenu";

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

const cleanup: (() => void)[] = [];
afterEach(() => cleanup.splice(0).forEach((f) => f()));

const user: User = { id: 1, email: "ayse@example.com", display_name: "Ayşe", role: "USER", created_at: "2026-01-01T00:00:00Z" };

function renderOpen(u: User, path = "/") {
  const el = document.createElement("div");
  document.body.appendChild(el);
  const root = createRoot(el);
  act(() =>
    root.render(
      <MemoryRouter initialEntries={[path]}>
        <UserMenu user={u} onLogout={() => {}} />
      </MemoryRouter>,
    ),
  );
  cleanup.push(() => {
    act(() => root.unmount());
    el.remove();
  });
  const button = el.querySelector<HTMLButtonElement>("button[aria-expanded]")!;
  act(() => button.click());
  return { el, button };
}

const pointerDown = (target: Element) => act(() => void target.dispatchEvent(new Event("pointerdown", { bubbles: true })));

describe("UserMenu", () => {
  it("dışarıda pointerdown menüyü kapatır", () => {
    const { button } = renderOpen(user);
    expect(button.getAttribute("aria-expanded")).toBe("true");
    pointerDown(document.body);
    expect(button.getAttribute("aria-expanded")).toBe("false");
  });

  it("panelin içinde pointerdown menüyü açık tutar", () => {
    const { el, button } = renderOpen(user);
    const panel = document.getElementById(button.getAttribute("aria-controls")!)!;
    pointerDown(panel.querySelector("p")!);
    expect(button.getAttribute("aria-expanded")).toBe("true");
    expect(el.contains(panel)).toBe(true);
  });

  it("USER için panelde \"Kitap öner\" var, \"Moderasyon\" yok", () => {
    const { button } = renderOpen(user);
    const panel = document.getElementById(button.getAttribute("aria-controls")!)!;
    const links = Array.from(panel.querySelectorAll("a")).map((a) => a.textContent);
    expect(links).toContain("Kitap öner");
    expect(links).not.toContain("Moderasyon");
  });

  it("bulunulan sayfanın bağlantısına tıklamak da menüyü kapatır (adres değişmese bile)", () => {
    const { button } = renderOpen(user, "/account");
    const panel = document.getElementById(button.getAttribute("aria-controls")!)!;
    const link = Array.from(panel.querySelectorAll("a")).find((a) => a.textContent === "Hesabım")!;
    act(() => void link.dispatchEvent(new MouseEvent("click", { bubbles: true, cancelable: true, button: 0 })));
    expect(button.getAttribute("aria-expanded")).toBe("false");
  });

  it("listede bağlantı dışında bir yere tıklamak menüyü kapatmaz", () => {
    const { button } = renderOpen(user);
    const panel = document.getElementById(button.getAttribute("aria-controls")!)!;
    act(() => void panel.querySelector("ul")!.dispatchEvent(new MouseEvent("click", { bubbles: true })));
    expect(button.getAttribute("aria-expanded")).toBe("true");
  });
});
