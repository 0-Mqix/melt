package melt

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"strings"
	"sync/atomic"
	"unicode"

	"golang.org/x/net/html"
)

/*
 TODO:

  - change the way of custom component parsing
	<Bruh .Reasons="?" /> should become <melt-bruh .Reason="?"></melt-bruh> and then iterate over the tree to change should also work neetly with <slot/>
  -  <slot />
  - make readme with documentation
*/

type Component struct {
	Template *template.Template
	Style    string
	Nodes    []*html.Node
	Name     string
}

type Furnace struct {
	Components       map[string]*Component
	ComponentComment bool
	lastArgumentId   atomic.Int64
}

func New() *Furnace {
	f := &Furnace{
		Components:       make(map[string]*Component),
		ComponentComment: true,
	}

	return f
}

func (f *Furnace) GetComponent(path string) (*Component, bool) {
	component, ok := f.Components[path]

	if ok {
		return component, true
	}

	raw, err := os.ReadFile(path)

	if err != nil {
		fmt.Println("[MELT]", err)
		return nil, false
	}

	split := strings.Split(path, ".")

	if len(split) < 2 {
		fmt.Println("[MELT] invalid import path:", path)
		return nil, false
	}

	component, err = f.Render(ComponentName(split[0]), bytes.NewBuffer(raw))

	if err != nil {
		fmt.Println("[MELT]", err)
		return nil, false
	}

	f.AddComponent(path, component)

	return component, true
}

func (f *Furnace) ComponentExists(path string) bool {
	_, ok := f.Components[path]
	return ok
}

func (f *Furnace) AddComponent(path string, component *Component) {
	if f.ComponentExists(path) {
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
