import { useState } from "react";
import { api } from "../api/client";
import { ErrorBox } from "../components/ui";

/** Kullanıcı önerisi: kitap PENDING doğar, admin onaylayana kadar listede görünmez. */
export function Suggest() {
  const [form, setForm] = useState({ title: "", author: "", description: "", year: "" });
  const [done, setDone] = useState<string | null>(null);
  const [error, setError] = useState<unknown>(null);
  const set = (k: keyof typeof form) => (e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) =>
    setForm({ ...form, [k]: e.target.value });

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    try {
      const b = await api.suggest({
        title: form.title,
        author: form.author,
        description: form.description || undefined,
        publication_year: form.year ? Number(form.year) : undefined,
      });
      setDone(`"${b.title}" önerildi — durum: ${b.status}. Admin onaylayınca listede görünecek.`);
      setForm({ title: "", author: "", description: "", year: "" });
    } catch (err) {
      setError(err);
    }
  };

  return (
    <form className="panel" onSubmit={submit}>
      <h1>Kitap öner</h1>
      <label>Başlık<input value={form.title} onChange={set("title")} required maxLength={300} /></label>
      <label>Yazar<input value={form.author} onChange={set("author")} required maxLength={200} /></label>
      <label>Yayın yılı<input type="number" value={form.year} onChange={set("year")} min={1450} /></label>
      <label>Açıklama<textarea value={form.description} onChange={set("description")} maxLength={5000} /></label>
      <ErrorBox error={error} />
      {done && <div className="success">{done}</div>}
      <button type="submit">Öner</button>
    </form>
  );
}
