import { useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { api } from "../api/client";
import { useAuth } from "../auth/AuthContext";
import { ErrorBox } from "../components/ui";

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
    <form className="panel" onSubmit={submit}>
      <h1>Giriş yap</h1>
      <label>E-posta<input type="email" value={email} onChange={(e) => setEmail(e.target.value)} required autoComplete="email" /></label>
      <label>Parola<input type="password" value={password} onChange={(e) => setPassword(e.target.value)} required autoComplete="current-password" /></label>
      <ErrorBox error={error} />
      <button type="submit">Giriş</button>
      <p className="muted small">Hesabın yok mu? <Link to="/register">Kayıt ol</Link></p>
    </form>
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
    <form className="panel" onSubmit={submit}>
      <h1>Kayıt ol</h1>
      <label>E-posta<input type="email" value={form.email} onChange={set("email")} required autoComplete="email" /></label>
      <label>Görünen ad<input value={form.name} onChange={set("name")} required maxLength={100} /></label>
      <label>Parola <span className="muted small">(en az 8 karakter)</span>
        <input type="password" value={form.password} onChange={set("password")} required minLength={8} autoComplete="new-password" />
      </label>
      <ErrorBox error={error} />
      <button type="submit">Kayıt ol</button>
    </form>
  );
}
