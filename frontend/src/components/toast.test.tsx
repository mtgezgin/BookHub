import { act } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ToastProvider, useToast } from "./toast";

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

const cleanup: (() => void)[] = [];
afterEach(() => {
  cleanup.splice(0).forEach((f) => f());
  vi.useRealTimers();
});

function Trigger({ message }: { message: string }) {
  const toast = useToast();
  return <button onClick={() => toast(message)}>tetikle</button>;
}

function render(node: React.ReactNode) {
  const el = document.createElement("div");
  document.body.appendChild(el);
  const root = createRoot(el);
  act(() => root.render(node));
  cleanup.push(() => {
    act(() => root.unmount());
    el.remove();
  });
  return el;
}

const region = (el: HTMLElement) => el.querySelector('[aria-label="Bildirimler"]')!;
const click = (el: HTMLElement, text: string) =>
  act(() => Array.from(el.querySelectorAll("button")).find((b) => b.textContent === text)!.click());

describe("toast", () => {
  it("mesaj, ekran okuyucunun sırası gelince okuduğu bölgede görünür", () => {
    const el = render(<ToastProvider><Trigger message="Puanın kaydedildi" /></ToastProvider>);
    expect(region(el).getAttribute("aria-live")).toBe("polite");
    click(el, "tetikle");
    expect(region(el).textContent).toContain("Puanın kaydedildi");
  });

  it("bölge role=status DEĞİL: sayfadaki başarı mesajlarıyla karışmasın", () => {
    const el = render(<ToastProvider><Trigger message="x" /></ToastProvider>);
    expect(el.querySelector("[role=status]")).toBeNull();
  });

  it("birkaç saniye sonra kendiliğinden kaybolur", () => {
    vi.useFakeTimers({ toFake: ["setTimeout", "clearTimeout"] });
    const el = render(<ToastProvider><Trigger message="Yorumun eklendi" /></ToastProvider>);
    click(el, "tetikle");
    act(() => vi.advanceTimersByTime(4000));
    expect(region(el).textContent).not.toContain("Yorumun eklendi");
  });

  it("\"Kapat\" ile hemen kapanır", () => {
    const el = render(<ToastProvider><Trigger message="Onaylandı" /></ToastProvider>);
    click(el, "tetikle");
    act(() => (region(el).querySelector('button[aria-label="Kapat"]') as HTMLButtonElement).click());
    expect(region(el).textContent).not.toContain("Onaylandı");
  });

  it("sağlayıcı yoksa hiçbir şey yapmaz (sayfa testleri sağlayıcı kurmak zorunda değil)", () => {
    const el = render(<Trigger message="sessiz" />);
    expect(() => click(el, "tetikle")).not.toThrow();
  });
});
