import { BookX } from "lucide-react";
import { Link } from "react-router-dom";
import { EmptyState } from "./EmptyState";
import { Button } from "./ui/button";

/** Bilinmeyen adres ya da bulunamayan kitap: kullanıcıyı çıkmazda bırakma, kataloğa döndür. */
export function NotFound({ title = "Sayfa bulunamadı", children = "Aradığın sayfa yok ya da taşınmış olabilir." }: {
  title?: string;
  children?: string;
}) {
  return (
    <EmptyState
      icon={BookX}
      title={title}
      titleAs="h1"
      className="mx-auto mt-8 max-w-lg"
      action={
        <Button asChild>
          <Link to="/">Kataloğa dön</Link>
        </Button>
      }
    >
      {children}
    </EmptyState>
  );
}
