import { useEffect, useState } from "react";

// Tema: <html data-theme="light|dark">. İlk boyamadan önce public/theme-init.js aynı kuralla
// uygular (CSP satır içi betiğe izin vermiyor); bu modül yalnız sonradan değiştirir.
export type Theme = "light" | "dark";
const KEY = "bookhub.theme";

function stored(): Theme | null {
  try {
    const v = localStorage.getItem(KEY);
    return v === "light" || v === "dark" ? v : null;
  } catch {
    return null; // localStorage yasaklı olabilir (Safari özel pencere)
  }
}

function system(): Theme {
  return typeof matchMedia === "function" && matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}

function current(): Theme {
  const t = document.documentElement.dataset.theme;
  return t === "light" || t === "dark" ? t : (stored() ?? system());
}

function apply(t: Theme, remember: boolean) {
  document.documentElement.dataset.theme = t;
  if (!remember) return;
  try {
    localStorage.setItem(KEY, t);
  } catch {
    // seçim bu oturumla sınırlı kalır
  }
}

/** Seçim yoksa sistem değişikliğini izler; kullanıcı seçince o kalıcıdır. */
export function useTheme(): [Theme, (t: Theme) => void] {
  const [theme, setTheme] = useState<Theme>(current);

  useEffect(() => {
    if (stored() || typeof matchMedia !== "function") return;
    const mq = matchMedia("(prefers-color-scheme: dark)");
    const follow = () => {
      if (stored()) return;
      const t: Theme = mq.matches ? "dark" : "light";
      apply(t, false);
      setTheme(t);
    };
    mq.addEventListener("change", follow);
    return () => mq.removeEventListener("change", follow);
  }, []);

  return [
    theme,
    (t) => {
      apply(t, true);
      setTheme(t);
    },
  ];
}
