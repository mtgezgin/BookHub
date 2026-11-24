import { useState } from "react";
import { ApiError } from "../api/client";
import type { useDeleteReadingLog, useUpdateReadingLog } from "../api/libraryQueries";
import type { Edition, ReadingLogEntry } from "../api/types";
import { today } from "../lib/day";
import { dayRange, editionLabel, outcomeLabel, progressText } from "../lib/shelf";
import { useToast } from "./toast";
import { Button } from "./ui/button";
import { Dialog, DialogClose, DialogContent, DialogDescription, DialogTitle } from "./ui/dialog";
import { Input } from "./ui/input";
import { Label } from "./ui/label";

/*
 * Okuma geçmişi (Faz 17A-1 adım 5): raf kontrolünün altında, kapanmış okumalar en yeni önce. Her satırda
 * sonuç, günler, (birden çok baskı varsa) baskı ve yarım kalanda nerede kalındığı; günleri düzelt ya da
 * sil. Kayıt kitaplıktan çıkarılsa da kalır (spec § Kararlar "Geçmiş ve kitaplık"), o yüzden kitap
 * kitaplıkta değilken de görünür.
 */

interface Props {
  log: ReadingLogEntry[];
  editions: Edition[];
  /** Günleri düzeltilen kayıt; durum satırındaki "tarih ekle" de bunu açar (ShelfControl). */
  editing: number | null;
  onEdit: (logId: number | null) => void;
  dates: ReturnType<typeof useUpdateReadingLog>;
  remove: ReturnType<typeof useDeleteReadingLog>;
  busy: boolean;
}

export function ReadingHistory({ log, editions, editing, onEdit, dates, remove, busy }: Props) {
  const toast = useToast();
  const [deleting, setDeleting] = useState<ReadingLogEntry | null>(null);
  if (log.length === 0) return null;

  function confirmDelete() {
    const target = deleting!;
    setDeleting(null);
    remove.mutate(target.id, {
      onSuccess: () => toast("Okuma geçmişinden silindi"),
      onError: (err) => toast(err instanceof ApiError ? err.message : "Silinemedi", "error"),
    });
  }

  return (
    <section aria-labelledby="reading-history-heading" className="mt-2">
      <h2 id="reading-history-heading" className="text-sm font-semibold">Okuma geçmişin</h2>
      <ul className="mt-2 flex flex-col gap-2">
        {log.map((l) => {
          const edition = editions.length > 1 ? editions.find((e) => e.id === l.edition_id) : undefined;
          const where = l.outcome === "ABANDONED" ? progressText(l) : null;
          return (
            <li key={l.id} className="rounded-md border px-3 py-2 text-sm">
              <div className="flex flex-wrap items-baseline justify-between gap-x-3 gap-y-1">
                <span>
                  <span className="font-medium">{outcomeLabel(l.outcome)}</span>
                  {where && <span className="text-muted-foreground"> · {where}</span>}
                  <span className="text-muted-foreground"> · {dayRange(l.started_on, l.ended_on)}</span>
                </span>
                <span className="flex gap-3">
                  <Button variant="link" size="sm" className="h-auto p-0" disabled={busy} onClick={() => onEdit(editing === l.id ? null : l.id)}>
                    Günleri düzelt
                  </Button>
                  <Button variant="link" size="sm" className="h-auto p-0 text-destructive" disabled={busy} onClick={() => setDeleting(l)}>
                    Sil
                  </Button>
                </span>
              </div>
              {edition && <p className="mt-0.5 text-xs text-muted-foreground">{editionLabel(edition)}</p>}
              {editing === l.id && <LogDatesForm entry={l} mutation={dates} onDone={() => onEdit(null)} />}
            </li>
          );
        })}
      </ul>

      <Dialog open={deleting !== null} onOpenChange={(open) => !open && setDeleting(null)}>
        <DialogContent>
          <DialogTitle>Bu okuma geçmişinden silinsin mi?</DialogTitle>
          <DialogDescription>Kitaplığındaki durumu değişmez.</DialogDescription>
          <div className="flex justify-end gap-2">
            <DialogClose asChild>
              <Button type="button" variant="outline">Vazgeç</Button>
            </DialogClose>
            <Button type="button" variant="destructive" disabled={busy} onClick={confirmDelete}>Sil</Button>
          </div>
        </DialogContent>
      </Dialog>
    </section>
  );
}

/**
 * Bir okumanın günleri. Yalnız değişen gün gönderilir. Bitiş başlangıçtan önceyse istek hiç gitmez
 * (sunucu da 400 invalid_dates der; bu yalnız beklemeyi kısaltır). Bir gün silinemez: sunucu "gelmeyen
 * alan değişmez" der, boş alan gönderilmez.
 */
function LogDatesForm({ entry, mutation, onDone }: {
  entry: ReadingLogEntry; mutation: ReturnType<typeof useUpdateReadingLog>; onDone: () => void;
}) {
  const toast = useToast();
  const [startedOn, setStartedOn] = useState(entry.started_on ?? "");
  const [endedOn, setEndedOn] = useState(entry.ended_on ?? "");
  const [error, setError] = useState<string | null>(null);

  function submit(e: React.FormEvent) {
    e.preventDefault();
    if (mutation.isPending) return;
    if (startedOn && endedOn && endedOn < startedOn) {
      setError("Bitiş günü başlangıç gününden önce olamaz.");
      return;
    }
    const dates: { started_on?: string; ended_on?: string } = {};
    if (startedOn && startedOn !== entry.started_on) dates.started_on = startedOn;
    if (endedOn && endedOn !== entry.ended_on) dates.ended_on = endedOn;
    if (Object.keys(dates).length === 0) {
      onDone();
      return;
    }
    setError(null);
    mutation.mutate({ logId: entry.id, dates }, {
      onSuccess: () => {
        toast("Günler kaydedildi");
        onDone();
      },
      onError: (err) => {
        if (err instanceof ApiError && err.status === 404) toast(err.message, "error");
        else setError(err instanceof ApiError ? err.message : "Kaydedilemedi");
      },
    });
  }

  return (
    <form aria-label="Okumanın günleri" onSubmit={submit} className="mt-2 flex flex-col gap-3 border-t pt-3">
      <div className="flex flex-wrap gap-3">
        <Label>
          Başlama günü
          <Input type="date" max={today()} value={startedOn} onChange={(e) => setStartedOn(e.target.value)} />
        </Label>
        <Label>
          Bitiş günü
          <Input type="date" max={today()} value={endedOn} onChange={(e) => setEndedOn(e.target.value)} />
        </Label>
      </div>
      {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
      <div className="flex gap-2">
        <Button type="submit" size="sm" disabled={mutation.isPending}>Kaydet</Button>
        <Button type="button" size="sm" variant="outline" onClick={onDone}>Vazgeç</Button>
      </div>
    </form>
  );
}
