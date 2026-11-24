import { ChevronDown, Ellipsis, Plus } from "lucide-react";
import { useState } from "react";
import { ApiError } from "../api/client";
import {
  useDeleteReadingLog, useShelf, useShelfCommand, useUpdateReading, useUpdateReadingLog,
} from "../api/libraryQueries";
import { useMyRating } from "../api/queries";
import type { Edition, Reading, Shelf, ShelfCommand } from "../api/types";
import { useAuth } from "../auth/AuthContext";
import { today } from "../lib/day";
import { libraryStatusLabel } from "../lib/format";
import {
  abandonedText, closedReadingOf, commandToast, finishedText, hasProgress, progressPercent, progressText,
} from "../lib/shelf";
import { ReadingHistory } from "./ReadingHistory";
import { ProgressForm, ReadingEditForm } from "./ShelfForms";
import { useToast } from "./toast";
import { Badge } from "./ui/badge";
import { Button } from "./ui/button";
import { Dialog, DialogClose, DialogContent, DialogDescription, DialogTitle } from "./ui/dialog";
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger,
} from "./ui/dropdown-menu";

/*
 * TEK RAF KONTROLÜ (Faz 17A-1 adım 5, spec 17A § 5). Faz 15'in durum menüsünün yerini alır: kullanıcı
 * dört durumdan birini SEÇMEZ; bulunduğu durumdan yapılabilecekleri görür ve niyetini söyler (spec kavram
 * 2). Her düğme bir komut; komut her durumda tanımlı olduğu için iki sekmede ya da çift tıkta hata yok.
 *
 *   Durum            Ana eylem                         Diğer seçenekler
 *   kitaplıkta değil Okumak istiyorum                  Okumaya başla · Zaten okudum
 *   Okumak istiyorum Okumaya başla                     Zaten okudum · Kitaplığımdan çıkar
 *   Okuyorum         İlerlemeyi güncelle · Bitirdim    Yarım bıraktım · Ertele · Okumayı düzenle · Çıkar
 *   Okudum           Yeniden oku                       Kitaplığımdan çıkar
 *   Yarım bıraktım   Devam et                          Baştan başla · Zaten okudum · Kitaplığımdan çıkar
 *
 * Veri: rafın tamamı tek sorgu (useShelf); her değişiklik güncel rafı döner ve kanca onu önbelleğe yazar,
 * bileşen kendi kopyasını tutmaz. Değişiklik sürerken bütün kontroller pasif: art arda iki komut yarışıp
 * sırayı belirsizleştirmesin.
 */

type Panel = "progress" | "edit" | null;
type Confirm = "remove" | "postpone" | null;

