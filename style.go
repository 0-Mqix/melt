package melt

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	sass "github.com/bep/godartsass/v2"
	"github.com/ericchiang/css"
	"golang.org/x/net/html"
)

var (
	globalsRegex       = regexp.MustCompile(`(?m)(?s)%g[^{},]*`)
	styleSelectorRegex = regexp.MustCompile(`(?m)(?s)([^{}]+)\s*({.+?})`)
	styleCommentRegex  = regexp.MustCompile(`(?m)(?s)\/\*.+?\*\/`)
	queryRegex         = regexp.MustCompile(`(?m)(?s)(@[^{}]+){((?:[^{}]+{[^{}]+})*\s*)}`)
)

const (
	MELT_INTERNAL_GLOBAL_PREFIX = "#MELT-INTERNAL-GLOBAL"
	MELT_INTERNAL_SCOPED_PREFIX = "#MELT-INTERNAL-GLOBAL"
)

func styleError(path string, message any) {
	fmt.Printf("[MELT] [SCSS] %v\n", strings.ReplaceAll(
		fmt.Sprint(message), "file: \".\"",
		fmt.Sprintf("file: \"%s\"", path),
	))
}

func (f *Furnace) sortStyles(styles string) string {

	var scopedStyles string
	var globalStyles string
	var queryStyles string

	styles = queryRegex.ReplaceAllStringFunc(styles, func(s string) string {
		queryStyles += s
		return ""
	})

	styles = styleSelectorRegex.ReplaceAllStringFunc(styles, func(s string) string {
		result := styleSelectorRegex.FindStringSubmatch(s)

		selector := strings.TrimLeftFunc(result[1], func(r rune) bool {
			return unicode.IsSpace(r)
		})

		scoped := false
		global := true

		for _, s := range strings.Split(selector, ".") {

			if strings.Index(s, f.stylePrefix+"-scoped") == 0 {
				scoped = true
				break

			} else if strings.Index(s, f.stylePrefix+"-") == 0 {
				global = false
				break
			}
		}

		value := selector + result[2]

		if !scoped && !global {
			return value
		}

		if scoped {
			scopedStyles += value

		} else if global {
			globalStyles += value
		}

		return ""
	})

	return styles + scopedStyles + globalStyles + queryStyles
}

func (f *Furnace) buildStyle(path, component, styles string, document *html.Node) (string, []string, error) {

	defer func() {
		if r := recover(); r != nil {
			styleError(path, r)
		}
	}()

	styles = styleCommentRegex.ReplaceAllString(styles, "")
	scopedSelectors := make([]string, 0)

	//STEP: PREPARE GLOBALS
	styles = prepareGlobals(styles)

	// STEP: TRANSPILE SCSS WITH DART SASS
	transpiler, err := sass.Start(sass.Options{LogEventHandler: func(e sass.LogEvent) {
		styleError(path, e.Message)
	}})

	if err != nil {
		styleError(path, err)
		return "", scopedSelectors, err
	}

	styleResult, err := transpiler.Execute(sass.Args{
		Source:       styles,
		SourceSyntax: sass.SourceSyntaxSCSS,
		OutputStyle:  sass.OutputStyleCompressed,
	})

	if err != nil {
		panic(err)
	}

	transpiler.Close()

	//STEP: LOCALIZE CSS
	styleId := f.stylePrefix + "-" + component
	selectors := make(map[string]string)

	style := queryRegex.ReplaceAllStringFunc(styleResult.CSS, func(s string) string {
		selector := queryRegex.FindStringSubmatch(s)
		querySelectors := make(map[string]string)

		for _, style := range styleSelectorRegex.FindAllStringSubmatch(selector[2], -1) {
			if len(style) != 3 {
				continue
			}

			querySelectors[style[1]] = style[2]
		}

		modified, scoped := f.modifySelectors(component, styleId, document, querySelectors)
		scopedSelectors = append(scopedSelectors, scoped...)

		selectors[selector[1]] = "{" + modified + "}"

		return ""
	})

	for _, style := range styleSelectorRegex.FindAllStringSubmatch(style, -1) {
		if len(style) != 3 {
			continue
		}

		selectors[style[1]] = style[2]
	}

	styles, scoped := f.modifySelectors(component, styleId, document, selectors)
	scopedSelectors = append(scopedSelectors, scoped...)

	return styles, scopedSelectors, nil
}

