package main

import (
	"embed"
	"net/http"
	"strconv"

	"github.com/0-mqix/melt"
	"github.com/go-chi/chi/v5"
)

//go:embed "melt.json"
var build embed.FS

func main() {

	m := melt.New(
		melt.WithAutoReloadEvent("/reload_event", true, "./templates"),
		melt.WithOutput("./melt.json", "./melt.css"),
		// melt.WithComponentComments(true),
		melt.WithStyle(true, "x"),
		// melt.WithPrintRenderOutput(true),
	)

	// build, _ := build.ReadFile("melt.json")
	// m := melt.NewProduction(build)

	index := m.MustGetComponent("templates/index.html")
	root := m.MustGetRoot("templates/root.html")

	// m.Output()

	r := chi.NewRouter()

	// r.Use(middleware.Logger)

	items := make(map[int]string)
	id := 0

	r.Post("/add", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()

		items[id] = r.FormValue("item")
		id++

		index.WriteTemplate(w, "items", items)
	})

	r.Delete("/delete", func(w http.ResponseWriter, r *http.Request) {
		raw := r.URL.Query().Get("item")
		id, _ := strconv.ParseInt(raw, 10, 0)

		delete(items, int(id))

		index.WriteTemplate(w, "items", items)
	})

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {

		var data struct {
			Items map[int]string
		}

		data.Items = items

		root.Write(w, index, data, "/.css")
	})

	r.Get("/style.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-type", "text/css")
		w.Write([]byte(m.Styles))
	})

	r.Get("/reload_event", m.ReloadEventHandler)

	http.ListenAndServe(":3000", r)
}
