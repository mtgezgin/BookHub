import { lazy, Suspense } from "react";
import { Link, useLocation } from "react-router-dom";
import { useAuth } from "../auth/AuthContext";
import { Button, buttonVariants } from "./ui/button";

// Menü ve onay diyaloğu (Radix) yalnız giriş yapmış okura gerekiyor: ayrı bir parçada yüklenir,
// anonim ziyaretçinin ana paketine girmez (Faz 14'ün Moderasyon kararıyla aynı gerekçe).
const LibraryMenu = lazy(() => import("./LibraryMenu").then((m) => ({ default: m.LibraryMenu })));

/** Kitap sayfasındaki kitaplık kontrolü: giriş yoksa giriş bağlantısı (dönüş bu kitaba), varsa menü. */
export function LibraryButton({ bookId }: { bookId: number }) {
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
      <LibraryMenu bookId={bookId} />
    </Suspense>
  );
}
