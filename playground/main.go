package main

import (
	"bytes"
	"net/http"

	"github.com/0-mqix/melt"
	"github.com/go-chi/chi"
)

func main() {

	m := melt.New(
		melt.WithAutoReloadEvent("/reload_event", "./"),
	)

	root := m.MustGetRoot("root.html")
	index := m.MustGetComponent("index.html")

	var data struct {
		Name  string
		Yeet  int
		Array []int
		Count int
	}

	data.Name = "Max"
	data.Yeet = 1
	data.Array = []int{1, 2, 3, 4}
	data.Count = 3

	htmlBuffer := bytes.NewBufferString("")

	err := index.Template.Execute(htmlBuffer, data)

	if err != nil {
		panic(err)
	}

	// root, err := os.ReadFile("root.html")

	if err != nil {
		panic(err)
	}

	// value := fmt.Sprintf(string(root), "/.css", htmlBuffer.String())

	r := chi.NewRouter()

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		root.Write(w, index, data, "/.css")
	})

	r.Get("/.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-type", "text/css")
		w.Write([]byte(index.Style))
	})

	r.Get("/reload_event", m.ReloadPageServerEvent)

	http.ListenAndServe(":3000", r)
}