func prepareGlobals(styles string) string {
	return globalsRegex.ReplaceAllStringFunc(styles, func(s string) string {

		if strings.Index(s, "%g") == 0 {
			return MELT_INTERNAL_GLOBAL_PREFIX + s[2:]

		} else if strings.Index(s, "%s") == 0 {
			return MELT_INTERNAL_SCOPED_PREFIX + s[2:]

		} else {
			return s
		}
	})
}

func (f *Furnace) modifySelectors(
	component, styleId string,
	document *html.Node,
	selectors map[string]string,
) (string, []string) {
	var scopedSelectors []string
	var styles string

	for selector, rules := range selectors {
		for _, name := range strings.Split(selector, ",") {

			name, global := strings.CutPrefix(name, MELT_INTERNAL_GLOBAL_PREFIX)

			if global || strings.Index(name, "@") == 0 {
				styles += name + rules
				continue
			}

			name, scoped := strings.CutPrefix(name, MELT_INTERNAL_SCOPED_PREFIX)
			selector, err := css.Parse(name)

			if scoped {
				styles += name + "." + f.stylePrefix + "-scoped-" + component + rules
				scopedSelectors = append(scopedSelectors, name)
			} else {
				styles += name + "." + styleId + rules
			}

			if err == nil {
				results := selector.Select(document)
				f.addMeltSelectors(results, styleId)
			}
		}
	}

	return styles, scopedSelectors
}

func (f *Furnace) addMeltSelectors(elements []*html.Node, styleId string) {
	for _, n := range elements {
		if strings.Index(n.Data, f.stylePrefix+"-") == 0 {
			continue
		}

		replacement := html.Attribute{
			Key: "class",
			Val: styleId,
		}

		var index int
		var class *html.Attribute

		for i, a := range n.Attr {

			if a.Key != "class" {
				continue
			}

			for _, name := range strings.Split(a.Val, " ") {

				if name != styleId {
					continue
				}

				goto Next
			}

			index = i
			class = &a
			break
		}

		if class != nil {
			replacement.Val = class.Val + " " + styleId
			n.Attr[index] = replacement
		} else {
			n.Attr = append(n.Attr, replacement)
		}

	Next:
		continue
	}
}

func (f *Furnace) addScopedMeltSelectors(path, component string, scoped []string, document *html.Node) error {
	for _, name := range scoped {

		selector, err := css.Parse(name)

		if err != nil {
			styleError(path, err)
			continue
		}

		results := selector.Select(document)
		f.addMeltSelectors(results, f.stylePrefix+"-scoped-"+component)
	}

	return nil
}

func (f *Furnace) transpileStyleFiles() string {

	if f.styleInputFile == "" {
		return ""
	}

	defer func() {
		if r := recover(); r != nil {
			fmt.Println(r)
		}
	}()

	// STEP: TRANSPILE SCSS WITH DART SASS
	transpiler, err := sass.Start(sass.Options{LogEventHandler: func(e sass.LogEvent) {
		fmt.Println(e.Message)
	}})

	if err != nil {
		fmt.Println(err)
	}

	content, err := os.ReadFile(f.styleInputFile)

	if err != nil {
		fmt.Println(err)
	}

	result, err := transpiler.Execute(sass.Args{
		Source:         string(content),
		SourceSyntax:   sass.SourceSyntaxSCSS,
		OutputStyle:    sass.OutputStyleCompressed,
		ImportResolver: resolver(f.styleInputFile),
	})

	if err != nil {
		fmt.Println(err)
	}

	return result.CSS
}

func resolver(inputFile string) *fileResolver {
	working, _ := os.Getwd()
	local := filepath.Dir(inputFile)
	return &fileResolver{start: filepath.Join(working, local)}
}

type fileResolver struct {
	start string
}

func (r fileResolver) CanonicalizeURL(path string) (string, error) {

	if strings.Index(path, "file:///") != 0 {
		path = filepath.Join(r.start, path)
		return "file:///" + path, nil
	}

	return path, nil
}

func (r fileResolver) Load(path string) (sass.Import, error) {

	content, err := os.ReadFile(formatPath(strings.TrimPrefix(path, "file:///")))

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
		return sass.Import{}, err
	}

	return sass.Import{
		Content:      string(content),
		SourceSyntax: sass.SourceSyntaxSCSS,
	}, nil
}
