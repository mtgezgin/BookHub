import { act } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ErrorBoundary } from "./ErrorBoundary";

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

function Boom({ fail }: { fail: boolean }): React.ReactNode {
  if (fail) throw new Error("iç ayrıntı: /srv/app/secret");
  return <p>içerik</p>;
}

// REVIEW_FEEDBACK F34: render sırasında fırlayan hata bütün kökü söküp boş sayfa bırakıyordu.
describe("ErrorBoundary", () => {
  const roots: ReturnType<typeof createRoot>[] = [];
  afterEach(() => {
    roots.splice(0).forEach((r) => act(() => r.unmount()));
    vi.restoreAllMocks();
  });

  function mount(node: React.ReactNode) {
    const el = document.createElement("div");
    const root = createRoot(el);
    roots.push(root);
    act(() => root.render(node));
    return { el, root };
  }

  it("hata yokken çocukları olduğu gibi çizer", () => {
    const { el } = mount(<ErrorBoundary><Boom fail={false} /></ErrorBoundary>);
    expect(el.textContent).toBe("içerik");
  });

  it("render hatasında başlık ve yeniden yükleme seçeneği gösterir; hata ayrıntısını göstermez", () => {
    const log = vi.spyOn(console, "error").mockImplementation(() => {});
    const { el } = mount(
      <div>
        <nav>menü</nav>
        <ErrorBoundary><Boom fail /></ErrorBoundary>
      </div>,
    );
    expect(el.querySelector("nav")?.textContent).toBe("menü"); // sınırın dışı yerinde durur
    expect(el.querySelector("[role=alert] h1")?.textContent).toBe("Bir şeyler ters gitti");
    expect([...el.querySelectorAll("button, a")].map((b) => b.textContent)).toEqual(["Sayfayı yeniden yükle", "Ana sayfaya dön"]);
    expect(el.textContent).not.toContain("secret");
    expect(log).toHaveBeenCalled();
  });

  it("key değişince (başka sayfaya geçiş) sınır sıfırlanır", () => {
    vi.spyOn(console, "error").mockImplementation(() => {});
    const { el, root } = mount(<ErrorBoundary key="/a"><Boom fail /></ErrorBoundary>);
    expect(el.querySelector("[role=alert]")).not.toBeNull();
    act(() => root.render(<ErrorBoundary key="/b"><Boom fail={false} /></ErrorBoundary>));
    expect(el.textContent).toBe("içerik");
  });
});
