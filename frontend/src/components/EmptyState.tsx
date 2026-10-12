import type { LucideIcon } from "lucide-react";
import type { ReactNode } from "react";
import { cn } from "../lib/utils";

/**
 * "Burada bir şey yok" ekranı: ne olduğunu söyler ve bir sonraki adımı (action) önerir.
 * Çıplak "Sonuç yok." yazısı kullanıcıyı çıkmazda bırakıyordu.
 */
export function EmptyState({ icon: Icon, title, titleAs: Title = "h2", children, action, className }: {
  icon: LucideIcon;
  title: string;
  titleAs?: "h1" | "h2";
  children?: ReactNode;
  action?: ReactNode;
  className?: string;
}) {
  return (
    <div className={cn("flex flex-col items-center gap-3 rounded-xl border border-dashed px-6 py-12 text-center", className)}>
      <div className="grid size-12 place-items-center rounded-full bg-accent text-accent-foreground">
        <Icon aria-hidden="true" className="size-6" />
      </div>
      <Title className="text-xl font-semibold">{title}</Title>
      {children && <p className="max-w-sm text-sm text-muted-foreground">{children}</p>}
      {action && <div className="mt-2">{action}</div>}
    </div>
  );
}
