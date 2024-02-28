package main

import (
	"embed"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/0-mqix/melt"
	"github.com/0-mqix/melt/internal/playground/templates"
	"github.com/go-chi/chi/v5"
)

//go:generate go run generate.go

//go:embed "melt.json"
var build embed.FS

func main() {
	production := len(os.Args) == 2 && os.Args[1] == "production"

	var m *melt.Furnace

	if !production {
		m = melt.New(
			melt.WithWatcher("/reload_event", true, true, []string{".html"}, "./templates"),
			melt.WithOutput("./melt.json"),
			melt.WithTailwind("tailwindcss", "./tailwind.config.js", "./styles/tailwind.css", ""),
			melt.WithStyle("melt", "./styles/main.scss", "./static/style.css"),
			melt.WithGeneration("./templates/templates.go"),
		)
	} else {
		build, _ := build.ReadFile("melt.json")
		m = melt.NewProduction(build, nil, nil)
	}

	templates.Load(m, templates.GlobalHandlers{})

	root := m.MustGetRoot("./templates/root.html")

	r := chi.NewRouter()

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("index")

		root.Write(w, nil, func(w io.Writer) {
			templates.Index.Write(w, r, nil)
		})
	})

	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		root.Write(w, nil, func(w io.Writer) {
			templates.Index.WriteTemplate(w, "test", 2)
		})
	})

	r.Get("/style.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-type", "text/css")
		w.Write([]byte(m.Styles))
	})

	r.Get("/reload_event", m.ReloadEventHandler)

	fmt.Println("[HTTP]", http.ListenAndServe(":4000", r))
}
