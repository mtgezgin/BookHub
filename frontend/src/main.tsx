import { lazy, StrictMode, Suspense } from "react";
import { QueryClientProvider } from "@tanstack/react-query";
import { createRoot } from "react-dom/client";
import { BrowserRouter } from "react-router-dom";
import { App } from "./App";
import { AuthProvider } from "./auth/AuthContext";
import { ToastProvider } from "./components/toast";
// Fontlar Vite paketine girer ve kendi sunucumuzdan gelir (CSP font CDN'ine izin vermiyor).
import "@fontsource-variable/fraunces";
import "@fontsource-variable/inter";
import "./styles.css";
import { reloadOnceForStaleChunk } from "./lib/chunkReload";
import { createQueryClient } from "./lib/queryClient";

// Vite, dinamik bir import (tembel parça) yüklenemeyince bu olayı yayar. preventDefault yalnız Vite'ın hatayı
// fırlatmasını durdurur: import undefined ile çözülür ve tembel bileşen yine hata verir. Sayfa o sırada zaten
// yeniden yüklendiği için ekran yalnız bir an boş kalır.
window.addEventListener("vite:preloadError", (event) => {
  if (reloadOnceForStaleChunk(sessionStorage, Date.now(), () => window.location.reload())) event.preventDefault();
});

// Önbellek modül düzeyinde bir kez kurulur: bileşenin içinde kurulsaydı her yeniden çizimde (ve
// StrictMode'un çift çiziminde) boş bir önbellek doğardı.
const queryClient = createQueryClient();

// Geliştirici aracı (önbellekteki sorguları, durumlarını ve anahtarlarını canlı gösteren panel) YALNIZ
// `make dev`'de yüklenir. import.meta.env.DEV üretim derlemesinde sabit `false` olur; Vite bu dalı ve
// dinamik import'u siler, paket üretim çıktısına girmez (adım 1'de derleme çıktısında doğrulandı).
const QueryDevtools = import.meta.env.DEV
  ? lazy(() => import("@tanstack/react-query-devtools").then((m) => ({ default: m.ReactQueryDevtools })))
  : null;

// QueryClientProvider EN DIŞTA: AuthProvider çıkışta önbelleği temizleyebilsin (Faz 16d adım 8).
createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <AuthProvider>
          <ToastProvider>
            <App />
          </ToastProvider>
        </AuthProvider>
      </BrowserRouter>
      {QueryDevtools && (
        <Suspense fallback={null}>
          <QueryDevtools initialIsOpen={false} />
        </Suspense>
      )}
    </QueryClientProvider>
  </StrictMode>,
);
