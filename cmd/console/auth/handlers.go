package auth

import (
	"html/template"
	"net/http"
	"os"

	"github.com/intentd/inrun/console/web"
)

var (
	loginTpl = template.Must(template.ParseFS(web.Assets, "assets/templates/login.html"))

	inrun         = "inrun"
	username      = os.Getenv("ADMIN_USERNAME")
	password      = os.Getenv("ADMIN_PASSWORD")
	sessionSecret = []byte(os.Getenv("SESSION_SECRET"))
	inrunSession  = "inrun_session"
)

func applyDefaults() {
	if username == "" {
		username = inrun
	}
	if password == "" {
		password = inrun
	}
	if sessionSecret == nil {
		sessionSecret = []byte("dev-secret")
	}
}

func LoginPage(w http.ResponseWriter, r *http.Request) {
	loginTpl.Execute(w, nil)
}

func LoginPost(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	user := r.Form.Get("username")
	pass := r.Form.Get("password")

	applyDefaults()

	if user != username || pass != password {
		w.WriteHeader(http.StatusUnauthorized)
		loginTpl.Execute(w, map[string]string{"Error": "Invalid credentials"})
		return
	}

	token := signSession(user)

	http.SetCookie(w, &http.Cookie{
		Name:     inrunSession,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   false, // set true in production (HTTPS)
	})

	http.Redirect(w, r, "/console", http.StatusFound)
}

func Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     inrunSession,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})
	http.Redirect(w, r, "/", http.StatusFound)
}
