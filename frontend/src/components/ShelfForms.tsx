import { useState } from "react";
import { ApiError } from "../api/client";
import type { useUpdateReading } from "../api/libraryQueries";
import type { Edition, ProgressUnit, Reading, ReadingPatch, ShelfEntry } from "../api/types";
import { today } from "../lib/day";
import { editionLabel, hasProgress, progressText } from "../lib/shelf";
import { useToast } from "./toast";
import { Button } from "./ui/button";
import { Input } from "./ui/input";
import { Label } from "./ui/label";

/*
 * Okurken düzeltme formları (Faz 17A-1 adım 5): ilerleme ve "Okumayı düzenle" (baskı, birim, başlangıç
 * günü). İkisi de PATCH …/shelf/reading gönderir ve sürüm ister; 409 akışı ortak (useReadingSave).
 */

type ReadingMutation = ReturnType<typeof useUpdateReading>;

interface FormProps {
  entry: ShelfEntry;
  reading: Reading;
  editions: Edition[];
  mutation: ReadingMutation;
  onDone: () => void;
}

/**
 * Kaydetme ve 409 akışı (spec 17A § 5 "409 version_conflict"):
 *  - Gönderilen sürüm o anki raftan (entry.version). Eskiyse sunucu 409 döner, kanca rafı yeniden okur ve
 *    hatayı ANCAK raf geldikten sonra yayar (api/libraryQueries.ts). Form açık kalır, kullanıcının girdiği
 *    değer korunur; üstte yeni değer görünür ("İlerlemen başka bir yerde güncellenmiş: sayfa 57").
 *  - Kullanıcı karar verir: "Benimkini kaydet" aynı değeri YENİ sürümle gönderir (bileşen yeni rafla
 *    yeniden çizildiği için entry.version artık güncel), "Vazgeç" formu kapatır. Sessiz yeniden deneme yok:
 *    öteki sekmede yazılanı kullanıcı görmeden ezmek, sürümün önlemek için var olduğu kayıp güncellemedir.
 *  - 404 (okuma başka yerde kapanmış) bildirimle söylenir: raf yeniden okunur, durum değişir ve form zaten
 *    kaybolur. Öteki hatalar (400) formun içinde.
 */
function useReadingSave({ entry, mutation, onDone }: Pick<FormProps, "entry" | "mutation" | "onDone">, success: string) {
  const toast = useToast();
  const [conflict, setConflict] = useState(false);
  const [error, setError] = useState<string | null>(null);

  function save(patch: Omit<ReadingPatch, "version">) {
    if (mutation.isPending) return;
    setError(null);
    mutation.mutate({ version: entry.version, ...patch }, {
      onSuccess: () => {
        toast(success);
        onDone();
      },
      onError: (err) => {
        if (err instanceof ApiError && err.code === "version_conflict") setConflict(true);
        else if (err instanceof ApiError && err.status === 404) toast(err.message, "error");
        else setError(err instanceof ApiError ? err.message : "Kaydedilemedi");
      },
    });
  }
  return { save, conflict, error, setError };
}

const fieldClass =
  "h-9 w-full min-w-0 rounded-md border border-input bg-card px-3 text-base shadow-xs outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/30 md:text-sm";

/**
 * İlerleme. Birim seçimi yalnız ilk girişte (Faz 15'ten kalan, birimsiz "Şu an okuyorum"): sonra birim
 * "Okumayı düzenle"den değişir, çünkü değişmesi ilerlemeyi sıfırlar ve bunu orada söylüyoruz.
 */
