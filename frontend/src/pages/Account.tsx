import { useState } from "react";
import { api } from "../api/client";
import { useAuth } from "../auth/AuthContext";
import { ErrorBox } from "../components/common";
import { Alert } from "../components/ui/alert";
import { Button } from "../components/ui/button";
import { Card } from "../components/ui/card";
import { Input } from "../components/ui/input";
import { Label } from "../components/ui/label";

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
    <Card className="mx-auto mt-4 w-full max-w-md p-6 sm:mt-10 sm:p-8">
      <form onSubmit={submit} className="flex flex-col gap-4">
        <header>
          <h1 className="text-2xl font-semibold">Hesabım</h1>
          {user && <p className="mt-1 text-sm text-muted-foreground">{user.display_name} · {user.email}</p>}
        </header>
        <h2 className="mt-2 text-lg font-semibold">Parola değiştir</h2>
        <Label>Mevcut parola
          <Input type="password" value={form.current} onChange={set("current")} required autoComplete="current-password" />
        </Label>
        <Label>
          <span>Yeni parola <span className="font-normal text-muted-foreground">(en az 8 karakter)</span></span>
          <Input type="password" value={form.next} onChange={set("next")} required minLength={8} autoComplete="new-password" />
        </Label>
        <Label>Yeni parola (tekrar)
          <Input type="password" value={form.repeat} onChange={set("repeat")} required autoComplete="new-password" />
        </Label>
        {mismatch && <Alert variant="destructive" role="alert">Yeni parolalar birbiriyle eşleşmiyor.</Alert>}
        <ErrorBox error={error} />
        {done && <Alert variant="success" role="status">Parolan değişti. Diğer tüm cihazlardaki oturumlar kapatıldı.</Alert>}
        <Button type="submit" disabled={busy}>Parolayı değiştir</Button>
      </form>
    </Card>
  );
}
