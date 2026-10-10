import { useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { api } from "../api/client";
import { useAuth } from "../auth/AuthContext";
import { ErrorBox } from "../components/common";
import { Button } from "../components/ui/button";
import { Card } from "../components/ui/card";
import { Input } from "../components/ui/input";
import { Label } from "../components/ui/label";

const PANEL = "mx-auto mt-4 w-full max-w-md p-6 sm:mt-10 sm:p-8";
const LINK = "font-medium text-primary underline-offset-4 hover:underline";

export function Login() {
  const { login } = useAuth();
  const nav = useNavigate();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<unknown>(null);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    try {
      await login(email, password);
      nav("/");
    } catch (err) {
      setError(err); // "e-posta veya parola hatalı" — backend hangisinin yanlış olduğunu söylemez
    }
  };

  return (
    <Card className={PANEL}>
      <form onSubmit={submit} className="flex flex-col gap-4">
        <h1 className="text-2xl font-semibold">Giriş yap</h1>
        <Label>E-posta<Input type="email" value={email} onChange={(e) => setEmail(e.target.value)} required autoComplete="email" /></Label>
        <Label>Parola<Input type="password" value={password} onChange={(e) => setPassword(e.target.value)} required autoComplete="current-password" /></Label>
        <ErrorBox error={error} />
        <Button type="submit">Giriş</Button>
        <p className="text-sm text-muted-foreground">Hesabın yok mu? <Link to="/register" className={LINK}>Kayıt ol</Link></p>
      </form>
    </Card>
  );
}

export function Register() {
  const { login } = useAuth();
  const nav = useNavigate();
  const [form, setForm] = useState({ email: "", name: "", password: "" });
  const [error, setError] = useState<unknown>(null);
  const set = (k: keyof typeof form) => (e: React.ChangeEvent<HTMLInputElement>) => setForm({ ...form, [k]: e.target.value });

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    try {
      await api.register(form.email, form.name, form.password);
      await login(form.email, form.password);
      nav("/");
    } catch (err) {
      setError(err);
    }
  };

  return (
    <Card className={PANEL}>
      <form onSubmit={submit} className="flex flex-col gap-4">
        <h1 className="text-2xl font-semibold">Kayıt ol</h1>
        <Label>E-posta<Input type="email" value={form.email} onChange={set("email")} required autoComplete="email" /></Label>
        <Label>Görünen ad<Input value={form.name} onChange={set("name")} required maxLength={100} /></Label>
        <Label>
          <span>Parola <span className="font-normal text-muted-foreground">(en az 8 karakter)</span></span>
          <Input type="password" value={form.password} onChange={set("password")} required minLength={8} autoComplete="new-password" />
        </Label>
        <ErrorBox error={error} />
        <Button type="submit">Kayıt ol</Button>
        <p className="text-sm text-muted-foreground">Zaten hesabın var mı? <Link to="/login" className={LINK}>Giriş yap</Link></p>
      </form>
    </Card>
  );
}
