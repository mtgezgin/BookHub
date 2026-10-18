import { useEffect, useState } from "react";
import { api } from "../api/client";
import type { LibraryStatus } from "../api/types";
import { useAuth } from "../auth/AuthContext";

/**
 * Ekrandaki kitapların kitaplık durumları — kitap başına bir istek değil, TEK istek (N+1 yok).
 * Kişisel veri katalog cevabına gömülmez: katalog ve "en sevilenler" herkes için aynı kalır
 * (ortak önbellek), durum ayrı uçtan gelir. Giriş yoksa ya da liste boşsa istek atılmaz.
 * Anahtar sıralı ve tekrarsız kimliklerden kurulur: aynı kitaplar başka sırayla gelince yeniden sormaz.
 * Okuma başarısızsa kartlar rozetsiz kalır (süs bilgi; sayfayı bozmaz).
 */
export function useLibraryStatuses(bookIds: number[]): Record<number, LibraryStatus> {
  const { user } = useAuth();
  const key = [...new Set(bookIds)].sort((a, b) => a - b).join(",");
  const [statuses, setStatuses] = useState<Record<number, LibraryStatus>>({});

  useEffect(() => {
    if (!user || key === "") {
      setStatuses({});
      return;
    }
    let current = true;
    api.libraryStatuses(key.split(",").map(Number)).then(
      // JSON nesne anahtarları zaten metin: statuses[3] ile statuses["3"] aynı alanı okur.
      (r) => current && setStatuses(r.statuses),
      () => {},
    );
    return () => {
      current = false;
    };
  }, [user, key]);

  return statuses;
}
