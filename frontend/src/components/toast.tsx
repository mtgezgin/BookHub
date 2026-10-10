import { CircleCheck, CircleAlert, X } from "lucide-react";
import { createContext, useCallback, useContext, useEffect, useRef, useState, type ReactNode } from "react";
import { cn } from "../lib/utils";

type Variant = "success" | "error";
type Toast = { id: number; message: string; variant: Variant };
type Notify = (message: string, variant?: Variant) => void;

const DURATION_MS = 4000;
const NOOP: Notify = () => {};
const ToastContext = createContext<Notify>(NOOP);

/** Kısa onay bildirimi. Sağlayıcı yoksa (ör. tek bir sayfanın testi) hiçbir şey yapmaz. */
export function useToast(): Notify {
  return useContext(ToastContext);
}

/**
 * Bildirim bölgesi: aria-live="polite" → ekran okuyucu sırası gelince okur. role="status"
 * BİLEREK yok: sayfadaki başarı mesajları (Alert role="status") ile aynı rolü taşısaydı
 * `getByRole("status")` iki öğe bulurdu.
 */
export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([]);
  const nextId = useRef(0);
  const timers = useRef(new Map<number, ReturnType<typeof setTimeout>>());

  const dismiss = useCallback((id: number) => {
    clearTimeout(timers.current.get(id));
    timers.current.delete(id);
    setToasts((all) => all.filter((t) => t.id !== id));
  }, []);

  const notify = useCallback<Notify>(
    (message, variant = "success") => {
      const id = ++nextId.current;
      setToasts((all) => [...all.slice(-2), { id, message, variant }]); // en çok 3 tane
      timers.current.set(id, setTimeout(() => dismiss(id), DURATION_MS));
    },
    [dismiss],
  );

  useEffect(() => {
    const pending = timers.current;
    return () => pending.forEach(clearTimeout);
  }, []);

  return (
    <ToastContext.Provider value={notify}>
      {children}
      <section
        aria-label="Bildirimler"
        aria-live="polite"
        className="pointer-events-none fixed inset-x-0 bottom-4 z-50 flex flex-col items-center gap-2 px-4 sm:right-4 sm:left-auto sm:items-end"
      >
        {toasts.map((t) => {
          const Icon = t.variant === "error" ? CircleAlert : CircleCheck;
          return (
            <div
              key={t.id}
              className={cn(
                "pointer-events-auto flex w-full max-w-sm items-center gap-3 rounded-lg border bg-card px-4 py-3 text-sm shadow-lg",
                t.variant === "error" ? "text-destructive" : "text-card-foreground",
              )}
            >
              <Icon aria-hidden="true" className={cn("size-4 shrink-0", t.variant === "error" ? "text-destructive" : "text-success")} />
              <span className="flex-1">{t.message}</span>
              <button type="button" aria-label="Kapat" className="rounded-sm text-muted-foreground hover:text-foreground" onClick={() => dismiss(t.id)}>
                <X aria-hidden="true" className="size-4" />
              </button>
            </div>
          );
        })}
      </section>
    </ToastContext.Provider>
  );
}
