import { lazy, Suspense, type ReactNode } from "react";
import { Link, Navigate, NavLink, Route, Routes, useLocation } from "react-router-dom";
import { useAuth } from "./auth/AuthContext";
import { BrandMark } from "./components/BrandMark";
import { NotFound } from "./components/NotFound";
import { ThemeToggle } from "./components/ThemeToggle";
import { UserMenu } from "./components/UserMenu";
import { Alert } from "./components/ui/alert";
import { Button, buttonVariants } from "./components/ui/button";
import { cn } from "./lib/utils";
import { Account } from "./pages/Account";
import { Login, Register } from "./pages/Auth";
import { BookDetail } from "./pages/BookDetail";
import { Home } from "./pages/Home";
import { Suggest } from "./pages/Suggest";

// Moderasyon yalnız admin içindir ve Radix Dialog'u (odak hapsi, kaydırma kilidi…) getirir:
// ayrı bir parça olarak, sayfa açılınca yüklenir → diğer ziyaretçiler o kodu hiç indirmez.
const Moderation = lazy(() => import("./pages/admin/Moderation").then((m) => ({ default: m.Moderation })));
// Kitaplığım da yalnız giriş yapmış okura gerekiyor ve Radix diyaloğunu getiriyor: aynı gerekçeyle ayrı parça.
const Library = lazy(() => import("./pages/Library").then((m) => ({ default: m.Library })));

/**
 * Rota koruması YALNIZCA kullanıcı deneyimi içindir (girişsiz kullanıcıyı login'e yönlendirir).
 * Asıl yetki kontrolü BACKEND'de: bu bileşen atlatılsa bile /api/admin/* 401/403 döner.
 */
function RequireRole({ role, children }: { role?: "ADMIN"; children: ReactNode }) {
  const { user, ready } = useAuth();
  const location = useLocation();
  if (!ready) return <p className="text-muted-foreground">Yükleniyor…</p>;
  // state.from: girişten sonra bu sayfaya dönülür (Auth.tsx useReturnTo).
  if (!user) return <Navigate to="/login" replace state={{ from: location }} />;
  if (role && user.role !== role) return <Alert variant="destructive" role="alert">Bu sayfa için yetkin yok.</Alert>;
  return <>{children}</>;
}

/** NavLink bulunulan sayfada aria-current="page" koyar; görünümü de ona bağlı. */
function NavItem({ to, children, className }: { to: string; children: ReactNode; className?: string }) {
  return (
    <NavLink
      to={to}
      className={({ isActive }) =>
        cn(buttonVariants({ variant: "ghost", size: "sm" }), "px-2 sm:px-3", isActive && "bg-accent text-accent-foreground", className)
      }
    >
      {children}
    </NavLink>
  );
}

export function App() {
  const { user, logout } = useAuth();
  return (
    <div className="flex min-h-svh flex-col">
      {/* Klavye kullanıcısı her sayfada menüyü Tab'la geçmek zorunda kalmasın; odaklanınca görünür. */}
      <a
        href="#main"
        className="sr-only focus:not-sr-only focus:fixed focus:top-3 focus:left-3 focus:z-50 focus:rounded-md focus:bg-card focus:px-3 focus:py-2 focus:shadow-lg"
      >
        İçeriğe atla
      </a>
      <header className="sticky top-0 z-20 border-b bg-background/85 backdrop-blur">
        <nav aria-label="Ana menü" className="mx-auto flex max-w-6xl items-center gap-1 px-4 py-3 sm:px-6">
          <Link to="/" className="mr-auto flex items-center gap-2 rounded-md font-serif text-lg font-semibold sm:text-xl">
            <BrandMark className="size-7 sm:size-8" />
            BookHub
          </Link>
          {/* Dar ekranda bunlar kullanıcı menüsünün içinde (UserMenu). */}
          {user && <NavItem to="/library" className="hidden md:inline-flex">Kitaplığım</NavItem>}
          {user && <NavItem to="/suggest" className="hidden md:inline-flex">Kitap öner</NavItem>}
          {user?.role === "ADMIN" && <NavItem to="/admin" className="hidden md:inline-flex">Moderasyon</NavItem>}
          <ThemeToggle />
          {user ? (
            <UserMenu user={user} onLogout={logout} />
          ) : (
            <>
              <NavItem to="/login">Giriş</NavItem>
              <Button asChild size="sm">
                <Link to="/register">Kayıt ol</Link>
              </Button>
            </>
          )}
        </nav>
      </header>
      <main id="main" tabIndex={-1} className="mx-auto w-full max-w-6xl flex-1 px-4 py-6 outline-none sm:px-6 sm:py-8">
        <Routes>
          <Route path="/" element={<Home />} />
          <Route path="/books/:id" element={<BookDetail />} />
          <Route path="/login" element={<Login />} />
          <Route path="/register" element={<Register />} />
          <Route path="/library" element={<RequireRole><Suspense fallback={<p className="text-muted-foreground">Yükleniyor…</p>}><Library /></Suspense></RequireRole>} />
          <Route path="/suggest" element={<RequireRole><Suggest /></RequireRole>} />
          <Route path="/account" element={<RequireRole><Account /></RequireRole>} />
          <Route path="/admin" element={<RequireRole role="ADMIN"><Suspense fallback={<p className="text-muted-foreground">Yükleniyor…</p>}><Moderation /></Suspense></RequireRole>} />
          <Route path="*" element={<NotFound />} />
        </Routes>
      </main>
    </div>
  );
}
