package main

import (
	"embed"
	"fmt"
	"io"
	"net/http"

	"github.com/0-mqix/melt"
	"github.com/0-mqix/melt/playground/templates"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

//go:generate go run generate.go

//go:embed "melt.json"
var build embed.FS

func Global1(r *http.Request, _ map[string]any) *templates.Global1Data {
	return &templates.Global1Data{Name: "1", Message: "hi"}
}
func Global2(r *http.Request, arguments map[string]any) *templates.Global2Data {
	return &templates.Global2Data{Data: arguments["name"]}
}

func main() {
	m := melt.New(
		melt.WithWatcher("/reload_event", true, true, []string{".html"}, "./templates"),
		melt.WithOutput("./melt.json", "./melt.css"),
		melt.WithStyle(true, "melt"),
		melt.WithGeneration("./templates/templates.go"),
	)

	// build, _ := build.ReadFile("melt.json")
	// m := melt.NewProduction(build)

	templates.Load(m, templates.GlobalHandlers{
		Global1: Global1,
		Global2: Global2,
	})

	root := m.MustGetRoot("./templates/root.html")

	r := chi.NewRouter()

	r.Use(middleware.Logger)

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {

		templateData := templates.IndexData{
			Request: r,
		}

		arguments := melt.GlobalArguments(map[string]any{
			"name": "stan",
		})

		root.Write(w, nil, func(w io.Writer) {
			templates.WriteIndex(w, r, templateData, arguments)
		})
	})

	r.Get("/style.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-type", "text/css")
		w.Write([]byte(m.Styles))
	})

	r.Get("/reload_event", m.ReloadEventHandler)

	fmt.Println("[HTTP]", http.ListenAndServe(":3000", r))
}
