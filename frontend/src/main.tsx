import { StrictMode } from "react";
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

// Vite, dinamik bir import (tembel parça) yüklenemeyince bu olayı yayar; preventDefault hatayı yutar.
window.addEventListener("vite:preloadError", (event) => {
  if (reloadOnceForStaleChunk(sessionStorage, Date.now(), () => window.location.reload())) event.preventDefault();
});

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <BrowserRouter>
      <AuthProvider>
        <ToastProvider>
          <App />
        </ToastProvider>
      </AuthProvider>
    </BrowserRouter>
  </StrictMode>,
);
