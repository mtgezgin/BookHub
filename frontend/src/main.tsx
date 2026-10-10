import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter } from "react-router-dom";
import { App } from "./App";
import { AuthProvider } from "./auth/AuthContext";
// Fontlar Vite paketine girer ve kendi sunucumuzdan gelir (CSP font CDN'ine izin vermiyor).
import "@fontsource-variable/fraunces";
import "@fontsource-variable/inter";
import "./styles.css";

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <BrowserRouter>
      <AuthProvider>
        <App />
      </AuthProvider>
    </BrowserRouter>
  </StrictMode>,
);
