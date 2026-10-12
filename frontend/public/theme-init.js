// İlk boyamadan ÖNCE temayı uygular (index.html <head>'inde, eşzamanlı). CSP satır içi betiğe
// izin vermediği için ayrı dosya. Kural src/lib/theme.ts ile aynı: kullanıcının seçimi, yoksa
// sistem tercihi. Bu dosya olmasaydı koyu temayı seçen kullanıcı her açılışta bir an açık tema
// görürdü.
(function () {
  var theme = null;
  try {
    theme = localStorage.getItem("bookhub.theme");
  } catch (e) {
    // localStorage yasaklı olabilir
  }
  if (theme !== "light" && theme !== "dark") {
    theme = window.matchMedia && window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
  }
  document.documentElement.setAttribute("data-theme", theme);
})();
