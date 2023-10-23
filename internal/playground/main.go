package main

import (
	"embed"
	"fmt"
	"io"
	"net/http"

	"github.com/0-mqix/melt"
	"github.com/0-mqix/melt/internal/playground/templates"
	"github.com/go-chi/chi/v5"
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
	production := false
	var m *melt.Furnace

	if !production {
		m = melt.New(
			melt.WithWatcher("/reload_event", true, true, []string{".html", ".scss"}, "./templates"),
			melt.WithOutput("./melt.json"),
			melt.WithStyle(true, "melt", "./templates/main.scss", ""),
			melt.WithGeneration("./templates/templates.go"),
		)
	} else {
		build, _ := build.ReadFile("melt.json")
		m = melt.NewProduction(build)
	}

	templates.Load(m, templates.GlobalHandlers{
		Global1: Global1,
		Global2: Global2,
	})

	root := m.MustGetRoot("./templates/root.html")

	r := chi.NewRouter()

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {

		templateData := templates.IndexData{
			Styles: m.Styles,
		}

		arguments := melt.GlobalArguments(map[string]any{
			"name": "mqix",
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
