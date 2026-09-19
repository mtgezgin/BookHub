import { useState } from "react";
import { api } from "../api/client";
import { useAuth } from "../auth/AuthContext";
import { ErrorBox } from "../components/ui";

const EMPTY = { current: "", next: "", repeat: "" };

/**
 * Hesabım: parola değiştirme (Faz 13). Sunucu, değişimle birlikte kullanıcının DİĞER tüm
 * cihazlardaki oturumlarını kapatır; bu cihaz yeni bir oturumla devam eder (api.changePassword).
 */
export function Account() {
  const { user } = useAuth();
  const [form, setForm] = useState(EMPTY);
  const [error, setError] = useState<unknown>(null);
  const [mismatch, setMismatch] = useState(false);
  const [done, setDone] = useState(false);
  const [busy, setBusy] = useState(false);
  const set = (k: keyof typeof form) => (e: React.ChangeEvent<HTMLInputElement>) => setForm({ ...form, [k]: e.target.value });

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setDone(false);
    // Tekrar alanı SUNUCUYA GİTMEZ: uyuşmazlık burada yakalanır, istek hiç atılmaz (yanlış
    // yazılmış bir parola kaydedilip kullanıcıyı her cihazdan düşürmesin).
    if (form.next !== form.repeat) {
      setMismatch(true);
      return;
    }
    setMismatch(false);
    setBusy(true);
    try {
      await api.changePassword(form.current, form.next);
      setForm(EMPTY); // parolalar ekranda kalmasın
      setDone(true);
    } catch (err) {
      setError(err); // 403 "mevcut parola hatalı", 400 politika mesajı — backend'in metni
    } finally {
      setBusy(false);
    }
  };

  return (
    <form className="panel" onSubmit={submit}>
      <h1>Hesabım</h1>
      {user && <p className="muted small">{user.display_name} · {user.email}</p>}
      <h2>Parola değiştir</h2>
      <label>Mevcut parola
        <input type="password" value={form.current} onChange={set("current")} required autoComplete="current-password" />
      </label>
      <label>Yeni parola <span className="muted small">(en az 8 karakter)</span>
        <input type="password" value={form.next} onChange={set("next")} required minLength={8} autoComplete="new-password" />
      </label>
      <label>Yeni parola (tekrar)
        <input type="password" value={form.repeat} onChange={set("repeat")} required autoComplete="new-password" />
      </label>
      {mismatch && <div className="error" role="alert">Yeni parolalar birbiriyle eşleşmiyor.</div>}
      <ErrorBox error={error} />
      {done && <div className="success" role="status">Parolan değişti. Diğer tüm cihazlardaki oturumlar kapatıldı.</div>}
      <button type="submit" disabled={busy}>Parolayı değiştir</button>
    </form>
  );
}
