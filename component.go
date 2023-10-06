package melt

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

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

type GlobalHandler = func(r *http.Request, arguments map[string]any) any

type GlobalOption any

func GlobalFunction(path string, handler GlobalHandler) GlobalOption {
	return []any{formatPath(path), handler}
}

func GlobalArguments(arguments map[string]any) GlobalOption {
	return arguments
}

type Component struct {
	furnace *Furnace `json:"-"`

	Template *template.Template `json:"-"`
	String   string             `json:"template"`

	Style string `json:"style"`
	Name  string `json:"name"`
	Path  string `json:"path"`

	defaults         map[string]string `json:"-"`
	partialsTemplate string            `json:"-"`

	Global        bool          `json:"global"`
	Globals       []string      `json:"globals"`
	GlobalHandler GlobalHandler `json:"-"`

	*generationData `json:"-"`
}

func (c *Component) SetGlobalHandler(handler GlobalHandler) {
	c.GlobalHandler = handler
}

func (f *Furnace) GetComponent(path string, force bool) (*Component, bool) {

	path = formatPath(path)

	old, ok := f.Components[path]

	if ok && !force {
		return old, true
	}

	if f.productionMode {
		return old, ok
	}

	raw, err := os.ReadFile(path)

	if err != nil {
		fmt.Println("[MELT] could not open file", path)
		return nil, false
	}

	component, err := f.Render(ComponentName(path), bytes.NewBuffer(raw), path)

	if err != nil {
		fmt.Println("[MELT]", err)
		return nil, false
	}

	if force {
		if old != nil {
			old.Template = component.Template
			old.String = component.String
			old.Style = component.Style
			old.defaults = component.defaults
			old.partialsTemplate = component.partialsTemplate
			old.Global = component.Global
			old.Globals = component.Globals
			old.generationData = component.generationData

			component = old
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

func ComponentName(path string) string {
	name, _ := strings.CutSuffix(path, filepath.Ext(path))

	words := strings.FieldsFunc(name, func(r rune) bool {
		return unicode.IsSpace(r) || r == '_' || r == '-' || r == '/'
	})

	transformed := make([]string, len(words))
	for i, word := range words {
		if len(word) > 0 {
			transformed[i] = strings.ToUpper(string(word[0])) + word[1:]
		}
	}

	result := strings.Join(transformed, "")
	result = strings.TrimPrefix(result, "Templates")

	return result
}

func (c *Component) Write(w io.Writer, r *http.Request, data any, globalOptions ...GlobalOption) error {

	if len(c.Globals) > 0 {
		var wg sync.WaitGroup
		var mutex sync.Mutex

		results := make(map[string]string)
		handlers := make(map[string]GlobalHandler)
		arguments := make(map[string]any)

		for _, option := range globalOptions {
			switch data := option.(type) {

			case []any:
				handlers[data[0].(string)] = data[1].(GlobalHandler)

			case map[string]any:
				if data == nil {
					arguments = data
				} else {
					maps.Copy(arguments, data)
				}
			}
		}

		for _, path := range c.Globals {
			wg.Add(1)

			go func(path string) {

				defer wg.Done()

				component, ok := c.furnace.Components[path]

				if !ok {
					fmt.Println("[MELT] [WRITE] component was not found for", path)
					return
				}

				handler := component.GlobalHandler

				if overwrite, ok := handlers[path]; ok {
					handler = overwrite
				}

				if handler == nil {
					fmt.Println("[MELT] [WRITE] no global handler defined for", path)
					return
				}

				buffer := bytes.NewBufferString("")
				component.Write(buffer, r, handler(r, arguments))

				mutex.Lock()
				results[path] = buffer.String()
				mutex.Unlock()
			}(path)
		}

		wg.Wait()

		*r = *r.WithContext(context.WithValue(r.Context(), GLOBALS_CONTEXT_KEY, results))
	}

	err := c.Template.Execute(w, data)

	if err != nil {
		fmt.Println("[MELT]", err)
	}

	return err
}

func (c *Component) WriteTemplate(w io.Writer, name string, data any) error {
	err := c.Template.ExecuteTemplate(w, name, data)

	if err != nil {
		fmt.Println("[MELT]", err)
	}

	return err
}
