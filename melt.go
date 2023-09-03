package melt

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"sync"
	"sync/atomic"
)

/*
TODO:
  - make readme with documentation
*/

type Furnace struct {
	ComponentComments  bool   //adds comments to the html so you can see wat the source of the html is
	AutoReloadEvent    bool   //enables the live reloading features you still have to call f.StartWatcher(paths ...string) or you can just use the option WithAutoReloadEvent
	AutoReloadEventUrl string //the url that is pointed to f.ReloadEventHandler
	PrintRenderOutput  bool   //prints out the template after a render
	AutoUpdateImports  bool   //update all imports with the renamed path only works with the watcher
	Style              bool   //scss in <style> -> dart sass -> localize the styles to the component
	StyleOutputFile    string //if not empty melt will write all the styles to this file
	OutputFile         string //if not empty melt will write a output file that is used to use your components in production
	StylePrefix        string //the prefix of the css melt adds to the elements for localization

	Components map[string]*Component
	Roots      map[string]*Root
	Styles     string

	reloadSubscribers map[string]chan bool
	subscribersMutex  sync.Mutex
	lastArgumentId    atomic.Int64
	dependencyOf      map[string]map[string]bool

	productionMode bool
}

type Build struct {
	Components []*Component `json:"components"`
	Roots      []*Root      `json:"roots"`
}

type meltOption func(*Furnace)

func New(options ...meltOption) *Furnace {
	f := &Furnace{
		Components: make(map[string]*Component),
		Roots:      make(map[string]*Root),

		reloadSubscribers: make(map[string]chan bool),
		dependencyOf:      make(map[string]map[string]bool),
	}

	for _, option := range options {
		option(f)

	}

	return f
}

func NewProduction(input []byte) *Furnace {
	var build Build
	err := json.Unmarshal(input, &build)

	if err != nil {
		panic("[MELT] invalid input")
	}

	f := &Furnace{
		productionMode: true,
		Components:     make(map[string]*Component),
		Roots:          make(map[string]*Root),
	}

	for _, c := range build.Components {
		template := template.New(c.Name).Funcs(Functions)
		c.Template, err = template.Parse(c.String)

		if err != nil {
			panic("[MELT] invalid input at component from " + c.Path)
		}

		f.Components[c.Path] = c
		f.Styles += c.Style
	}

	for _, r := range build.Roots {
		template := template.New(r.Path).Funcs(RootFunctions)
		r.Template, err = template.Parse(r.String)

		if err != nil {
			panic("[MELT] invalid input at root from " + r.Path)
		}

		f.Roots[r.Path] = r
	}

	return f
}

func WithPrintRenderOutput(value bool) meltOption {
	return func(f *Furnace) {
		f.PrintRenderOutput = value
	}
}

func WithComponentComments(value bool) meltOption {
	return func(f *Furnace) {
		f.ComponentComments = value
	}
}

func WithAutoReloadEvent(reloadEventUrl string, autoUpdateImports bool, extentions []string, paths ...string) meltOption {
	return func(f *Furnace) {
		f.AutoReloadEvent = true
		f.AutoReloadEventUrl = reloadEventUrl
		f.AutoUpdateImports = autoUpdateImports

		go f.StartWatcher(extentions, paths...)
	}
}

func WithOutput(outputFile, styleOutputFile string) meltOption {
	return func(f *Furnace) {
		f.StyleOutputFile = formatPath(styleOutputFile)
		f.OutputFile = formatPath(outputFile)
	}
}

func WithStyle(value bool, prefix string) meltOption {
	return func(f *Furnace) {
		f.Style = value
		f.StylePrefix = prefix
	}
}

func writeOutputFile(path string, content []byte) {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)

	if err != nil {
		fmt.Println("[MELT] [BUILD]", err)
		return
	}

	_, err = file.Write(content)

	if err != nil {
		fmt.Println("[MELT] [BUILD]", err)
		return
	}

	file.Close()

}

func (f *Furnace) Output() {
	f.Styles = ""

	for _, c := range f.Components {
		f.Styles += c.Style
	}

	f.Styles = f.sortStyles(f.Styles)

	if f.StyleOutputFile != "" {
		writeOutputFile(f.StyleOutputFile, []byte(f.Styles))
	}

	if f.OutputFile != "" {
		var output Build

		for _, c := range f.Components {
			output.Components = append(output.Components, c)
		}

		for path := range f.Roots {
			raw, err := os.ReadFile(path)

			if err != nil {
				fmt.Println("[MELT]", err)
				continue
			}

			root, _ := f.createRoot(path, bytes.NewBuffer(raw), false)
			output.Roots = append(output.Roots, root)
		}

		raw, err := json.Marshal(output)
		if err != nil {
			fmt.Println("[MELT] [BUILD]", err)
			return
		}
		writeOutputFile(f.OutputFile, raw)
	}
}
