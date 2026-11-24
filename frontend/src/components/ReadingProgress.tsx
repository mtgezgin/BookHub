import type { Reading } from "../api/types";
import { progressPercent, progressText } from "../lib/shelf";
import { cn } from "../lib/utils";

/**
 * Süren okumanın ilerlemesi: oran biliniyorsa çubuk (role="progressbar"; ekran okuyucu metni de okur,
 * "42 / 310 sayfa"), altında metin. Toplamı bilinmeyen sayfada çubuk yok, neyin yüzdesi olduğu bilinmiyor.
 * Kitap sayfasındaki raf kontrolü ve Kitaplığım kartları (Faz 17A-1 adım 6) aynı bileşeni kullanır.
 *
 * label: çubuğun erişilebilir adı. Kitaplığım'da her kartta bir çubuk var; ad kitabı da söylemeli, yoksa
 * ekran okuyucu on iki aynı "Okuma ilerlemen" okur.
 * empty: ilerleme girilmemişken yazılacak metin; verilmezse hiçbir şey çizilmez (kartlarda gürültü olmasın).
 */
export function ReadingProgress({ reading, label, empty, className }: {
  reading: Pick<Reading, "unit" | "progress" | "total_pages">;
  label: string;
  empty?: string;
  className?: string;
}) {
  const text = progressText(reading);
  const percent = progressPercent(reading);
  if (text === null && empty === undefined) return null;
  return (
    <div className={cn("flex max-w-sm flex-col gap-1.5", className)}>
      {percent !== null && (
        <div
          role="progressbar"
          aria-label={label}
          aria-valuemin={0}
          aria-valuemax={100}
          aria-valuenow={percent}
          aria-valuetext={text ?? undefined}
          className="h-2 overflow-hidden rounded-full bg-muted"
        >
          <div className="h-full rounded-full bg-primary transition-[width]" style={{ width: `${percent}%` }} />
        </div>
      )}
      <p className="text-sm tabular-nums text-muted-foreground">{text ?? empty}</p>
    </div>
  );
}
