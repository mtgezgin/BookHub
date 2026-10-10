import { useState } from "react";
import { api } from "../api/client";
import { ErrorBox } from "../components/common";
import { Alert } from "../components/ui/alert";
import { Button } from "../components/ui/button";
import { Card } from "../components/ui/card";
import { Input, Textarea } from "../components/ui/input";
import { Label } from "../components/ui/label";

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
      setDone(`"${b.title}" incelemeye gönderildi. Bir yönetici onaylayınca katalogda görünecek.`);
      setForm({ title: "", author: "", description: "", year: "" });
    } catch (err) {
      setError(err);
    }
  };

  return (
    <Card className="mx-auto mt-4 w-full max-w-lg p-6 sm:mt-10 sm:p-8">
      <form onSubmit={submit} className="flex flex-col gap-4">
        <header>
          <h1 className="text-2xl font-semibold">Kitap öner</h1>
          <p className="mt-1 text-sm text-muted-foreground">Önerin bir yönetici onaylayınca katalogda görünür.</p>
        </header>
        <Label>Başlık<Input value={form.title} onChange={set("title")} required maxLength={300} /></Label>
        <Label>Yazar<Input value={form.author} onChange={set("author")} required maxLength={200} /></Label>
        <Label>Yayın yılı<Input type="number" value={form.year} onChange={set("year")} min={1450} className="w-32" /></Label>
        <Label>Açıklama<Textarea value={form.description} onChange={set("description")} maxLength={5000} /></Label>
        <ErrorBox error={error} />
        {done && <Alert variant="success" role="status">{done}</Alert>}
        <Button type="submit">Öner</Button>
      </form>
    </Card>
  );
}
