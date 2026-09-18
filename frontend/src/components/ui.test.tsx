import { act } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, describe, expect, it } from "vitest";
import { ApiError } from "../api/client";
import { ErrorBox, Rating } from "./ui";

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

const cleanup: (() => void)[] = [];
afterEach(() => cleanup.splice(0).forEach((f) => f()));

function render(node: React.ReactNode) {
  const el = document.createElement("div");
  const root = createRoot(el);
  act(() => root.render(node));
  cleanup.push(() => act(() => root.unmount()));
  return el;
}

describe("ErrorBox", () => {
  it("ApiError olmayan hatada genel metni gösterir, ham mesajı ASLA göstermez", () => {
    const el = render(<ErrorBox error={new Error("pq: relation users does not exist")} />);
    expect(el.textContent).toBe("Beklenmeyen bir hata oluştu.");
    expect(el.textContent).not.toContain("pq:");
  });

  it("null hiçbir şey çizmez", () => {
    expect(render(<ErrorBox error={null} />).innerHTML).toBe("");
  });

  it("ApiError mesajını gösterir", () => {
    expect(render(<ErrorBox error={new ApiError(404, "not_found", "kitap bulunamadı")} />).textContent).toBe("kitap bulunamadı");
  });
});

describe("Rating", () => {
  it("null değer \"Henüz puan yok\" çizer", () => {
    expect(render(<Rating value={null} count={0} />).textContent).toBe("Henüz puan yok");
  });

  it("sayı ★ x.xx (n oy) biçiminde çizilir", () => {
    expect(render(<Rating value={7.5} count={12} />).textContent).toBe("★ 7.50 (12 oy)");
  });
});
