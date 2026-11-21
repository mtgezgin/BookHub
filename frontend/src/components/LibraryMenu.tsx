import { Check, ChevronDown, Plus } from "lucide-react";
import { useState } from "react";
import { ApiError } from "../api/client";
import { useLibraryStatus, useRemoveFromLibrary, useSetLibraryStatus } from "../api/libraryQueries";
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
 *
 * Faz 16d adım 7: durum bir sorgu, ekleme ve çıkarma birer değişiklik. Eskiden burada dört durum (status,
 * loaded, busy, failed), bir "tekrar dene" sayacı ve bir etki vardı. Değişiklikten sonra durumu artık
 * menü kendisi yazmıyor: değişiklik kitaplık kayıtlarını geçersiz kılıyor, menünün durumu da (katalog
 * rozetleri ve Kitaplığım gibi) sunucudan yeniden okunuyor.
 */
export function LibraryMenu({ bookId }: { bookId: number }) {
  const { user } = useAuth();
  const toast = useToast();
  const [confirming, setConfirming] = useState(false);
  // Kimliğe bağlı, nesneye değil: her sessiz token yenilemesi YENİ bir kullanıcı nesnesi getirir; anahtar
  // nesneye bağlı olsaydı durum her yenilemede yeniden okunurdu.
  const userId = user?.id;
  const statusQuery = useLibraryStatus(userId, bookId);
  const setStatus = useSetLibraryStatus(userId);
  const removal = useRemoveFromLibrary(userId);
  const status = statusQuery.data ?? null;

  const failToast = (err: unknown) => toast(err instanceof ApiError ? err.message : "Kitaplık güncellenemedi", "error");

  function change(next: LibraryStatus) {
    if (next === status) return;
    // `status` tıklama anındaki durum: bildirim "eklendi" mi "durum değişti" mi, ona göre.
    setStatus.mutate({ bookId, status: next }, {
      onSuccess: () => toast(status ? `Durum: ${libraryStatusLabel(next)}` : `Kitaplığına eklendi: ${libraryStatusLabel(next)}`),
      onError: failToast,
    });
  }

  function remove() {
    setConfirming(false);
    removal.mutate(bookId, { onSuccess: () => toast("Kitaplığından çıkarıldı"), onError: failToast });
  }

  // Pasif: değişiklik sürerken (sunucuya yazma + ardından durumun yeniden okunması) ve durum ilk kez
  // okunurken.
  const busy = setStatus.isPending || removal.isPending;
  const disabled = busy || statusQuery.data === undefined;
  const menu = (
    <DropdownMenuContent align="start">
      <DropdownMenuRadioGroup value={status ?? ""} onValueChange={(v) => change(v as LibraryStatus)}>
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

  // Okuma başarısızsa durum BİLİNMİYOR: "Okumak istiyorum" gösterilmez, çünkü PUT kayıtlı bir durumun
  // (ör. Okudum) üstüne yazar. Onun yerine tekrar dene. Daha önce okunmuş bir durum varken yeniden okuma
  // (ör. değişiklikten sonra) başarısız olursa da: ekrandaki durum artık doğru olmayabilir.
  if (statusQuery.isError) {
    return (
      <Button variant="outline" disabled={statusQuery.isFetching} onClick={() => void statusQuery.refetch()}>
        Kitaplık durumu yüklenemedi, tekrar dene
      </Button>
    );
  }

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
          <Button disabled={disabled} className="rounded-r-none" onClick={() => change("WANT_TO_READ")}>
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
            <Button type="button" variant="destructive" disabled={busy} onClick={() => remove()}>Çıkar</Button>
          </div>
        </DialogContent>
      </Dialog>
    </>
  );
}
