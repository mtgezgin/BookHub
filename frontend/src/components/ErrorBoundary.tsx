import { Component, type ErrorInfo, type ReactNode } from "react";
import { Button } from "./ui/button";

interface State {
  failed: boolean;
}

/**
 * Hata sınırı: bir sayfa RENDER sırasında hata fırlatırsa React bütün ağacı söker ve ekran boş kalır
 * (REVIEW_FEEDBACK F34). Sınır hatayı yakalar; menü yerinde durur, içerik alanında ne olduğunu ve ne
 * yapılabileceğini söyleyen bir kutu görünür.
 *
 * Neleri yakalar: render, yaşam döngüsü ve tembel parça (lazy) hataları. Neleri YAKALAMAZ: olay
 * işleyicileri ve fetch hataları; onlar zaten ErrorBox ile gösteriliyor.
 *
 * Sınıf bileşeni: React'te hata yakalamanın (getDerivedStateFromError / componentDidCatch) hook
 * karşılığı yok. App, sınıra rota yolunu `key` olarak verir: başka sayfaya geçince sınır sıfırlanır.
 */
export class ErrorBoundary extends Component<{ children: ReactNode }, State> {
  state: State = { failed: false };

  static getDerivedStateFromError(): State {
    return { failed: true };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    // Yalnız konsola: hata mesajı kullanıcıya gösterilmez (iç ayrıntı taşıyabilir).
    console.error("Sayfa çizilirken hata:", error, info.componentStack);
  }

  render() {
    if (!this.state.failed) return this.props.children;
    return (
      <section role="alert" className="mx-auto max-w-md space-y-4 py-12 text-center">
        <h1 className="font-serif text-2xl font-semibold">Bir şeyler ters gitti</h1>
        <p className="text-muted-foreground">
          Bu sayfa gösterilirken beklenmeyen bir hata oluştu. Sayfayı yeniden yüklemek çoğu zaman sorunu giderir.
        </p>
        <div className="flex justify-center gap-2">
          <Button onClick={() => window.location.reload()}>Sayfayı yeniden yükle</Button>
          <Button asChild variant="outline">
            <a href="/">Ana sayfaya dön</a>
          </Button>
        </div>
      </section>
    );
  }
}
