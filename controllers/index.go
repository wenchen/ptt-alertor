package controllers

import (
	"html/template"
	"net/http"
	"os"
	"strconv"

	"strings"

	"github.com/wenchen/ptt-alertor/connections"
	"github.com/wenchen/ptt-alertor/models/counter"
	"github.com/wenchen/ptt-alertor/models/top"
	"github.com/wenchen/ptt-alertor/myutil"
	"github.com/wenchen/ptt-alertor/shorturl"
	"github.com/garyburd/redigo/redis"
	"github.com/julienschmidt/httprouter"
	"golang.org/x/net/websocket"
)

var tpls = []string{
	"public/docs.html",
	"public/top.html",
	"public/telegram.html",
	"public/messenger.html",
	"public/tpls/head.tpl",
	"public/tpls/header.tpl",
	"public/tpls/slogan.tpl",
	"public/tpls/command.tpl",
	"public/tpls/counter.tpl",
	"public/tpls/footer.tpl",
	"public/tpls/script.tpl",
}

func parseTemplates() *template.Template {
	prefix := ""
	if _, err := os.Stat("public"); os.IsNotExist(err) {
		if _, err := os.Stat("../public"); err == nil {
			prefix = "../"
		}
	}
	files := make([]string, len(tpls))
	for i, f := range tpls {
		files[i] = prefix + f
	}
	return template.Must(template.ParseFiles(files...))
}

var (
	templates = parseTemplates()
	wsHost    = os.Getenv("APP_WS_HOST")
)

// Index Handles router "/" request
func Index(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	TelegramIndex(w, r, nil)
}

// MessengerIndex Handles router "/messenger" request
func MessengerIndex(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	err := templates.ExecuteTemplate(w, "messenger.html", struct {
		URI    string
		Host   string
		WSHost string
		Count  []string
	}{"messenger", myutil.AppHost(), wsHost, count()})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// TelegramIndex Handles router "/telegram" request
func TelegramIndex(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	err := templates.ExecuteTemplate(w, "telegram.html", struct {
		URI    string
		Host   string
		WSHost string
		Count  []string
	}{"telegram", myutil.AppHost(), wsHost, count()})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func count() (counterStrs []string) {
	count, err := counter.Alert()
	if err != nil {
		return nil
	}
	countStrs := strings.Split((strconv.Itoa(count)), "")
	for index, num := range countStrs {
		counterStrs = append(counterStrs, num)
		if backIndex := len(countStrs) - index; backIndex != 1 && backIndex%3 == 1 {
			counterStrs = append(counterStrs, ",")
		}
	}
	return counterStrs
}

// Top Handles router "/top" request, it shows top rank of keywords, authors, pushsum
func Top(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	count := 100
	keywords := top.ListKeywordWithScore(count)
	authors := top.ListAuthorWithScore(count)
	pushsum := top.ListPushSumWithScore(count)
	data := struct {
		URI      string
		Host     string
		Keywords top.WordOrders
		Authors  top.WordOrders
		PushSum  top.WordOrders
	}{
		"top",
		myutil.AppHost(),
		keywords,
		authors,
		pushsum,
	}
	err := templates.ExecuteTemplate(w, "top.html", data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// Docs shows advanced intructions
func Docs(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	err := templates.ExecuteTemplate(w, "docs.html", struct {
		URI  string
		Host string
	}{
		"docs",
		myutil.AppHost(),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// Redirect redirects short url to original url
func Redirect(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	checksum := params.ByName("checksum")
	url := shorturl.Original(checksum)
	if url != "" {
		http.Redirect(w, r, url, http.StatusMovedPermanently)
	} else {
		t, err := template.ParseFiles("public/404.html")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		t.Execute(w, nil)
	}
}

// WebSocket upgrades http request to websocket
func WebSocket(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	websocket.Handler(counterHandler).ServeHTTP(w, r)
}

func counterHandler(ws *websocket.Conn) {
	conn := connections.Redis()
	defer conn.Close()
	psc := redis.PubSubConn{Conn: conn}
	psc.Subscribe("alert-counter")
	defer psc.Unsubscribe("alert-counter")
	for {
		switch v := psc.Receive().(type) {
		case redis.Message:
			_, err := ws.Write([]byte(v.Data))
			if err != nil {
				ws.Close()
				return
			}
		case redis.Subscription:
			// fmt.Printf("%s: %s %d\n", v.Channel, v.Kind, v.Count)
		case error:
			return
		}
	}
}
