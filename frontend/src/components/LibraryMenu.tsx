import { Check, ChevronDown, Plus } from "lucide-react";
import { useEffect, useState } from "react";
import { api, ApiError } from "../api/client";
import type { LibraryStatus } from "../api/types";
import { useAuth } from "../auth/AuthContext";
import { LIBRARY_STATUSES, libraryStatusLabel } from "../lib/format";
import { useToast } from "./toast";
import { Button } from "./ui/button";
import { Dialog, DialogClose, DialogContent, DialogDescription, DialogTitle } from "./ui/dialog";
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuRadioGroup, DropdownMenuRadioItem,
  DropdownMenuSeparator, DropdownMenuTrigger,
} from "./ui/dropdown-menu";

/**
 * Kitap sayfasındaki kitaplık kontrolünün giriş yapmış okura ait kısmı (LibraryButton tembel yükler).
 * Kitaplıkta değilse bölünmüş düğme: ana kısım tek tıkla
 * "Okumak istiyorum" olarak ekler, ok diğer durumları açar. Kitaplıktaysa tek bir menü düğmesi
 * mevcut durumu gösterir; menüde durum değiştirilir ya da kitap (onayla) çıkarılır.
 * İstek sürerken düğmeler pasif: art arda iki seçim iki isteği yarıştırıp sırayı belirsizleştirmesin.
 */
export function LibraryMenu({ bookId }: { bookId: number }) {
  const { user } = useAuth();
  const toast = useToast();
  const [status, setStatus] = useState<LibraryStatus | null>(null);
  const [loaded, setLoaded] = useState(false);
  const [busy, setBusy] = useState(false);
  const [confirming, setConfirming] = useState(false);
  // Kimliğe bağlı, nesneye değil: her sessiz token yenilemesi YENİ bir kullanıcı nesnesi getirir;
  // nesneye bağlı olsaydı durum yeniden okunur ve yeniden denenen PUT'la yarışıp eskisini yazabilirdi.
  const userId = user?.id;

  useEffect(() => {
    if (userId === undefined) return;
    let current = true;
    setLoaded(false);
    setStatus(null);
    // Okuma başarısızsa durum bilinmiyor sayılır; eklemek yine denenebilir (PUT idempotent).
    api.libraryStatuses([bookId]).then(
      (r) => current && setStatus(r.statuses[String(bookId)] ?? null),
      () => {},
    ).finally(() => current && setLoaded(true));
    return () => {
      current = false;
    };
  }, [userId, bookId]);

  async function change(next: LibraryStatus) {
    if (next === status) return;
    setBusy(true);
    try {
      const e = await api.setLibraryStatus(bookId, next);
      toast(status ? `Durum: ${libraryStatusLabel(e.status)}` : `Kitaplığına eklendi: ${libraryStatusLabel(e.status)}`);
      setStatus(e.status);
    } catch (err) {
      toast(err instanceof ApiError ? err.message : "Kitaplık güncellenemedi", "error");
    } finally {
      setBusy(false);
    }
  }

  async function remove() {
    setConfirming(false);
    setBusy(true);
    try {
      await api.removeFromLibrary(bookId);
      setStatus(null);
      toast("Kitaplığından çıkarıldı");
    } catch (err) {
      toast(err instanceof ApiError ? err.message : "Kitaplık güncellenemedi", "error");
    } finally {
      setBusy(false);
    }
  }

  const disabled = busy || !loaded;
  const menu = (
    <DropdownMenuContent align="start">
      <DropdownMenuRadioGroup value={status ?? ""} onValueChange={(v) => void change(v as LibraryStatus)}>
        {LIBRARY_STATUSES.map((s) => (
          <DropdownMenuRadioItem key={s} value={s}>{libraryStatusLabel(s)}</DropdownMenuRadioItem>
        ))}
      </DropdownMenuRadioGroup>
      {status && (
        <>
          <DropdownMenuSeparator />
          <DropdownMenuItem onSelect={() => setConfirming(true)}>Kitaplığımdan çıkar</DropdownMenuItem>
        </>
      )}
    </DropdownMenuContent>
  );

  // disabled Trigger'a da verilir: Radix menüyü pointerdown ile açar ve yalnız KENDİ prop'una bakar;
  // tarayıcı pasif düğmede de pointerdown tetiklediği için düğmenin disabled'ı tek başına yetmez.
  return (
    <>
      {status ? (
        <DropdownMenu>
          <DropdownMenuTrigger asChild disabled={disabled}>
            <Button variant="secondary" disabled={disabled}>
              <Check aria-hidden="true" />
              <span className="sr-only">Kitaplığında: </span>
              {libraryStatusLabel(status)}
              <ChevronDown aria-hidden="true" />
            </Button>
          </DropdownMenuTrigger>
          {menu}
        </DropdownMenu>
      ) : (
        <div role="group" aria-label="Kitaplık" className="inline-flex">
          <Button disabled={disabled} className="rounded-r-none" onClick={() => void change("WANT_TO_READ")}>
            <Plus aria-hidden="true" />
            Okumak istiyorum
          </Button>
          <DropdownMenu>
            <DropdownMenuTrigger asChild disabled={disabled}>
              <Button disabled={disabled} aria-label="Diğer durumlar" className="rounded-l-none border-l border-primary-foreground/30 px-2">
                <ChevronDown aria-hidden="true" />
              </Button>
            </DropdownMenuTrigger>
            {menu}
          </DropdownMenu>
        </div>
      )}

      <Dialog open={confirming} onOpenChange={setConfirming}>
        <DialogContent>
          <DialogTitle>Kitaplığından çıkarılsın mı?</DialogTitle>
          <DialogDescription>Puanın ve yorumların silinmez.</DialogDescription>
          <div className="flex justify-end gap-2">
            <DialogClose asChild>
              <Button type="button" variant="outline">Vazgeç</Button>
            </DialogClose>
            <Button type="button" variant="destructive" disabled={busy} onClick={() => void remove()}>Çıkar</Button>
          </div>
        </DialogContent>
      </Dialog>
    </>
  );
}
