package melt

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"sync"
	"sync/atomic"
	text "text/template"
)

/* \
NOTE:
	- %s does ofcouse not work for global components

TODO:
  - rewrite styling to work better and i think %s not working
  - component without vars not generated
  - type gen is silly
  - templates do feel icky
  - component write with & data
  - create a template project with lit
  - improve error handeling
  - add build in hx boost in root
  - make readme with documentation
*/

type contextValueKey string

const GLOBALS_CONTEXT_KEY contextValueKey = "globals"

type Furnace struct {
	ComponentComments      bool   //adds comments to the html so you can see wat the source of the html is
	AutoReloadEvent        bool   //enables the live reloading features you still have to call f.StartWatcher(paths ...string) or you can just use the option WithAutoReloadEvent
	AutoReloadEventUrl     string //the url that is pointed to f.ReloadEventHandler
	PrintRenderOutput      bool   //prints out the template after a render
	AutoUpdateImports      bool   //update all imports with the renamed path only works with the watcher
	WatcherSendReloadEvent bool   //send a reload event on watcher event
	Style                  bool   //scss in <style> -> dart sass -> localize the styles to the component
	StyleOutputFile        string //if not empty melt will write all the styles to this file
	StyleInputFile         string //if not empty melt will use this file as an main scss file
	StylePrefix            string //the prefix of the css melt adds to the elements for localization
	OutputFile             string //if not empty melt will write a output file that is used to use your components in production
	GenerationOutputFile   string //if not empty melt will generate a golang file with types and functions to easly call a template with the found types

	Components       map[string]*Component
	ComponentFuncMap template.FuncMap

	Roots       map[string]*Root
	RootFuncMap text.FuncMap

	Styles string

	reloadSubscribers map[string]chan bool
	subscribersMutex  sync.Mutex
	lastArgumentId    atomic.Int64
	dependencyOf      map[string]map[string]bool

	productionMode bool
}

type Build struct {
	Components []*Component `json:"components"`
	Roots      []*Root      `json:"roots"`
	FileStyles string       `json:"file_styles"`
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
		c.furnace = f
		f.Components[c.Path] = c
		f.Styles += c.Style
		f.Styles = build.FileStyles + f.Styles
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

func WithWatcher(reloadEventUrl string, autoUpdateImports, watcherSendReloadEvent bool, extentions []string, paths ...string) meltOption {
	return func(f *Furnace) {
		f.AutoReloadEvent = true
		f.AutoReloadEventUrl = reloadEventUrl
		f.AutoUpdateImports = autoUpdateImports
		f.WatcherSendReloadEvent = watcherSendReloadEvent

		go f.StartWatcher(extentions, paths...)
	}
}

func WithOutput(outputFile string) meltOption {
	return func(f *Furnace) {
		if outputFile != "" {
			f.OutputFile = formatPath(outputFile)
		}
	}
}

func WithStyle(value bool, prefix, inputPath, outputPath string) meltOption {
	return func(f *Furnace) {
		f.Style = value
		f.StylePrefix = prefix

		if inputPath != "" {
			f.StyleInputFile = formatPath(inputPath)
		}

		if outputPath != "" {
			f.StyleOutputFile = formatPath(outputPath)
		}

	}
}

func WithComponentFuncMap(funcs template.FuncMap) meltOption {
	return func(f *Furnace) {
		f.ComponentFuncMap = funcs
	}
}

func WithRootFuncMap(funcs text.FuncMap) meltOption {
	return func(f *Furnace) {
		f.RootFuncMap = funcs
	}
}

func WithGeneration(path string) meltOption {
	return func(f *Furnace) {
		f.GenerationOutputFile = path
	}
}

func writeOutputFile(path string, content []byte) {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)

	if err != nil {
		fmt.Println("[MELT] [OUTPUT]", err)
		return
	}

	_, err = file.Write(content)

	if err != nil {
		fmt.Println("[MELT] [OUTPUT]", err)
		return
	}

	file.Close()
}

func (f *Furnace) SetGlobalHandlers(handlers map[string]GlobalHandler) {
	for path, handler := range handlers {
		f.MustGetComponent(path).SetGlobalHandler(handler)
	}
}

func (f *Furnace) Output() {
	var fileStyles string

	if f.Style {
		fmt.Println("[MELT] updating: styles")
		var styles string

		for _, c := range f.Components {
			styles += c.Style
		}

		fileStyles = f.transpileStyleFiles()
		styles = fileStyles + f.sortStyles(styles)

		if f.StyleOutputFile != "" {
			writeOutputFile(f.StyleOutputFile, []byte(styles))
		}

		f.Styles = styles
	}

	if f.OutputFile != "" {

		if f.productionMode {
			fmt.Println("[MELT] output is currently not suported in production mode")
			return
		}

		var output Build

		output.FileStyles = fileStyles

		for _, c := range f.Components {
			output.Components = append(output.Components, c)
		}

		for path := range f.Roots {
			raw, err := os.ReadFile(path)

			if err != nil {
				fmt.Println("[MELT] [BUILD]", err)
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
