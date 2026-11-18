# Yedek anahtarı

`recipients.txt` bu klasörde durur ve yedeklerin şifrelendiği **açık** `age` anahtarını içerir
(`age1…` ile başlayan tek satır). Açık anahtar gizli değildir: yalnız şifrelemeye yarar.

Yedeği **açan** gizli anahtar (`AGE-SECRET-KEY-1…`) bu repoda, GitHub'da ya da herhangi bir sohbette
durmaz. Yalnız proje sahibinin parola yöneticisinde durur. Kaybolursa eski yedekler açılamaz; yeni
bir anahtar çifti üretilip bu dosya güncellenir.

Kurulum, döküm ve geri yükleme: [docs/DEPLOYMENT.md](../../docs/DEPLOYMENT.md) "Yedek".
