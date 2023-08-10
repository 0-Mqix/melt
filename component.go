package melt

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"strings"

	"unicode"
)

func (f *Furnace) AddDependency(path, to string) {
	m, ok := f.dependencyOf[path]

	if !ok {
		f.dependencyOf[path] = map[string]bool{to: true}
		return
	}

	m[to] = true
}

type Component struct {
	Template *template.Template
	Style    string
	Name     string
	Path     string

	partialsTemplate string
}

func (f *Furnace) GetComponent(path string, force bool) (*Component, bool) {
	path = strings.ToLower(path)
	component, ok := f.Components[path]

	if ok && !force {
		return component, true
	}

	raw, err := os.ReadFile(path)

	if err != nil {
		fmt.Println("[MELT] could not open file", path)
		return nil, false
	}

	name, ok := strings.CutSuffix(path, ".html")

	if !ok {
		fmt.Println("[MELT] invalid import path:", path)
		return nil, false
	}

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
	words := strings.FieldsFunc(input, func(r rune) bool {
		return unicode.IsSpace(r) || r == '_' || r == '/'
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
	buffer := bytes.NewBufferString("")
	err := c.Template.Execute(buffer, data)

	if err != nil {
		fmt.Println("[MELT]", err)
	}

	buffer.WriteTo(w)
}