export function ProgressForm({ entry, reading, editions, mutation, onDone }: FormProps) {
  const firstEntry = reading.unit === null;
  const primary = editions.find((e) => e.is_primary);
  const [unit, setUnit] = useState<ProgressUnit>(reading.unit ?? (primary?.page_count ? "page" : "percent"));
  const [value, setValue] = useState(reading.progress ? String(reading.progress) : "");
  const { save, conflict, error, setError } = useReadingSave({ entry, mutation, onDone }, "İlerlemen kaydedildi");

  // Üst sınır: yüzde 100; sayfada okumanın sayfa toplamı (ilk girişte varsayılan baskınınki).
  const total = firstEntry ? (primary?.page_count ?? null) : reading.total_pages;
  const max = unit === "percent" ? 100 : (total ?? undefined);

  function submit(e: React.FormEvent) {
    e.preventDefault();
    const n = Number(value);
    if (value.trim() === "" || !Number.isInteger(n) || n < 0 || (max !== undefined && n > max)) {
      setError(max !== undefined ? `0 ile ${max} arasında bir tam sayı gir.` : "0 ya da daha büyük bir tam sayı gir.");
      return;
    }
    save(firstEntry ? { progress: n, unit } : { progress: n });
  }

  const now = progressText(reading);
  return (
    // noValidate: tarayıcının kendi balonu (max, required) formu bizim denetimimizden ÖNCE durdurur, dili
    // tarayıcınınkidir ve ekran okuyucuya her zaman ulaşmaz; sınırı submit denetler, mesaj role="alert".
    <form aria-label="İlerleme" noValidate onSubmit={submit} className="flex max-w-sm flex-col gap-3 rounded-lg border p-4">
      {conflict && (
        <p role="alert" className="text-sm">
          İlerlemen başka bir yerde güncellenmiş: <strong>{now ?? "ilerleme girilmemiş"}</strong>. Girdiğin değer
          aşağıda duruyor; kaydedersen onun yerine geçer.
        </p>
      )}
      {firstEntry && (
        <UnitChoice value={unit} onChange={setUnit} />
      )}
      <Label>
        {unit === "percent" ? "Yüzde kaçtasın?" : "Kaçıncı sayfadasın?"}
        <span className="flex items-center gap-2">
          <Input
            type="number"
            inputMode="numeric"
            min={0}
            max={max}
            value={value}
            onChange={(e) => setValue(e.target.value)}
            aria-invalid={error !== null}
            className="w-28"
            required
          />
          {unit === "page" && total !== null && <span className="text-muted-foreground tabular-nums">/ {total}</span>}
        </span>
      </Label>
      {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
      <div className="flex gap-2">
        <Button type="submit" disabled={mutation.isPending}>{conflict ? "Benimkini kaydet" : "Kaydet"}</Button>
        <Button type="button" variant="outline" onClick={onDone}>Vazgeç</Button>
      </div>
    </form>
  );
}

/**
 * Okumayı düzenle: baskı, birim, başlangıç günü. Yalnız DEĞİŞEN alanlar gönderilir (PATCH: gelmeyen alan
 * değişmez). Baskı ya da birim değişirse sunucu ilerlemeyi sıfırlar (sayfa ↔ yüzde dönüşümü yok, spec §
 * Kararlar "İlerleme birimi"); ilerleme varken bu, kaydetmeden önce yazılır.
 */
export function ReadingEditForm({ entry, reading, editions, mutation, onDone }: FormProps) {
  const [editionId, setEditionId] = useState<number | null>(reading.edition_id);
  const [unit, setUnit] = useState<ProgressUnit | null>(reading.unit);
  const [startedOn, setStartedOn] = useState(reading.started_on ?? "");
  const { save, conflict, error } = useReadingSave({ entry, mutation, onDone }, "Okuman güncellendi");

  const editionChanged = editionId !== reading.edition_id;
  const unitChanged = unit !== reading.unit;
  const resets = (editionChanged || unitChanged) && hasProgress(reading);

  function submit(e: React.FormEvent) {
    e.preventDefault();
    const patch: Omit<ReadingPatch, "version"> = {};
    if (editionChanged && editionId !== null) patch.edition_id = editionId;
    if (unitChanged && unit !== null) patch.unit = unit;
    if (startedOn !== "" && startedOn !== reading.started_on) patch.started_on = startedOn;
    if (Object.keys(patch).length === 0) {
      onDone();
      return;
    }
    save(patch);
  }

  return (
    <form aria-label="Okumayı düzenle" onSubmit={submit} className="flex max-w-sm flex-col gap-3 rounded-lg border p-4">
      {conflict && (
        <p role="alert" className="text-sm">
          Okuman başka bir yerde güncellenmiş; güncel hâli yukarıda. Seçimlerin duruyor; kaydedersen onların yerine geçer.
        </p>
      )}
      {editions.length > 0 && (
        <Label>
          Baskı
          <select
            className={fieldClass}
            value={editionId ?? ""}
            onChange={(e) => setEditionId(e.target.value === "" ? null : Number(e.target.value))}
          >
            {reading.edition_id === null && <option value="">Seçilmedi</option>}
            {editions.map((ed) => (
              <option key={ed.id} value={ed.id}>{editionLabel(ed)}</option>
            ))}
          </select>
        </Label>
      )}
      <UnitChoice value={unit} onChange={setUnit} />
      <Label>
        Başlama günü
        <Input type="date" max={today()} value={startedOn} onChange={(e) => setStartedOn(e.target.value)} />
      </Label>
      {resets && (
        <p role="note" className="text-sm text-destructive">
          Baskı ya da birim değişince ilerlemen ({progressText(reading)}) sıfırlanacak.
        </p>
      )}
      {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
      <div className="flex gap-2">
        <Button type="submit" disabled={mutation.isPending}>{conflict ? "Yine de kaydet" : "Kaydet"}</Button>
        <Button type="button" variant="outline" onClick={onDone}>Vazgeç</Button>
      </div>
    </form>
  );
}

function UnitChoice({ value, onChange }: { value: ProgressUnit | null; onChange: (u: ProgressUnit) => void }) {
  return (
    <fieldset className="flex flex-col gap-1.5 text-sm">
      <legend className="mb-1.5 font-medium">İlerleme birimi</legend>
      <div className="flex gap-4">
        {(["page", "percent"] as const).map((u) => (
          <label key={u} className="flex items-center gap-1.5">
            <input type="radio" name="unit" value={u} checked={value === u} onChange={() => onChange(u)} />
            {u === "page" ? "Sayfa" : "Yüzde"}
          </label>
        ))}
      </div>
    </fieldset>
  );
}
