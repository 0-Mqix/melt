package melt

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"unicode"
)

func (f *Furnace) AddDependency(path, to string) {

	if f.productionMode {
		fmt.Println("[MELT] dependencies are not supported in production mode")
		return
	}

	m, ok := f.dependencyOf[path]

	if !ok {
		f.dependencyOf[path] = map[string]bool{to: true}
		return
	}

	m[to] = true
}

type Component struct {
	Template *template.Template `json:"-"`
	String   string             `json:"template"`

	Style string `json:"style"`
	Name  string `json:"name"`
	Path  string `json:"path"`

	partialsTemplate string `json:"-"`
}

func (f *Furnace) GetComponent(path string, force bool) (*Component, bool) {
	path = strings.ToLower(path)
	path = filepath.Clean(path)
	path = filepath.ToSlash(path)

	component, ok := f.Components[path]

	if ok && !force {
		return component, true
	}

	if f.productionMode {
		return component, ok
	}

	raw, err := os.ReadFile(path)

	if err != nil {
		fmt.Println("[MELT] could not open file", path)
		return nil, false
	}

	name, _ := strings.CutSuffix(path, filepath.Ext(path))
	component, err = f.Render(ComponentName(name), bytes.NewBuffer(raw), path)

	if err != nil {
		fmt.Println("[MELT]", err)
		return nil, false
	}

	if force {
		if _, ok := f.Components[path]; ok {
			*f.Components[path] = *component
		} else {
			f.Components[path] = component
		}
	} else {
		f.AddComponent(path, component)
	}

	f.Output()

	return component, true
}

func (f *Furnace) MustGetComponent(path string) *Component {
	component, ok := f.GetComponent(path, true)
	if !ok {
		panic(fmt.Sprintf("[MELT] could not get component at path: %s", path))
	}

	return component
}

func (f *Furnace) AddComponent(path string, component *Component) {
	if _, ok := f.Components[path]; ok {
		fmt.Printf("[MELT] %s was already defined", path)
	}

	f.Components[path] = component
}

func ComponentName(input string) string {
	input = filepath.Base(input)

	words := strings.FieldsFunc(input, func(r rune) bool {
		return unicode.IsSpace(r) || r == '_' || r == '-'
	})

	transformed := make([]string, len(words))
	for i, word := range words {
		if len(word) > 0 {
			transformed[i] = strings.ToUpper(string(word[0])) + word[1:]
		}
	}

	return strings.Join(transformed, "")
}

func (c *Component) Write(w http.ResponseWriter, data any) {
	err := c.Template.Execute(w, data)

	if err != nil {
		fmt.Println("[MELT]", err)
	}
}

func (c *Component) WriteTemplate(w http.ResponseWriter, name string, data any) {
	err := c.Template.ExecuteTemplate(w, name, data)

	if err != nil {
		fmt.Println("[MELT]", err)
	}
}
