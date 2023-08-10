package melt

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"golang.org/x/net/html"
)

const RELOAD_EVENT_SCRIPT = "new EventSource(\"%s\").onmessage = function (e) {if (e.data == \"reload\") {location.reload();} };"

type Root struct {
	Raw string
}

func (r *Root) Write(w http.ResponseWriter, component *Component, data any, style string) {
	raw := bytes.NewBufferString("")
	err := component.Template.Execute(raw, data)

	if err != nil {
		fmt.Println("[MELT]", err)
	}

	fmt.Fprintf(w, r.Raw, style, raw)
}

func (f *Furnace) GetRoot(path string, force bool) (*Root, bool) {
	path = strings.ToLower(path)

	root, ok := f.Roots[path]

	if ok && !force {
		return root, true
	}

	raw, err := os.ReadFile(path)

	if err != nil {
		fmt.Println("[MELT]", err)
		return nil, false
	}

	_, ok = strings.CutSuffix(path, ".html")

	if !ok {
		fmt.Println("[MELT] invalid import path:", path)
		return nil, false
	}

	root, err = f.createRoot(bytes.NewBuffer(raw))

	if err != nil {
		fmt.Println("[MELT]", err)
		return nil, false
	}

	if force {
		if _, ok := f.Roots[path]; ok {
			*f.Roots[path] = *root
		} else {
			f.Roots[path] = root
		}
	} else {
		f.AddRoot(path, root)
	}

	return root, true
}

func (f *Furnace) MustGetRoot(path string) *Root {
	root, ok := f.GetRoot(path, true)
	if !ok {
		panic(fmt.Sprintf("[MELT] could not get root at path: %s", path))
	}

	return root
}

func (f *Furnace) AddRoot(path string, root *Root) {
	if _, ok := f.Roots[path]; ok {
		fmt.Printf("[MELT] %s was already defined", path)
	}

	f.Roots[path] = root
}

func appendReloadEventScript(n *html.Node, url string) {
	if n.Type == html.ElementNode && n.Data == "html" {

		script := html.Node{Type: html.ElementNode, Data: "script"}
		content := html.Node{Type: html.TextNode, Data: fmt.Sprintf(RELOAD_EVENT_SCRIPT, url)}

		script.AppendChild(&content)
		n.AppendChild(&script)

		return
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		appendReloadEventScript(c, url)
	}
}

func (f *Furnace) createRoot(reader io.Reader) (*Root, error) {
	document, err := html.Parse(reader)

	if err != nil {
		return nil, err
	}

	buffer := bytes.NewBufferString("")

	if f.AutoReloadEvent {
		appendReloadEventScript(document, f.ReloadEventUrl)
	}

	html.Render(buffer, document)

	return &Root{Raw: buffer.String()}, nil
}
