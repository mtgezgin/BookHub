import { CircleAlert, Star } from "lucide-react";
import { ApiError } from "../api/client";
import { formatScore } from "../lib/format";
import { cn } from "../lib/utils";
import { Alert } from "./ui/alert";

export { formatDate } from "../lib/format";

/** Hata mesajı: ApiError'ın kullanıcıya gösterilebilir mesajı, yoksa genel metin. */
export function ErrorBox({ error, className }: { error: unknown; className?: string }) {
  if (!error) return null;
  const msg = error instanceof ApiError ? error.message : "Beklenmeyen bir hata oluştu.";
  return (
    <Alert variant="destructive" role="alert" className={cn("flex items-start gap-2", className)}>
      <CircleAlert aria-hidden="true" className="mt-0.5 size-4 shrink-0" />
      <span>{msg}</span>
    </Alert>
  );
}

export function Rating({ value, count, className }: { value: number | null; count: number; className?: string }) {
  if (value === null) return <span className={cn("text-sm text-muted-foreground", className)}>Henüz puan yok</span>;
  return (
    <span className={cn("inline-flex items-center gap-1 text-sm", className)}>
      <Star aria-hidden="true" className="size-4 fill-star text-star" />
      <span className="sr-only">Puan </span>
      <span className="font-semibold tabular-nums">{formatScore(value)}</span>
      <span className="text-muted-foreground"> ({count} oy)</span>
    </span>
  );
}

