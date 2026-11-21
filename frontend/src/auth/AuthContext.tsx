import { useQueryClient } from "@tanstack/react-query";
import { createContext, useContext, useEffect, useState, type ReactNode } from "react";
import { api, subscribeSession } from "../api/client";
import type { User } from "../api/types";

interface AuthState {
  user: User | null;
  /** Açılıştaki oturum geri yükleme KESİN bir sonuçla bitti mi (girişli ya da anonim). */
  ready: boolean;
  /**
   * Oturum belirsiz: tarayıcıda oturum açılmış görünüyor ama sunucuya sorulamadı (ağ, 429, 502).
   * Bu sürede ready false kalır; korumalı sayfalar giriş sayfasına YÖNLENDİRMEZ, arka planda
   * yeniden denenir (REVIEW_FEEDBACK F36).
   */
  uncertain: boolean;
  login: (email: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
}

const AuthContext = createContext<AuthState | null>(null);

/** Belirsiz oturumda yeniden deneme aralıkları. Sonuncudan sonra yalnız "online" olayı dener. */
export const RESTORE_RETRY_DELAYS_MS = [1_000, 3_000, 10_000, 30_000];

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [ready, setReady] = useState(false);
  const [uncertain, setUncertain] = useState(false);
  // Karar 4 (Faz 16d adım 8): oturum kapanınca sunucu verisinin önbelleği TAMAMEN boşaltılır. Aynı
  // tarayıcıda sonraki kişi öncekinin kitaplığını, puanlarını ya da (admin idiyse) moderasyon kuyruğunu
  // önbellekten görmesin. Anahtarlarda kullanıcı kimliği olduğu için ekranda zaten görünmezdi; ama veri
  // bellekte durur, geliştirici aracında okunur ve yalnız "me" altına yazılmayı unutulan bir sonraki
  // anahtar onu ekrana taşır. Yalnız "me"yi silmek yerine hepsi: kural basit kalsın, ileride eklenen
  // kullanıcıya özel bir anahtar da kendiliğinden kapsansın.
  // Bedeli: çıkış anında açık sayfanın herkese açık verisi de (katalog, kitap) yeniden çekilir.
  const queryClient = useQueryClient();

  useEffect(() => {
    let cancelled = false;
    let waiting = false; // belirsiz durumdayız ve bir deneme bekliyor
    let attempt = 0;
    let timer: ReturnType<typeof setTimeout> | undefined;

    const settle = (u: User | null) => {
      waiting = false;
      setUser(u);
      setUncertain(false);
      setReady(true);
    };

    // İstemci oturumu düşürürse (yenileme reddedildi) ya da başka bir istek oturumu yenilerse
    // arayüz de hemen bilsin. Kullanıcı geldiyse belirsizlik bitmiştir.
    // Oturumun kapandığı HER yol buradan geçer: çıkış düğmesi (api.logout → clearSession), başka sekmede
    // çıkış ("storage" olayı), yenilemenin reddi. Önbellek bu yüzden burada temizlenir. Oturum yenilemesi
    // (aynı kullanıcı, yeni token) da buraya gelir ama u dolu olduğu için önbelleğe dokunmaz.
    subscribeSession((u) => {
      if (cancelled) return;
      if (u) settle(u);
      else {
        queryClient.clear();
        setUser(null);
      }
    });

    // async + try/catch: zincirin her adımındaki hata burada biter (yakalanmamış bir ret kalmaz).
    const attemptRestore = async () => {
      let restored: User | null;
      try {
        restored = await api.restore();
      } catch {
        restored = null; // ağ hatası: aşağıda "belirsiz mi?" sorusu karar verir
      }
      if (cancelled) return;
      if (restored) return settle(restored);
      if (!api.sessionUncertain()) return settle(null);
      // Oturum olabilir ama sorulamadı: anonim SAYMA, yeniden dene.
      waiting = true;
      setUncertain(true);
      if (attempt < RESTORE_RETRY_DELAYS_MS.length) timer = setTimeout(run, RESTORE_RETRY_DELAYS_MS[attempt++]);
    };
    const run = () => void attemptRestore();
    // Ağ geri geldiğinde beklemeden dene.
    const onOnline = () => {
      if (!waiting) return;
      clearTimeout(timer);
      run();
    };
    window.addEventListener("online", onOnline);
    run();

    return () => {
      cancelled = true;
      clearTimeout(timer);
      window.removeEventListener("online", onOnline);
    };
    // queryClient uygulamada tek ve hiç değişmez (main.tsx); bağımlılık listesinde yalnız kural gereği.
  }, [queryClient]);

  const value: AuthState = {
    user,
    ready,
    uncertain,
    login: async (email, password) => {
      const u = await api.login(email, password);
      setUser(u);
      // Belirsiz durumdayken yapılan giriş de kesin bir sonuçtur.
      setUncertain(false);
      setReady(true);
    },
    logout: async () => {
      await api.logout();
      // api.logout oturumu düşürürken yukarıdaki abonelik önbelleği zaten temizledi; burada yeniden
      // yazmak, istemcinin o bildirimi bir gün atlaması durumunda da çıkışı güvenli tutar.
      queryClient.clear();
      setUser(null);
    },
  };
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthState {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth, AuthProvider içinde kullanılmalı");
  return ctx;
}
