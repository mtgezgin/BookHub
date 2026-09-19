package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/mtgezgin/bookhub/backend/internal/auth"
	"github.com/mtgezgin/bookhub/backend/internal/config"
)

// authService, auth handler'larının servisten beklediği davranışlar (tüketen tarafta
// tanımlı). *auth.Service bunu sağlar.
type authService interface {
	Register(ctx context.Context, email, displayName, password string) (auth.User, error)
	Login(ctx context.Context, email, password string) (auth.Session, error)
	Refresh(ctx context.Context, refreshToken string) (auth.Session, error)
	Logout(ctx context.Context, refreshToken string) error
	Me(ctx context.Context, userID int64) (auth.User, error)
	ChangePassword(ctx context.Context, userID int64, current, next string) (auth.Session, error)
}

// userResponse, kullanıcının dışa açık temsili. password_hash'in burada olmaması tesadüf
// değil: auth.User tipinde o alan zaten yok (bkz. auth.User).
type userResponse struct {
	ID          int64     `json:"id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	Role        string    `json:"role"`
	CreatedAt   time.Time `json:"created_at"`
}

func toUserResponse(u auth.User) userResponse {
	return userResponse{ID: u.ID, Email: u.Email, DisplayName: u.DisplayName, Role: u.Role, CreatedAt: u.CreatedAt}
}

// sessionResponse, giriş/yenileme cevabı. İstemci access token'ı her istekte
// "Authorization: Bearer ..." olarak yollar; süresi dolunca /api/auth/refresh ile yenisini alır.
//
// refresh_token alanı BİLEREK YOK (Faz 13): uzun ömürlü sır artık YALNIZCA httpOnly çerezde
// taşınıyor, JS onu hiç göremiyor. Cevap gövdesinde de dönseydi çerez tamamen anlamsız olurdu —
// bir XSS betiği gövdeyi okuyup token'ı kendi sunucusuna yollayabilirdi. refresh_expires_at
// KALIYOR: sır değil, yalnız "oturum en geç ne zaman biter" bilgisi (arayüz uyarı gösterebilir).
type sessionResponse struct {
	AccessToken      string       `json:"access_token"`
	TokenType        string       `json:"token_type"`
	AccessExpiresAt  time.Time    `json:"access_expires_at"`
	RefreshExpiresAt time.Time    `json:"refresh_expires_at"`
	User             userResponse `json:"user"`
}

func toSessionResponse(s auth.Session) sessionResponse {
	return sessionResponse{
		AccessToken:      s.AccessToken,
		TokenType:        "Bearer",
		AccessExpiresAt:  s.AccessExpiresAt,
		RefreshExpiresAt: s.RefreshExpiresAt,
		User:             toUserResponse(s.User),
	}
}

// cookieSettings, refresh çerezinin yapılandırılabilir iki parçası (bkz. config.Config).
// deps üzerinden handler'lara geçer: testler farklı ad/Secure kombinasyonlarını env'e
// dokunmadan kurabilsin.
type cookieSettings struct {
	name   string
	secure bool
}

// cookieSettingsFrom, config'ten çerez ayarlarını üretir. NEDEN AYRI FONKSİYON: main.go'da
// satır içi bir literal olsaydı (cookieSettings{name: cfg.CookieName, ...}) bir alanın yanlış
// bağlanması ya da hiç bağlanmaması test EDİLEMEZDİ — main.go'yu hiçbir test çalıştırmıyor.
// Secure'un sessizce false kalması, 30 güne kadar yaşayabilen refresh sırrının düz http'de
// ağa çıkması demek; bu yüzden eşleme sınanabilir bir yere alındı.
func cookieSettingsFrom(cfg config.Config) cookieSettings {
	return cookieSettings{name: cfg.CookieName, secure: cfg.CookieSecure}
}

// refreshCookiePath, çerezin kapsamı. BİLEREK DAR: tarayıcı çerezi yalnızca /api/auth
// altındaki isteklere ekler — /api/books'a giden her istekte uzun ömürlü sırrı ağda
// dolaştırmanın bir faydası yok, riski var (ara loglar, proxy'ler, hata ayıklama çıktıları).
const refreshCookiePath = "/api/auth"

// maxRefreshTokenLen, çerezden kabul edeceğimiz en uzun değer. Bizim ürettiğimiz token 26
// karakter; sınır, uydurma dev bir çerezin servise/DB'ye kadar gitmesini engeller (ucuz kapı).
const maxRefreshTokenLen = 256

// newRefreshCookie, set ve silme çerezinin ORTAK iskeleti.
//
// NEDEN TEK YER: tarayıcı bir çerezi silerken ad/Path/Secure/SameSite üçlüsünün AYNI olmasını
// bekler (RFC 6265 §4.1.2: çerez kimliği ad + domain + path). Silme çerezi farklı bir Path'le
// yazılsaydı, tarayıcıda ikinci bir çerez oluşur, asıl çerez yaşamaya devam ederdi.
func newRefreshCookie(c cookieSettings, value string) *http.Cookie {
	return &http.Cookie{
		Name:  c.name,
		Value: value,
		Path:  refreshCookiePath,
		// HttpOnly: document.cookie bu çerezi GÖRMEZ. Faz 13'ün bütün amacı bu tek satır —
		// XSS'li bir betik artık 7 günlük sırrı okuyup dışarı sızdıramaz.
		HttpOnly: true,
		// Secure: çerez yalnız HTTPS ile gider (COOKIE_SECURE; localhost tarayıcılarca
		// "güvenli bağlam" sayıldığı için lokalde de true çalışır).
		Secure: c.secure,
		// SameSite=Strict: başka bir siteden BAŞLATILAN hiçbir istekte gönderilmez —
		// CSRF'e karşı crossOriginProtection'dan bağımsız İKİNCİ katman.
		SameSite: http.SameSiteStrictMode,
	}
}

// setRefreshCookie, oturumun refresh token'ını çereze yazar. Expires = oturumun GERÇEK
// (mutlak tavana kırpılmış) bitişi: tarayıcı o andan sonra çerezi kendiliğinden atar.
func setRefreshCookie(w http.ResponseWriter, c cookieSettings, s auth.Session) {
	ck := newRefreshCookie(c, s.RefreshToken)
	ck.Expires = s.RefreshExpiresAt
	http.SetCookie(w, ck)
}

// clearRefreshCookie, çerezi tarayıcıdan siler (MaxAge<0 → "Max-Age=0" → hemen sil).
func clearRefreshCookie(w http.ResponseWriter, c cookieSettings) {
	ck := newRefreshCookie(c, "")
	ck.MaxAge = -1
	http.SetCookie(w, ck)
}

// readRefreshCookie, istekteki refresh çerezini okur. İkinci dönüş false ise istek
// "çerezsiz" sayılır — DB'ye hiç gidilmez.
//
// r.Cookie DEĞİL r.CookiesNamed: r.Cookie aynı addaki İLK çerezi döndürür ve gerisini
// sessizce yutar. Aynı adla birden fazla çerez gelmesi normal bir durum değildir; tipik
// nedeni GÖLGELEME (cookie shadowing) girişimidir: saldırgan, alt alan adı ya da farklı bir
// Path üzerinden ikinci bir çerez yazdırıp kurbanın gerçek çerezini "gizler" — tarayıcı
// ikisini de gönderir, sunucu hangisinin gerçek olduğunu AYIRT EDEMEZ (istek başlığında
// Path/Domain bilgisi yoktur). Bu yüzden tahmin etmek yerine isteği reddediyoruz.
func readRefreshCookie(w http.ResponseWriter, r *http.Request, c cookieSettings) (string, bool) {
	cookies := r.CookiesNamed(c.name)
	if len(cookies) > 1 {
		slog.Warn("aynı adlı birden fazla refresh çerezi geldi, çerez yok sayıldı",
			"cookie_name", c.name,
			"cookie_count", len(cookies),
			"request_id", w.Header().Get(requestIDHeader),
		)
		return "", false
	}
	if len(cookies) == 0 {
		return "", false
	}
	v := cookies[0].Value
	if v == "" || len(v) > maxRefreshTokenLen {
		return "", false
	}
	return v, true
}

// registerRequest: role alanı YOK (mass-assignment) — `{"role":"ADMIN"}` yollayan istek
// DisallowUnknownFields sayesinde 400 alır; alan olsaydı bile servis imzası rol almıyor.
type registerRequest struct {
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Password    string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// handleRegister, POST /api/auth/register → 201 + kullanıcı.
func handleRegister(svc authService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req registerRequest
		if err := readJSON(w, r, &req); err != nil {
			writeRequestError(w, err)
			return
		}

		u, err := svc.Register(r.Context(), req.Email, req.DisplayName, req.Password)
		var ve *auth.ValidationError
		switch {
		case errors.As(err, &ve):
			writeError(w, http.StatusBadRequest, "validation_error", ve.Error())
		// 409 e-postanın kayıtlı olduğunu söyler — login'in aksine burada bilinçli bir
		// ödünleşim: aksi hâlde kullanıcı neden kayıt olamadığını anlayamaz. Toplu
		// sorgulamaya karşı savunma rate limit (Faz 7). E-posta doğrulamalı akışta
		// ("bağlantı gönderdik") bu sızıntı da kapanır.
		case errors.Is(err, auth.ErrEmailTaken):
			writeError(w, http.StatusConflict, "email_taken", "bu e-posta zaten kayıtlı")
		case err != nil:
			writeInternalError(w, "Register", err)
		default:
			writeJSON(w, http.StatusCreated, toUserResponse(u))
		}
	}
}

// handleLogin, POST /api/auth/login → 200 + oturum (refresh token'ı httpOnly çerezde).
// Hatalı kimlikte TEK mesaj.
func handleLogin(svc authService, cookies cookieSettings) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req loginRequest
		if err := readJSON(w, r, &req); err != nil {
			writeRequestError(w, err)
			return
		}
		var v validator
		v.required("email", req.Email)
		v.required("password", req.Password)
		if err := v.err(); err != nil {
			writeRequestError(w, err)
			return
		}

		// Tarayıcıda zaten bir oturum çerezi varsa: yeni giriş onu EZECEK ve o ailenin
		// token'ı bir daha hiçbir istemciye ulaşmayacak — ama DB'de CANLI kalırdı (yetim
		// oturum). Kim olduğunu bilmediğimiz için değeri giriş cevabını yazmadan ÖNCE okuruz;
		// ezildikten sonra artık okunamaz.
		oldToken, hadOld := readRefreshCookie(w, r, cookies)

		sess, err := svc.Login(r.Context(), req.Email, req.Password)
		switch {
		case errors.Is(err, auth.ErrInvalidCredentials):
			writeError(w, http.StatusUnauthorized, "invalid_credentials", "e-posta veya parola hatalı")
		case err != nil:
			writeInternalError(w, "Login", err)
		default:
			// EN İYİ ÇABA: iptal başarısız olsa da giriş başarılıdır. Kullanıcıyı, ona ait
			// OLMAYABİLECEK eski bir çerezin DB hatası yüzünden dışarıda bırakmak yanlış olurdu.
			// (Yalnız başarılı girişte: yanlış parolayla gelen bir istek başkasının oturumunu
			// öldürememeli.)
			if hadOld {
				if err := svc.Logout(r.Context(), oldToken); err != nil {
					slog.Warn("giriş sırasında önceki oturum iptal edilemedi",
						"err", err, "request_id", w.Header().Get(requestIDHeader))
				}
			}
			setRefreshCookie(w, cookies, sess)
			writeJSON(w, http.StatusOK, toSessionResponse(sess))
		}
	}
}

// handleRefresh, POST /api/auth/refresh → 200 + YENİ oturum (eski refresh token ölür).
//
// GÖVDE OKUNMAZ: token yalnızca çerezden gelir. Gövdeyi de kabul etseydik httpOnly'nin
// anlamı kalmazdı — XSS'li bir betik çaldığı değeri gövdede yollayabilirdi.
func handleRefresh(svc authService, cookies cookieSettings) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, ok := readRefreshCookie(w, r, cookies)
		if !ok {
			// Çerez yok/boş/çoklu/aşırı uzun: servise hiç gitmeden 401. Silme çerezi, tarayıcıda
			// duran bozuk/eskimiş bir çerezin her istekte aynı 401'i üretmesini engeller.
			clearRefreshCookie(w, cookies)
			writeError(w, http.StatusUnauthorized, "invalid_token", "geçersiz ya da süresi dolmuş refresh token")
			return
		}
		sess, err := svc.Refresh(r.Context(), token)
		switch {
		case errors.Is(err, auth.ErrInvalidToken):
			// Kullanılmış bir token yeniden geldi (reuse): servis ailenin tamamını çoktan öldürdü.
			// Bu bir GÜVENLİK OLAYI (token büyük olasılıkla çalındı) → operatör için WARN. Ama
			// istemciye giden cevap düz "geçersiz token"la BİREBİR aynı: ayrı bir kod/mesaj,
			// saldırgana "bu token'ı biri zaten kullanmış" diyen bir kâhin (oracle) olurdu. Aynı
			// dala düşmesi (ReuseError ErrInvalidToken'ı sarar) bu eşitliği yapısal olarak garanti
			// eder. Token'ın kendisi ASLA loglanmaz — kimlik (user/family) ve request_id yeter.
			var reuse *auth.ReuseError
			if errors.As(err, &reuse) {
				slog.Warn("refresh token yeniden kullanıldı, oturum ailesi iptal edildi",
					"user_id", reuse.UserID,
					"family_id", reuse.FamilyID,
					"request_id", w.Header().Get(requestIDHeader),
				)
			}
			// Geçersiz token = tarayıcıdaki çerez ölü. Silme çerezi olmasaydı istemci her
			// açılışta aynı ölü çerezle bir 401 daha üretirdi.
			clearRefreshCookie(w, cookies)
			writeError(w, http.StatusUnauthorized, "invalid_token", "geçersiz ya da süresi dolmuş refresh token")
		case err != nil:
			// 500'de çerezi SİLMİYORUZ: DB'nin geçici arızası kullanıcıyı oturumdan etmemeli
			// (token büyük olasılıkla hâlâ geçerli). İstemci sonra yeniden dener.
			writeInternalError(w, "Refresh", err)
		default:
			// Rotasyon: eski token öldü, yenisi çereze yazılır. Aynı cevapta Set-Cookie olmazsa
			// tarayıcıda ARTIK GEÇERSİZ olan eski değer kalır ve bir sonraki yenileme reuse
			// sayılıp tüm aileyi öldürürdü.
			setRefreshCookie(w, cookies, sess)
			writeJSON(w, http.StatusOK, toSessionResponse(sess))
		}
	}
}

// logoutRevokeBackoff, çıkıştaki iptal denemeleri arasındaki bekleme süreleri (yani ilk
// denemeden sonra 2 deneme daha).
//
// NEDEN SUNUCU KENDİ KENDİNİ ONARIYOR: aileyi öldürebilecek tek anahtar refresh token'ın
// kendisi ve o an ona sahip olan İKİ taraf var — tarayıcıdaki httpOnly çerez ve elimizdeki
// bu değişken. İstemci token'ı GÖREMEZ (JS'e kapalı), yalnızca "isteği tekrar at" diyebilir;
// biz ise anında, bedavaya yeniden deneyebiliriz. Zaten context.WithoutCancel ile koşuyoruz,
// yani istemci kopsa bile deneme sürer. Toplam bekleme (1,2 sn) aşağıdaki 5 sn'lik bütçenin
// içinde kalır; DB'nin bir saniyelik sarsıntısı çoğu zaman burada yutulur.
//
// Sabit değil DEĞİŞKEN: handler testleri süreleri sıfırlayıp boşuna saniyelerce beklemesin.
var logoutRevokeBackoff = []time.Duration{200 * time.Millisecond, time.Second}

// sleepCtx, d kadar bekler ama ctx biterse ERKEN döner. Dönüş: bekleme tamamlandı mı
// (false = bütçe doldu, yeniden denemenin anlamı yok). Düz time.Sleep, 5 sn'lik timeout
// dolduktan sonra bile beklemeye devam ederdi.
func sleepCtx(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// handleLogout, POST /api/auth/logout → 204; çerezdeki token'ın AİLESİ (o girişin tüm
// token'ları) ölür. Token'ın sahibi olmak yeterli yetki: refresh token'ı bilen zaten o
// oturumu yenileyebilir, öldürebilmesi de doğal.
func handleLogout(svc authService, cookies cookieSettings) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, ok := readRefreshCookie(w, r, cookies)
		if !ok {
			// Çerezsiz çıkış geçerli bir istektir (istemci zaten düşmüş bir oturumu
			// temizliyordur): 204, sessizce. Silme çerezini yine de yazıyoruz: BOZUK bir
			// çerez (boş, çoklu, aşırı uzun) tarayıcıda kalırsa her istekte aynı yolu
			// yeniden tetikler; öldürebileceğimiz bir aile de yok.
			clearRefreshCookie(w, cookies)
			w.WriteHeader(http.StatusNoContent)
			return
		}

		// context.WithoutCancel: iptal işlemi, isteğin context'ine BAĞLI OLMAMALI. Tarayıcı
		// sekmesi kapanırken atılan bir çıkış isteğinde bağlantı çoğu zaman cevap dönmeden
		// kopar; r.Context() iptal olunca DB sorgusu yarıda kesilir ve oturum DB'de canlı
		// kalırdı — yani "çıkış yaptım" diyen kullanıcının token'ı yaşamaya devam ederdi.
		// Bağımsız 5 sn'lik timeout: context tamamen süresiz kalmasın (sızan goroutine yok).
		ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 5*time.Second)
		defer cancel()

		var err error
		attempts := 0
		for {
			attempts++
			if err = svc.Logout(ctx, token); err == nil || attempts > len(logoutRevokeBackoff) {
				break
			}
			if !sleepCtx(ctx, logoutRevokeBackoff[attempts-1]) {
				break // bütçe doldu: elimizdeki hatayla devam
			}
		}

		if err != nil {
			// ÇEREZİ SİLMİYORUZ — handleRefresh'in 500'deki duruşunun aynısı, ama gerekçe
			// daha sert: aile DB'de CANLI kaldı ve onu öldürebilecek tek anahtar tarayıcıdaki
			// bu çerez. Silseydik istemcinin LOGOUT_PENDING yeniden denemesi ÇEREZSİZ giderdi;
			// sunucu hiçbir şey iptal etmeden 204 der, istemci "çıkış tamam" sanıp bayrağı
			// silerdi ve aile mutlak tavana (JWT_SESSION_MAX_AGE) kadar yaşardı — çalınmış bir
			// çerez hâlâ işe yarardı. Yani iptal ASLA gerçekleşemezdi.
			// ÖDÜNLEŞİM (bilerek): DB çökmüşken yapılan çıkış, o tarayıcıda hâlâ GEÇERLİ bir
			// refresh çerezi bırakır (HttpOnly, Path=/api/auth, SameSite=Strict). Yerel arayüz
			// zaten kapandı (HINT silindi); istemcinin yeniden denemeleri ve açılıştaki
			// api.restore() denemesi artık gerçekten iptal edebilecek bir anahtarla gider.
			//
			// writeInternalError zaten "istek başarısız" satırı yazıyor; bu AYRI satır
			// operatör içindir: ortada çıkıştan SAĞ ÇIKMIŞ bir oturum var. Token ASLA loglanmaz.
			slog.Error("çıkışta oturum ailesi iptal edilemedi, oturum çıkıştan sağ çıktı",
				"err", err,
				"attempts", attempts,
				"request_id", w.Header().Get(requestIDHeader),
			)
			writeInternalError(w, "Logout", err)
			return
		}
		// Aile öldü: çerez artık ölü bir sırrı taşıyor, tarayıcıdan da sil.
		clearRefreshCookie(w, cookies)
		w.WriteHeader(http.StatusNoContent)
	}
}

// handleMe, GET /api/auth/me (requireAuth arkasında) → token sahibinin güncel bilgisi.
func handleMe(svc authService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := principalFrom(r.Context())
		if !ok {
			// requireAuth'suz bağlanmış bir rota = programlama hatası. Kapalı başarısız ol.
			unauthorized(w, "missing_token", "kimlik doğrulaması gerekli")
			return
		}
		u, err := svc.Me(r.Context(), p.UserID)
		switch {
		case errors.Is(err, auth.ErrUserNotFound):
			// Token geçerli ama hesap silinmiş: kimlik artık yok.
			unauthorized(w, "invalid_token", "bu token'ın kullanıcısı artık yok")
		case err != nil:
			writeInternalError(w, "Me", err)
		default:
			writeJSON(w, http.StatusOK, toUserResponse(u))
		}
	}
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// handleChangePassword, POST /api/auth/password (requireAuth + rateLimit "password" arkasında)
// → 200 + YENİ oturum (yeni refresh çerezi); kullanıcının diğer TÜM oturumları ölür.
//
// Yanlış mevcut parola 403 invalid_current_password — 401 DEĞİL. İstemci (client.ts) 401'i
// "access token bitti" sayıp /refresh'e gider ve isteği yeniler; yanlış parola için de aynı
// şeyi yapsaydı kullanıcı hatasını hiç göremez, boşuna bir rotasyon harcanırdı. 403 ise
// "kimliğin belli ama BU isteğe iznin yok" — mevcut parolayı bilmeyen birine tam olarak bu denir.
func handleChangePassword(svc authService, cookies cookieSettings) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := principalFrom(r.Context())
		if !ok {
			unauthorized(w, "missing_token", "kimlik doğrulaması gerekli")
			return
		}
		var req changePasswordRequest
		if err := readJSON(w, r, &req); err != nil {
			writeRequestError(w, err)
			return
		}
		var v validator
		v.required("current_password", req.CurrentPassword)
		v.required("new_password", req.NewPassword)
		if err := v.err(); err != nil {
			writeRequestError(w, err)
			return
		}

		sess, err := svc.ChangePassword(r.Context(), p.UserID, req.CurrentPassword, req.NewPassword)
		var ve *auth.ValidationError
		switch {
		case errors.As(err, &ve):
			writeError(w, http.StatusBadRequest, "validation_error", ve.Error())
		case errors.Is(err, auth.ErrWrongPassword):
			writeError(w, http.StatusForbidden, "invalid_current_password", "mevcut parola hatalı")
		case errors.Is(err, auth.ErrUserNotFound):
			// Token geçerli ama hesap silinmiş (handleMe ile aynı duruş).
			unauthorized(w, "invalid_token", "bu token'ın kullanıcısı artık yok")
		case err != nil:
			writeInternalError(w, "ChangePassword", err)
		default:
			// Bu cihazın eski ailesi de az önce öldü; yeni çerez olmasa tarayıcıda ölü bir
			// çerez kalır ve bir sonraki yenileme 401 ile oturumu düşürürdü.
			setRefreshCookie(w, cookies, sess)
			writeJSON(w, http.StatusOK, toSessionResponse(sess))
		}
	}
}
