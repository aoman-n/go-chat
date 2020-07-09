package main

import (
	"flag"
	"fmt"
	"go-chat/trace"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/joho/godotenv"
	"github.com/stretchr/gomniauth"
	"github.com/stretchr/gomniauth/providers/google"
	"github.com/stretchr/objx"
)

type templateHandler struct {
	once     sync.Once
	filename string
	templ    *template.Template
}

func (t *templateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	t.once.Do(func() {
		t.templ = template.Must(
			template.ParseFiles(filepath.Join("templates", t.filename)),
		)
	})
	data := map[string]interface{}{
		"Host": r.Host,
	}
	if authCookie, err := r.Cookie("auth"); err == nil {
		data["UserData"] = objx.MustFromBase64(authCookie.Value)
	}

	t.templ.Execute(w, data)
}

func init() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
}

// 内部状態を保持する必要があるときはhttp.Handlerのインターフェースを実装しているstructを定義 => http.Handleを使う
// 内部状態を保持する必要がない場合には、func (http.ResponseWriter, *http.Request){}のファンクションを定義する => http.HandleFuncを使う
func main() {
	fmt.Println("id: ", os.Getenv("GOOGLE_CLIENT_ID"))
	fmt.Println("secret: ", os.Getenv("GOOGLE_CLIENT_SECRET"))

	// 引数でportを指定できるようにする
	var addr = flag.String("addr", ":8080", "application address")
	flag.Parse()
	var baseUrl = "http://localhost" + *addr

	// oauthパッケージのセットアップ
	// TODO: security key はランダムな値にする
	gomniauth.SetSecurityKey("Security Key")
	gomniauth.WithProviders(
		google.New(
			os.Getenv("GOOGLE_CLIENT_ID"),
			os.Getenv("GOOGLE_CLIENT_SECRET"),
			fmt.Sprintf("%s/auth/callback/google", baseUrl),
		),
	)

	r := newRoom(UseGravatarAvatar)
	r.tracer = trace.New(os.Stdout)

	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))
	// chat用のrouting
	http.Handle("/chat", MustAuth(&templateHandler{filename: "chat.html"}))
	http.Handle("/login", &templateHandler{filename: "login.html"})
	http.HandleFunc("/auth/", loginHandler)
	http.HandleFunc("/logout", logoutHandler)
	http.Handle("/room", r)

	go r.run()

	log.Println("start server. port", *addr)
	if err := http.ListenAndServe(*addr, nil); err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:   "auth",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})
	w.Header()["Location"] = []string{"/chat"}
	w.WriteHeader(http.StatusTemporaryRedirect)
}
