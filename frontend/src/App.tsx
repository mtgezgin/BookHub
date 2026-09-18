import type { ReactNode } from "react";
import { Link, Navigate, Route, Routes } from "react-router-dom";
import { useAuth } from "./auth/AuthContext";
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
  if (!ready) return <p className="muted">Yükleniyor…</p>;
  if (!user) return <Navigate to="/login" replace />;
  if (role && user.role !== role) return <p className="error">Bu sayfa için yetkin yok.</p>;
  return <>{children}</>;
}

export function App() {
  const { user, logout } = useAuth();
  return (
    <>
      <nav className="topbar">
        <Link to="/" className="brand">📚 BookHub</Link>
        <div className="spacer" />
        {user && <Link to="/suggest">Kitap öner</Link>}
        {user?.role === "ADMIN" && <Link to="/admin">Moderasyon</Link>}
        {user ? (
          <>
            <span className="muted small">{user.display_name}</span>
            <button className="link" onClick={logout}>Çıkış</button>
          </>
        ) : (
          <>
            <Link to="/login">Giriş</Link>
            <Link to="/register">Kayıt</Link>
          </>
        )}
      </nav>
      <main>
        <Routes>
          <Route path="/" element={<Home />} />
          <Route path="/books/:id" element={<BookDetail />} />
          <Route path="/login" element={<Login />} />
          <Route path="/register" element={<Register />} />
          <Route path="/suggest" element={<RequireRole><Suggest /></RequireRole>} />
          <Route path="/admin" element={<RequireRole role="ADMIN"><Moderation /></RequireRole>} />
          <Route path="*" element={<p>Sayfa bulunamadı.</p>} />
        </Routes>
      </main>
    </>
  );
}
