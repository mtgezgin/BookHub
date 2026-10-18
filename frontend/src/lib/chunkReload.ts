// Yeni bir yayın eski parçaları (assets/*-<hash>.js) siler. Yayından önce açılmış bir sekme tembel
// bir parçayı (LibraryMenu, Kitaplığım, Moderasyon) istediğinde dosya artık yok, import düşer ve
// uygulama boş ekranla çökerdi. Sayfayı yeniden yüklemek yeni index.html'i, onunla yeni adları getirir.
// Döngüye girmemek için en çok 10 sn'de bir kez; depolama okunamıyorsa hiç yeniden yüklenmez.

const KEY = "bookhub.chunk_reload_at";
const WINDOW_MS = 10_000;

/** Yeniden yükleme başlatıldıysa true döner (çağıran hatayı yutabilir). */
export function reloadOnceForStaleChunk(
  storage: Pick<Storage, "getItem" | "setItem">,
  now: number,
  reload: () => void,
): boolean {
  try {
    const last = Number(storage.getItem(KEY) ?? 0);
    if (now - last < WINDOW_MS) return false;
    storage.setItem(KEY, String(now));
  } catch {
    return false;
  }
  reload();
  return true;
}
