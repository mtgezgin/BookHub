import type { ReactNode } from "react";
import { ApiError } from "../api/client";

/** Hata mesajı: ApiError'ın kullanıcıya gösterilebilir mesajı, yoksa genel metin. */
export function ErrorBox({ error }: { error: unknown }) {
  if (!error) return null;
  const msg = error instanceof ApiError ? error.message : "Beklenmeyen bir hata oluştu.";
  return <div className="error" role="alert">{msg}</div>;
}

export function Rating({ value, count }: { value: number | null; count: number }) {
  if (value === null) return <span className="muted">Henüz puan yok</span>;
  return (
    <span className="rating" title={`${count} oy`}>
      ★ {value.toFixed(2)} <span className="muted">({count} oy)</span>
    </span>
  );
}

export function Card({ children }: { children: ReactNode }) {
  return <div className="card">{children}</div>;
}

export function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString("tr-TR", { day: "numeric", month: "long", year: "numeric" });
}
