package main

import (
	"embed"
	"fmt"
	"io"
	"net/http"

	"github.com/0-mqix/melt"
	"github.com/0-mqix/melt/playground/data"
	"github.com/0-mqix/melt/playground/templates"
	"github.com/go-chi/chi/v5"
)

//go:generate go run generate.go

//go:embed "melt.json"
var build embed.FS

func main() {

	m := melt.New(
		melt.WithWatcher("/reload_event", true, true, []string{".html"}, "./templates"),
		melt.WithOutput("./melt.json", "./melt.css"),
		melt.WithComponentComments(true),
		melt.WithStyle(true, "melt"),
		melt.WithGeneration("./templates/templates.go"),
		melt.WithPrintRenderOutput(true),
	)

	// build, _ := build.ReadFile("melt.json")
	// m := melt.NewProduction(build)

	templates.Load(m)

	root := m.MustGetRoot("./templates/root.html")

	r := chi.NewRouter()

	r.Use(func(h http.Handler) http.Handler {

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		})
	})

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {

		templateData := templates.TemplatesIndexData{
			Name:   data.Data[int]{Data: 1},
			Number: 13,
		}

		root.Write(w, nil, func(w io.Writer) {
			templates.WriteTemplatesIndex(w, templateData)
		})
	})

	r.Get("/style.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-type", "text/css")
		w.Write([]byte(m.Styles))
	})

	r.Get("/reload_event", m.ReloadEventHandler)

	fmt.Println("[HTTP]", http.ListenAndServe(":3000", r))
}
