import { ChevronDown } from "lucide-react";
import { useEffect, useId, useRef, useState } from "react";
import { Link, useLocation } from "react-router-dom";
import type { User } from "../api/types";
import { initialOf } from "../lib/format";
import { cn } from "../lib/utils";
import { Button } from "./ui/button";

const ITEM =
  "flex w-full items-center rounded-md px-3 py-2 text-left text-sm outline-none hover:bg-accent hover:text-accent-foreground focus-visible:ring-[3px] focus-visible:ring-ring/50";

/**
 * Kullanıcı menüsü — "disclosure" kalıbı (düğme + aria-expanded + açılır panel). Site gezintisi
 * için ARIA `menu` rolünden doğru: içerik sıradan bağlantılar, Tab ile gezilir.
 * Dar ekranda üst çubuğa sığmayan "Kitap öner" ve "Moderasyon" da burada (md'den itibaren gizli).
 * Kapanır: Esc (odak düğmeye döner), dışarı tıklama, sayfa değişimi, bir öğeye tıklama (zaten
 * açık olan sayfanın bağlantısı adresi değiştirmez).
 */
export function UserMenu({ user, onLogout }: { user: User; onLogout: () => void }) {
  const [open, setOpen] = useState(false);
  const box = useRef<HTMLDivElement>(null);
  const button = useRef<HTMLButtonElement>(null);
  const panelId = useId();
  const { pathname } = useLocation();

  useEffect(() => setOpen(false), [pathname]);

  useEffect(() => {
    if (!open) return;
    const onPointer = (e: PointerEvent) => {
      if (!box.current?.contains(e.target as Node)) setOpen(false);
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== "Escape") return;
      setOpen(false);
      button.current?.focus();
    };
    document.addEventListener("pointerdown", onPointer);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("pointerdown", onPointer);
      document.removeEventListener("keydown", onKey);
    };
  }, [open]);

  return (
    <div ref={box} className="relative">
      <Button
        ref={button}
        variant="ghost"
        size="sm"
        aria-label={`Hesap menüsü: ${user.display_name}`}
        aria-expanded={open}
        aria-controls={panelId}
        className="gap-2 px-1.5"
        onClick={() => setOpen((o) => !o)}
      >
        <span aria-hidden="true" className="grid size-7 place-items-center rounded-full bg-accent font-serif text-sm font-semibold text-accent-foreground">
          {initialOf(user.display_name)}
        </span>
        <span className="hidden max-w-32 truncate sm:inline">{user.display_name}</span>
        <ChevronDown aria-hidden="true" className={cn("transition-transform", open && "rotate-180")} />
      </Button>
      {open && (
        <div id={panelId} className="absolute top-full right-0 z-30 mt-2 w-56 rounded-lg border bg-card p-1 text-card-foreground shadow-lg">
          <p className="truncate px-3 py-2 text-xs text-muted-foreground">{user.email}</p>
          <ul className="flex flex-col" onClick={() => setOpen(false)}>
            <li className="md:hidden"><Link to="/suggest" className={ITEM}>Kitap öner</Link></li>
            {user.role === "ADMIN" && <li className="md:hidden"><Link to="/admin" className={ITEM}>Moderasyon</Link></li>}
            <li><Link to="/account" className={ITEM}>Hesabım</Link></li>
            <li className="mt-1 border-t pt-1"><button type="button" className={ITEM} onClick={onLogout}>Çıkış</button></li>
          </ul>
        </div>
      )}
    </div>
  );
}
