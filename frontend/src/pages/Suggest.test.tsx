import { act } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

vi.mock("../api/client", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../api/client")>()),
  api: {},
}));

import { api, ApiError } from "../api/client";
import { Suggest } from "./Suggest";

let sent: unknown[] = [];
let result: () => Promise<unknown>;

const cleanup: (() => void)[] = [];
afterEach(() => cleanup.splice(0).forEach((f) => f()));
beforeEach(() => {
  sent = [];
  (api as unknown as Record<string, unknown>).suggest = (b: unknown) => {
    sent.push(b);
    return result();
  };
});

async function fillAndSubmit(values: { title: string; author: string; year: string; description: string }) {
  const el = document.createElement("div");
  document.body.appendChild(el);
  const root = createRoot(el);
  act(() => root.render(<Suggest />));
  cleanup.push(() => {
    act(() => root.unmount());
    el.remove();
  });
  const [title, author, year] = Array.from(el.querySelectorAll("input"));
  const description = el.querySelector("textarea")!;
  const pairs: [HTMLInputElement | HTMLTextAreaElement, string][] = [
    [title, values.title], [author, values.author], [year, values.year], [description, values.description],
  ];
  for (const [input, v] of pairs) {
    act(() => {
      Object.getOwnPropertyDescriptor(Object.getPrototypeOf(input), "value")!.set!.call(input, v);
      input.dispatchEvent(new Event("input", { bubbles: true }));
    });
  }
  await act(async () => {
    el.querySelector("form")!.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
  });
  await act(async () => {});
  return { el, inputs: [title, author, year, description] };
}

describe("Suggest", () => {
  it("boş açıklama ve yıl undefined gider", async () => {
    result = async () => ({ title: "Kürk Mantolu Madonna", status: "PENDING" });
    await fillAndSubmit({ title: "Kürk Mantolu Madonna", author: "Sabahattin Ali", year: "", description: "" });
    expect(sent).toEqual([{ title: "Kürk Mantolu Madonna", author: "Sabahattin Ali", description: undefined, publication_year: undefined }]);
  });

  it("yıl sayı olarak gider; başarıda \"incelemeye gönderildi\" mesajı çıkar ve form sıfırlanır", async () => {
    result = async () => ({ title: "Tutunamayanlar", status: "PENDING" });
    const { el, inputs } = await fillAndSubmit({ title: "Tutunamayanlar", author: "Oğuz Atay", year: "1972", description: "roman" });
    expect(sent).toEqual([{ title: "Tutunamayanlar", author: "Oğuz Atay", description: "roman", publication_year: 1972 }]);
    expect(el.querySelector("[role=status]")!.textContent).toContain("\"Tutunamayanlar\" incelemeye gönderildi");
    // İç durum adı (PENDING) kullanıcıya gösterilmez.
    expect(el.querySelector("[role=status]")!.textContent).not.toContain("PENDING");
    expect(inputs.map((i) => i.value)).toEqual(["", "", "", ""]);
  });

  it("hata gösterilir", async () => {
    result = () => Promise.reject(new ApiError(409, "slug_conflict", "bu slug zaten kullanılıyor"));
    const { el } = await fillAndSubmit({ title: "T", author: "A", year: "", description: "" });
    expect(el.querySelector("[role=alert]")!.textContent).toBe("bu slug zaten kullanılıyor");
    expect(el.querySelector("[role=status]")).toBeNull();
  });
});
