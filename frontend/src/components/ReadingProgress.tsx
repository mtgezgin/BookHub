import type { Reading } from "../api/types";
import { progressPercent, progressText } from "../lib/shelf";
import { cn } from "../lib/utils";

/**
 * Süren okumanın ilerlemesi: oran biliniyorsa çubuk (yerleşik <progress>, rolü kendiliğinden progressbar;
 * ekran okuyucu yüzde yerine metni okur, "42 / 310 sayfa"), altında metin. Toplamı bilinmeyen sayfada çubuk yok, neyin yüzdesi olduğu bilinmiyor.
 * Kitap sayfasındaki raf kontrolü ve Kitaplığım kartları (Faz 17A-1 adım 6) aynı bileşeni kullanır.
 *
 * label: çubuğun erişilebilir adı. Kitaplığım'da her kartta bir çubuk var; ad kitabı da söylemeli, yoksa
 * ekran okuyucu on iki aynı "Okuma ilerlemen" okur.
 * empty: ilerleme girilmemişken yazılacak metin; verilmezse hiçbir şey çizilmez (kartlarda gürültü olmasın).
 *
 * Neden <progress>, elle ARIA'lı <div> değil (Sonar S6819): değer, en büyük değer ve rol tarayıcıdan gelir,
 * aria-valuenow'u elde tutmak gerekmez. Görünüm motorların sözde öğeleriyle: Chromium/Safari
 * ::-webkit-progress-bar (zemin) ve ::-webkit-progress-value (dolu kısım), Firefox ::-moz-progress-bar.
 */
export function ReadingProgress({ reading, label, empty, className }: Readonly<{
  reading: Pick<Reading, "unit" | "progress" | "total_pages">;
  label: string;
  empty?: string;
  className?: string;
}>) {
  const text = progressText(reading);
  const percent = progressPercent(reading);
  if (text === null && empty === undefined) return null;
  return (
    <div className={cn("flex max-w-sm flex-col gap-1.5", className)}>
      {percent !== null && (
        <progress
          aria-label={label}
          aria-valuetext={text ?? undefined}
          max={100}
          value={percent}
          className={cn(
            "block h-2 w-full appearance-none overflow-hidden rounded-full border-0 bg-muted",
            "[&::-webkit-progress-bar]:bg-muted",
            "[&::-webkit-progress-value]:rounded-full [&::-webkit-progress-value]:bg-primary [&::-webkit-progress-value]:transition-[width]",
            "[&::-moz-progress-bar]:rounded-full [&::-moz-progress-bar]:bg-primary",
          )}
        />
      )}
      <p className="text-sm tabular-nums text-muted-foreground">{text ?? empty}</p>
    </div>
  );
}
