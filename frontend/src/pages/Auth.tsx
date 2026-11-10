import { useId, useState } from "react";
import { Link, useLocation, useNavigate } from "react-router-dom";
import { api } from "../api/client";
import { useAuth } from "../auth/AuthContext";
import { ErrorBox, PageTitle } from "../components/common";
import { PasswordField } from "../components/PasswordField";
import { Alert } from "../components/ui/alert";
import { Button } from "../components/ui/button";
import { Card } from "../components/ui/card";
import { Input } from "../components/ui/input";
import { Label } from "../components/ui/label";

const PANEL = "mx-auto mt-4 w-full max-w-md p-6 sm:mt-10 sm:p-8";
const LINK = "font-medium text-primary underline-offset-4 hover:underline";

/**
 * Girişten sonra gidilecek yer: korumalı sayfa ya da "giriş yap" bağlantısı geldiği adresi
 * router state'iyle (`from`) taşır. Yalnız uygulama içi yol kabul edilir: "//site" tarayıcıda
 * başka bir siteye gider (açık yönlendirme), ona düşerse ana sayfa.
 */
function useReturnTo(): string {
  const from = (useLocation().state as { from?: { pathname?: string; search?: string } } | null)?.from;
  const path = from?.pathname ?? "";
  if (!path.startsWith("/") || path.startsWith("//") || path.startsWith("/\\")) return "/";
  return path + (from?.search ?? "");
}

export function Login() {
  const { login } = useAuth();
  const nav = useNavigate();
  const { state } = useLocation();
  const returnTo = useReturnTo();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setBusy(true);
    try {
      await login(email, password);
      nav(returnTo, { replace: true });
    } catch (err) {
      setError(err); // "e-posta veya parola hatalı" — backend hangisinin yanlış olduğunu söylemez
      setBusy(false);
    }
  };

  return (
    <Card className={PANEL}>
      <PageTitle title="Giriş yap" />
      <form onSubmit={submit} className="flex flex-col gap-4">
        <h1 className="text-2xl font-semibold">Giriş yap</h1>
        <Label>E-posta<Input type="email" value={email} onChange={(e) => setEmail(e.target.value)} required autoComplete="email" /></Label>
        <PasswordField label="Parola" value={password} onChange={(e) => setPassword(e.target.value)} required autoComplete="current-password" />
        <ErrorBox error={error} />
        <Button type="submit" disabled={busy}>{busy ? "Giriş yapılıyor…" : "Giriş"}</Button>
        <p className="text-sm text-muted-foreground">
          Hesabın yok mu? <Link to="/register" state={state} className={LINK}>Kayıt ol</Link>
        </p>
      </form>
    </Card>
  );
}

/**
 * Demo uyarısı (#64): site gerçek kullanıcıya açık değil (ROADMAP "Hedef"), ama kayıt herkese açık.
 * Formdan önce durur; parola alanı da aria-describedby ile buna bağlı, yani ekran okuyucu alana
 * gelince uyarıyı yeniden okur. role="note": canlı bölge (alert/status) değil, sayfa açılınca bağırmaz.
 */
const DEMO_NOTICE =
  "Bu site bir öğrenme projesinin demosu. Başka bir yerde kullandığın parolayı kullanma: e-posta adresi doğrulanmaz ve veriler her an silinebilir.";

export function Register() {
  const { login } = useAuth();
  const noticeId = useId();
  const nav = useNavigate();
  const { state } = useLocation();
  const returnTo = useReturnTo();
  const [form, setForm] = useState({ email: "", name: "", password: "" });
  const [error, setError] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);
  const set = (k: keyof typeof form) => (e: React.ChangeEvent<HTMLInputElement>) => setForm({ ...form, [k]: e.target.value });

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setBusy(true);
    try {
      await api.register(form.email, form.name, form.password);
      await login(form.email, form.password);
      nav(returnTo, { replace: true });
    } catch (err) {
      setError(err);
      setBusy(false);
    }
  };

  return (
    <Card className={PANEL}>
      <PageTitle title="Kayıt ol" />
      <form onSubmit={submit} className="flex flex-col gap-4">
        <h1 className="text-2xl font-semibold">Kayıt ol</h1>
        <Alert id={noticeId} role="note" className="text-muted-foreground">{DEMO_NOTICE}</Alert>
        <Label>E-posta<Input type="email" value={form.email} onChange={set("email")} required autoComplete="email" /></Label>
        <Label>Görünen ad<Input value={form.name} onChange={set("name")} required maxLength={100} /></Label>
        <PasswordField label="Parola" hint="(en az 8 karakter)" value={form.password} onChange={set("password")} required minLength={8} autoComplete="new-password" aria-describedby={noticeId} />
        <ErrorBox error={error} />
        <Button type="submit" disabled={busy}>{busy ? "Kaydediliyor…" : "Kayıt ol"}</Button>
        <p className="text-sm text-muted-foreground">
          Zaten hesabın var mı? <Link to="/login" state={state} className={LINK}>Giriş yap</Link>
        </p>
      </form>
    </Card>
  );
}
