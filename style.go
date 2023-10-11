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
	styleSelectorRegex = regexp.MustCompile(`(?m)(?s)([^{}]+)\s*({.+?})`)
	styleCommentRegex  = regexp.MustCompile(`(?m)(?s)\/\*.+?\*\/`)
)

const (
	MELT_INTERNAL_GLOBAL_PREFIX = "#MELT-INTERNAL-GLOBAL"
	MELT_INTERNAL_SCOPED_PREFIX = "#MELT-INTERNAL-SCOPED"
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

	styles = styleSelectorRegex.ReplaceAllStringFunc(styles, func(s string) string {
		result := styleSelectorRegex.FindStringSubmatch(s)

		selector := strings.TrimLeftFunc(result[1], func(r rune) bool {
			return unicode.IsSpace(r)
		})

		scoped := false
		global := true

		for _, s := range strings.Split(selector, ".") {

			if strings.Index(s, f.StylePrefix+"-scoped") == 0 {
				scoped = true
				break

			} else if strings.Index(s, f.StylePrefix+"-") == 0 {
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

	return styles + scopedStyles + globalStyles
}

func (f *Furnace) style(path, component, styles string, document *html.Node) (string, []string, error) {

	defer func() {
		if r := recover(); r != nil {
			styleError(path, r)
		}
	}()

	styles = styleCommentRegex.ReplaceAllString(styles, "")
	scopedSelectors := make([]string, 0)

	//STEP: PREPARE SCSS FOR % GLOBAL
	styles = styleSelectorRegex.ReplaceAllStringFunc(styles, func(s string) string {
		result := styleSelectorRegex.FindStringSubmatch(s)
		selectors := make([]string, 0)

		for _, selector := range strings.Split(result[1], ",") {

			selector = strings.TrimLeftFunc(selector, func(r rune) bool {
				return unicode.IsSpace(r)
			})

			if strings.Index(selector, "%g") == 0 {
				selectors = append(selectors, MELT_INTERNAL_GLOBAL_PREFIX+selector[2:])

			} else if strings.Index(selector, "%s") == 0 {
				selectors = append(selectors, MELT_INTERNAL_SCOPED_PREFIX+selector[2:])

			} else {
				selectors = append(selectors, selector)
			}
		}

		return strings.Join(selectors, ",") + result[2]
	})

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
	styleId := f.StylePrefix + "-" + component
	selectors := make(map[string]string)

	foundStyles := styleSelectorRegex.FindAllStringSubmatch(styleResult.CSS, -1)

	for _, style := range foundStyles {
		if len(style) != 3 {
			continue
		}

		selectors[style[1]] = style[2]
	}

	styles = ""
	for selector, rules := range selectors {
		for _, name := range strings.Split(selector, ",") {

			name, global := strings.CutPrefix(name, MELT_INTERNAL_GLOBAL_PREFIX)

			if global {
				styles += name + rules
				continue
			}

			name, scoped := strings.CutPrefix(name, MELT_INTERNAL_SCOPED_PREFIX)

			selector, err := css.Parse(name)

			if err != nil {
				styleError(path, err)
				continue
			}

			results := selector.Select(document)

			if len(results) == 0 && !scoped {
				continue
			}

			if scoped {
				styles += name + "." + f.StylePrefix + "-scoped-" + component + rules
				scopedSelectors = append(scopedSelectors, name)
			} else {
				styles += name + "." + styleId + rules
			}

			f.addMeltSelectors(results, styleId)
		}

	}

	return styles, scopedSelectors, nil
}

func (f *Furnace) addMeltSelectors(elements []*html.Node, styleId string) {

	for _, n := range elements {
		if strings.Index(n.Data, "melt-") == 0 {
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
		f.addMeltSelectors(results, f.StylePrefix+"-scoped-"+component)
	}

	return nil
}

func (f *Furnace) transpileStyleFiles() string {

	if f.StyleInputFile == "" {
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

	content, err := os.ReadFile(f.StyleInputFile)

	if err != nil {
		fmt.Println(err)
	}

	result, err := transpiler.Execute(sass.Args{
		Source:         string(content),
		SourceSyntax:   sass.SourceSyntaxSCSS,
		OutputStyle:    sass.OutputStyleCompressed,
		ImportResolver: resolver(f.StyleInputFile),
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