export function ShelfControl({ bookId, editions, onRate }: { bookId: number; editions: Edition[]; onRate?: () => void }) {
  const { user } = useAuth();
  const toast = useToast();
  // Kimliğe bağlı, nesneye değil: her sessiz token yenilemesi YENİ bir kullanıcı nesnesi getirir.
  const userId = user?.id;
  const shelfQuery = useShelf(userId, bookId);
  const command = useShelfCommand(userId, bookId);
  const reading = useUpdateReading(userId, bookId);
  const logDates = useUpdateReadingLog(userId, bookId);
  const deleteLog = useDeleteReadingLog(userId, bookId);
  const myRating = useMyRating(userId, bookId);
  const [panel, setPanel] = useState<Panel>(null);
  const [confirm, setConfirm] = useState<Confirm>(null);
  const [editingLog, setEditingLog] = useState<number | null>(null);
  // "Puan ver" daveti yalnız bu ekranda bitirilince (spec: "Bitirince … isteğe bağlı davet").
  const [justFinished, setJustFinished] = useState(false);

  const busy = command.isPending || reading.isPending || logDates.isPending || deleteLog.isPending;
  const shelf = shelfQuery.data;

  function run(c: ShelfCommand) {
    setConfirm(null);
    command.mutate(c, {
      onSuccess: () => {
        toast(commandToast(c));
        setPanel(null);
        setEditingLog(null);
        setJustFinished(c.command === "finish");
      },
      onError: (err) => toast(err instanceof ApiError ? err.message : "Kitaplık güncellenemedi", "error"),
    });
  }

  // Okuma başarısızsa durum BİLİNMİYOR: "Okumak istiyorum" gösterilmez (yanlış bir komut kayıtlı bir
  // durumu değiştirirdi), tekrar dene gösterilir. Daha önce okunmuş bir raf varken yeniden okuma (ör.
  // 409'dan sonra) başarısız olursa da: ekrandaki raf artık doğru olmayabilir.
  if (shelfQuery.isError) {
    return (
      <Button variant="outline" disabled={shelfQuery.isFetching} onClick={() => void shelfQuery.refetch()}>
        Kitaplık durumu yüklenemedi, tekrar dene
      </Button>
    );
  }
  if (!shelf) return <Button disabled>Okumak istiyorum</Button>;

  const entry = shelf.entry;
  const current = entry?.reading ?? null;
  const closed = closedReadingOf(shelf);
  // Onay metni için: ilerleme varsa nerede olduğu ("42 / 310 sayfa").
  const where = hasProgress(current) ? progressText(current!) : null;
  const startToday: ShelfCommand = { command: "start", body: { started_on: today() } };
  const markRead: ShelfCommand = { command: "mark-read" };
  const removeItem = { label: "Kitaplığımdan çıkar", onSelect: () => setConfirm("remove"), separated: true };

  let primary: React.ReactNode;
  let menu: MenuItem[];
  switch (entry?.status) {
    case undefined:
      primary = null; // bölünmüş düğme aşağıda
      menu = [{ label: "Okumaya başla", onSelect: () => run(startToday) }, { label: "Zaten okudum", onSelect: () => run(markRead) }];
      break;
    case "WANT_TO_READ":
      primary = <Button disabled={busy} onClick={() => run(startToday)}>Okumaya başla</Button>;
      menu = [{ label: "Zaten okudum", onSelect: () => run(markRead) }, removeItem];
      break;
    case "READING":
      primary = (
        <>
          <Button variant="secondary" disabled={busy} aria-expanded={panel === "progress"} onClick={() => setPanel(panel === "progress" ? null : "progress")}>
            {progressText(current!) ? "İlerlemeyi güncelle" : "İlerlemeni gir"}
          </Button>
          <Button disabled={busy} onClick={() => run({ command: "finish", body: { ended_on: today() } })}>Bitirdim</Button>
        </>
      );
      menu = [
        { label: "Yarım bıraktım", onSelect: () => run({ command: "abandon", body: { ended_on: today() } }) },
        // İlerleme varken ertelemek geçmişe "yarım bıraktın" yazar: önce sorulur. Yoksa yan etkisiz.
        { label: "Ertele", onSelect: () => (hasProgress(current) ? setConfirm("postpone") : run({ command: "postpone" })) },
        { label: "Okumayı düzenle", onSelect: () => setPanel("edit") },
        removeItem,
      ];
      break;
    case "READ":
      primary = <Button variant="secondary" disabled={busy} onClick={() => run(startToday)}>Yeniden oku</Button>;
      menu = [removeItem];
      break;
    case "ABANDONED":
      // Devam et: son yarım kalan okuma geri açılır (sayfa ve başlangıç günü korunur); yoksa sunucu baştan başlatır.
      primary = <Button disabled={busy} onClick={() => run({ command: "start", body: { mode: "resume" } })}>Devam et</Button>;
      menu = [
        { label: "Baştan başla", onSelect: () => run({ command: "start", body: { mode: "restart", started_on: today() } }) },
        { label: "Zaten okudum", onSelect: () => run(markRead) },
        removeItem,
      ];
      break;
  }

  return (
    <div role="group" aria-label="Kitaplık" className="flex flex-col gap-3">
      {entry && <StatusLine shelf={shelf} closed={closed} busy={busy} onEditDates={setEditingLog} />}
      {current && <ProgressLine reading={current} />}

      <div className="flex flex-wrap items-center gap-2">
        {entry ? (
          <>
            {primary}
            <MoreMenu items={menu} disabled={busy} />
          </>
        ) : (
          <div className="inline-flex">
            <Button disabled={busy} className="rounded-r-none" onClick={() => run({ command: "want" })}>
              <Plus aria-hidden="true" />
              Okumak istiyorum
            </Button>
            <MoreMenu items={menu} disabled={busy} split />
          </div>
        )}
      </div>

      {current && entry && panel === "progress" && (
        <ProgressForm entry={entry} reading={current} editions={editions} mutation={reading} onDone={() => setPanel(null)} />
      )}
      {current && entry && panel === "edit" && (
        <ReadingEditForm entry={entry} reading={current} editions={editions} mutation={reading} onDone={() => setPanel(null)} />
      )}

      {justFinished && entry?.status === "READ" && myRating.data === null && onRate && (
        <p className="text-sm">
          Kitap hakkında ne düşünüyorsun?{" "}
          <Button variant="link" size="sm" className="h-auto p-0" onClick={onRate}>Puan ver</Button>
        </p>
      )}

      <ReadingHistory
        log={shelf.log}
        editions={editions}
        editing={editingLog}
        onEdit={setEditingLog}
        dates={logDates}
        remove={deleteLog}
        busy={busy}
      />

      <Dialog open={confirm !== null} onOpenChange={(open) => !open && setConfirm(null)}>
        <DialogContent>
          {confirm === "postpone" ? (
            <>
              <DialogTitle>Okuma ertelensin mi?</DialogTitle>
              <DialogDescription>
                Okuman ({where}) geçmişine “yarım bıraktın” olarak kaydedilecek; kitap okumak
                istediklerine döner.
              </DialogDescription>
            </>
          ) : (
            <>
              <DialogTitle>Kitaplığından çıkarılsın mı?</DialogTitle>
              <DialogDescription>
                {where
                  ? `Okuman (${where}) geçmişine “yarım bıraktın” olarak kaydedilecek. Puanın ve yorumların silinmez.`
                  : "Puanın, yorumların ve okuma geçmişin silinmez."}
              </DialogDescription>
            </>
          )}
          <div className="flex justify-end gap-2">
            <DialogClose asChild>
              <Button type="button" variant="outline">Vazgeç</Button>
            </DialogClose>
            {confirm === "postpone" ? (
              <Button type="button" disabled={busy} onClick={() => run({ command: "postpone" })}>Ertele</Button>
            ) : (
              <Button type="button" variant="destructive" disabled={busy} onClick={() => run({ command: "remove" })}>Çıkar</Button>
            )}
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
}

/** Durum rozeti ve durumu açıklayan satır ("Bugün bitirdin", "Okudun · tarih ekle", "Yarım bıraktın · %35"). */
function StatusLine({ shelf, closed, busy, onEditDates }: {
  shelf: Shelf; closed: ReturnType<typeof closedReadingOf>; busy: boolean; onEditDates: (logId: number) => void;
}) {
  const status = shelf.entry!.status;
  let detail: React.ReactNode = null;
  if (status === "READ") {
    detail = (
      <>
        {finishedText(closed)}
        {/* Tarih yalnız bir geçmiş kaydı varsa düzeltilebilir (Faz 15'ten kalan "Okudum"da kayıt yok). */}
        {closed && (
          <>
            {" · "}
            <Button variant="link" size="sm" className="h-auto p-0" disabled={busy} onClick={() => onEditDates(closed.id)}>
              {closed.ended_on ? "tarihi değiştir" : "tarih ekle"}
            </Button>
          </>
        )}
      </>
    );
  } else if (status === "ABANDONED") {
    detail = abandonedText(closed);
  }
  return (
    <p className="flex flex-wrap items-center gap-x-2 gap-y-1 text-sm text-muted-foreground">
      <Badge>
        <span className="sr-only">Kitaplığında: </span>
        {libraryStatusLabel(status)}
      </Badge>
      {detail}
    </p>
  );
}

/**
 * Süren okumanın ilerlemesi. Oran biliniyorsa çubuk (role="progressbar", ekran okuyucu metni de okur);
 * girilmemişse yalnız bir cümle. Toplamı bilinmeyen sayfada çubuk yok: neyin yüzdesi olduğu bilinmiyor.
 */
function ProgressLine({ reading }: { reading: Reading }) {
  const text = progressText(reading);
  const percent = progressPercent(reading);
  return (
    <div className="flex max-w-sm flex-col gap-1.5">
      {percent !== null && (
        <div
          role="progressbar"
          aria-label="Okuma ilerlemen"
          aria-valuemin={0}
          aria-valuemax={100}
          aria-valuenow={percent}
          aria-valuetext={text ?? undefined}
          className="h-2 overflow-hidden rounded-full bg-muted"
        >
          <div className="h-full rounded-full bg-primary transition-[width]" style={{ width: `${percent}%` }} />
        </div>
      )}
      <p className="text-sm tabular-nums text-muted-foreground">{text ?? "İlerlemeni henüz girmedin"}</p>
    </div>
  );
}

interface MenuItem {
  label: string;
  onSelect: () => void;
  separated?: boolean;
}

/**
 * "Diğer seçenekler" menüsü. disabled Trigger'a da verilir: Radix menüyü pointerdown ile açar ve yalnız
 * KENDİ prop'una bakar; tarayıcı pasif düğmede de pointerdown tetiklediği için düğmenin disabled'ı tek
 * başına yetmez (Faz 15'ten).
 */
function MoreMenu({ items, disabled, split = false }: { items: MenuItem[]; disabled: boolean; split?: boolean }) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild disabled={disabled}>
        <Button
          variant={split ? "default" : "outline"}
          disabled={disabled}
          aria-label="Diğer seçenekler"
          className={split ? "rounded-l-none border-l border-primary-foreground/30 px-2" : "px-2"}
        >
          {split ? <ChevronDown aria-hidden="true" /> : <Ellipsis aria-hidden="true" />}
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start">
        {items.map((item) => (
          <MenuEntry key={item.label} item={item} />
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

function MenuEntry({ item }: { item: MenuItem }) {
  return (
    <>
      {item.separated && <DropdownMenuSeparator />}
      <DropdownMenuItem onSelect={item.onSelect}>{item.label}</DropdownMenuItem>
    </>
  );
}
