package melt

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"os"

	"golang.org/x/net/html"
)

const RELOAD_EVENT_SCRIPT = "new EventSource(\"%s\").onmessage = function (e) {if (e.data == \"reload\") {location.reload();} };"

type Root struct {
	Template *template.Template `json:"-"`
	String   Html               `json:"template"`
	Path     string             `json:"path"`
}

func (r *Root) Write(w io.Writer, data any, write func(w io.Writer)) error {
	var templateData struct {
		Body template.HTML
		Data any
	}

	templateData.Data = data

	buffer := bytes.NewBufferString("")
	write(buffer)
	templateData.Body = template.HTML(buffer.String())

	err := r.Template.Execute(w, templateData)

	if err != nil {
		fmt.Println("[MELT]", err)
	}

	return err
}

func (f *Furnace) GetRoot(path string, force bool) (*Root, bool) {
	path = formatPath(path)

	root, ok := f.Roots[path]

	if ok && !force {
		return root, true
	}

	if f.productionMode {
		return root, ok
	}

	raw, err := os.ReadFile(path)

	if err != nil {
		fmt.Println("[MELT]", err)
		return nil, false
	}

	root, err = f.createRoot(path, bytes.NewBuffer(raw), f.AutoReloadEvent)

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

	f.Output()

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

func (f *Furnace) createRoot(path string, reader io.Reader, withReloadEvents bool) (*Root, error) {
	document, err := html.Parse(reader)

	if err != nil {
		return nil, err
	}

	buffer := bytes.NewBufferString("")

	if withReloadEvents {
		appendReloadEventScript(document, f.AutoReloadEventUrl)
	}

	html.Render(buffer, document)
	raw := buffer.String()

	template := template.New(path).Funcs(rootFunctions).Funcs(f.RootFunctions)
	template, err = template.Parse(raw)

	if err != nil {
		fmt.Printf("[MELT] error with creating root at path %s: %s\n", path, err)
	}

	return &Root{Template: template, String: Html(raw), Path: path}, nil
}
