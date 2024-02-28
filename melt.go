package melt

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"sync"
	"sync/atomic"
	text "text/template"
)

type contextValueKey string

const GLOBALS_CONTEXT_KEY contextValueKey = "globals"

type Furnace struct {
	componentComments bool
	printRenderOutput bool

	autoReloadEvent        bool
	autoReloadEventUrl     string
	autoUpdateImports      bool
	watcherSendReloadEvent bool

	style           bool
	styleOutputFile string
	styleInputFile  string
	stylePrefix     string

	outputFile           string
	generationOutputFile string

	tailwind           bool
	tailwindExecutable string
	tailwindInputFile  string
	tailwindOutputFile string
	tailwindConfigFile string

	Components         map[string]*Component
	ComponentFunctions template.FuncMap

	Roots         map[string]*Root
	RootFunctions text.FuncMap

	Styles         string
	TailwindStyles string

	reloadSubscribers map[string]chan bool
	subscribersMutex  sync.Mutex
	lastArgumentId    atomic.Int64
	dependencyOf      map[string]map[string]bool

	productionMode bool
}

type Build struct {
	Components     []*Component `json:"components"`
	Roots          []*Root      `json:"roots"`
	FileStyles     string       `json:"file_styles"`
	TailwindStyles string       `json:"tailwind_styles"`
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

func NewProduction(input []byte, ComponentFunctions, RootFunctions text.FuncMap) *Furnace {
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
		template := template.New(c.Name).Funcs(componentFunctions)

		if ComponentFunctions != nil {
			template.Funcs(ComponentFunctions)
		}

		c.Template, err = template.Parse(string(c.String))

		if err != nil {
			panic("[MELT] invalid input at component from " + c.Path)
		}

		c.furnace = f
		f.Components[c.Path] = c
		f.Styles += c.Style
	}

	f.Styles = build.FileStyles + f.sortStyles(f.Styles)
	f.TailwindStyles = build.TailwindStyles

	for _, r := range build.Roots {
		template := template.New(r.Path).Funcs(rootFunctions)

		if RootFunctions != nil {
			template.Funcs(RootFunctions)
		}

		r.Template, err = template.Parse(string(r.String))

		if err != nil {
			panic("[MELT] invalid input at root from " + r.Path)
		}

		f.Roots[r.Path] = r
	}

	return f
}

func WithPrintRenderOutput(value bool) meltOption {
	return func(f *Furnace) {
		f.printRenderOutput = value
	}
}

func WithComponentComments(value bool) meltOption {
	return func(f *Furnace) {
		f.componentComments = value
	}
}

func WithWatcher(reloadEventUrl string, autoUpdateImports, watcherSendReloadEvent bool, extentions []string, paths ...string) meltOption {
	return func(f *Furnace) {
		f.autoReloadEvent = true
		f.autoReloadEventUrl = reloadEventUrl
		f.autoUpdateImports = autoUpdateImports
		f.watcherSendReloadEvent = watcherSendReloadEvent

		go f.StartWatcher(extentions, paths...)
	}
}

func WithOutput(outputFile string) meltOption {
	return func(f *Furnace) {
		if outputFile != "" {
			f.outputFile = formatPath(outputFile)
		}
	}
}

func WithStyle(prefix, inputPath, outputPath string) meltOption {
	return func(f *Furnace) {
		f.style = true
		f.stylePrefix = prefix

		if inputPath != "" {
			f.styleInputFile = formatPath(inputPath)
		}

		if outputPath != "" {
			f.styleOutputFile = formatPath(outputPath)
		}

	}
}

func WithTailwind(executable, configPath, inputPath, outputPath string) meltOption {
	return func(f *Furnace) {
		f.tailwind = true
		f.tailwindExecutable = executable
		f.tailwindConfigFile = formatPath(configPath)

		if inputPath != "" {
			f.tailwindInputFile = formatPath(inputPath)
		}

		if outputPath != "" {
			f.tailwindOutputFile = formatPath(outputPath)
		}
	}
}

func WithComponentFuncMap(funcs template.FuncMap) meltOption {
	return func(f *Furnace) {
		f.ComponentFunctions = funcs
	}
}

func WithRootFuncMap(funcs text.FuncMap) meltOption {
	return func(f *Furnace) {
		f.RootFunctions = funcs
	}
}

func WithGeneration(path string) meltOption {
	return func(f *Furnace) {
		f.generationOutputFile = path
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

	if f.tailwind {
		f.runTailwind()
	}

	if f.style {
		fmt.Println("[MELT] updating: styles")
		var styles string

		for _, c := range f.Components {
			styles += c.Style
		}

		fileStyles = f.transpileStyleFiles()
		styles = fileStyles + f.sortStyles(styles)

		if f.styleOutputFile != "" {
			writeOutputFile(f.styleOutputFile, []byte(styles))
		}

		f.Styles = styles
	}

	if f.outputFile != "" {

		if f.productionMode {
			fmt.Println("[MELT] output is currently not suported in production mode")
			return
		}

		var output Build

		output.FileStyles = fileStyles
		output.TailwindStyles = f.TailwindStyles

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

		build, err := json.Marshal(output)
		if err != nil {
			fmt.Println("[MELT] [BUILD]", err)
			return
		}

		writeOutputFile(f.outputFile, build)
	}
}

type Html string

// MarshalJSON encodes the html content using base64 encoding.
func (h Html) MarshalJSON() ([]byte, error) {
	encoded := base64.RawStdEncoding.EncodeToString([]byte(h))
	return json.Marshal(encoded)
}

// UnmarshalJSON decodes the html content from base64 encoding.
func (h *Html) UnmarshalJSON(data []byte) error {
	var encoded string

	if err := json.Unmarshal(data, &encoded); err != nil {
		return err
	}

	decoded, err := base64.RawStdEncoding.DecodeString(encoded)

	if err != nil {
		return err
	}

	*h = Html(decoded)
	return nil
}
