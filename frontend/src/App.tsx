import type { ReactNode } from "react";
import { Link, Navigate, NavLink, Route, Routes } from "react-router-dom";
import { useAuth } from "./auth/AuthContext";
import { BrandMark } from "./components/BrandMark";
import { NotFound } from "./components/NotFound";
import { Alert } from "./components/ui/alert";
import { Button, buttonVariants } from "./components/ui/button";
import { cn } from "./lib/utils";
import { Account } from "./pages/Account";
import { Moderation } from "./pages/admin/Moderation";
import { Login, Register } from "./pages/Auth";
import { BookDetail } from "./pages/BookDetail";
import { Home } from "./pages/Home";
import { Suggest } from "./pages/Suggest";

/**
 * Rota koruması YALNIZCA kullanıcı deneyimi içindir (girişsiz kullanıcıyı login'e yönlendirir).
 * Asıl yetki kontrolü BACKEND'de: bu bileşen atlatılsa bile /api/admin/* 401/403 döner.
 */
function RequireRole({ role, children }: { role?: "ADMIN"; children: ReactNode }) {
  const { user, ready } = useAuth();
  if (!ready) return <p className="text-muted-foreground">Yükleniyor…</p>;
  if (!user) return <Navigate to="/login" replace />;
  if (role && user.role !== role) return <Alert variant="destructive" role="alert">Bu sayfa için yetkin yok.</Alert>;
  return <>{children}</>;
}

/** NavLink bulunulan sayfada aria-current="page" koyar; görünümü de ona bağlı. */
function NavItem({ to, children }: { to: string; children: ReactNode }) {
  return (
    <NavLink
      to={to}
      className={({ isActive }) =>
        cn(buttonVariants({ variant: "ghost", size: "sm" }), "px-2 sm:px-3", isActive && "bg-accent text-accent-foreground")
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
      <header className="sticky top-0 z-20 border-b bg-background/85 backdrop-blur">
        <nav aria-label="Ana menü" className="mx-auto flex max-w-6xl flex-wrap items-center gap-1 px-4 py-3 sm:px-6">
          <Link to="/" className="mr-auto flex items-center gap-2 rounded-md font-serif text-lg font-semibold sm:text-xl">
            <BrandMark className="size-7 sm:size-8" />
            BookHub
          </Link>
          {user && <NavItem to="/suggest">Kitap öner</NavItem>}
          {user?.role === "ADMIN" && <NavItem to="/admin">Moderasyon</NavItem>}
          {user ? (
            <>
              {/* Dar ekranda ad gizli: menü iki satıra taşmasın (kullanıcı menüsü Faz C). */}
              <span className="hidden px-2 text-sm text-muted-foreground sm:inline">{user.display_name}</span>
              <NavItem to="/account">Hesabım</NavItem>
              <Button variant="ghost" size="sm" className="px-2 sm:px-3" onClick={logout}>Çıkış</Button>
            </>
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
      <main className="mx-auto w-full max-w-6xl flex-1 px-4 py-6 sm:px-6 sm:py-8">
        <Routes>
          <Route path="/" element={<Home />} />
          <Route path="/books/:id" element={<BookDetail />} />
          <Route path="/login" element={<Login />} />
          <Route path="/register" element={<Register />} />
          <Route path="/suggest" element={<RequireRole><Suggest /></RequireRole>} />
          <Route path="/account" element={<RequireRole><Account /></RequireRole>} />
          <Route path="/admin" element={<RequireRole role="ADMIN"><Moderation /></RequireRole>} />
          <Route path="*" element={<NotFound />} />
        </Routes>
      </main>
    </div>
  );
}
