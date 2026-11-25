import { lazy, Suspense } from "react";
import { Link, useLocation } from "react-router-dom";
import type { Edition } from "../api/types";
import { useAuth } from "../auth/AuthContext";
import { Button, buttonVariants } from "./ui/button";

// Raf kontrolü (menü, diyaloglar, formlar, geçmiş) yalnız giriş yapmış okura gerekiyor: ayrı bir parçada
// yüklenir, anonim ziyaretçinin ana paketine girmez (Faz 14'ün Moderasyon kararıyla aynı gerekçe).
const ShelfControl = lazy(() => import("./ShelfControl").then((m) => ({ default: m.ShelfControl })));

/**
 * Kitap sayfasındaki kitaplık kontrolü: giriş yoksa giriş bağlantısı (dönüş bu kitaba), varsa raf kontrolü.
 * onRate: bitirince gösterilen "Puan ver" davetinin yapacağı (kitap sayfası puan seçiciye götürür).
 */
export function LibraryButton({ bookId, editions, onRate }: Readonly<{ bookId: number; editions: Edition[]; onRate?: () => void }>) {
  const { user } = useAuth();
  const location = useLocation();
  if (!user) {
    return (
      <Link to="/login" state={{ from: location }} className={buttonVariants({ variant: "outline" })}>
        Kitaplığa eklemek için giriş yap
      </Link>
    );
  }
  return (
    <Suspense fallback={<Button disabled>Okumak istiyorum</Button>}>
      <ShelfControl bookId={bookId} editions={editions} onRate={onRate} />
    </Suspense>
  );
}
