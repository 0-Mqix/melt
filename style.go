package melt

import (
	"fmt"
	"strings"
	"unicode"

	sass "github.com/bep/godartsass/v2"
	"github.com/ericchiang/css"
	"golang.org/x/net/html"
)

// :TODO
// - talk to tailwind cli tool
func tailwind() {

}

func scss(name, style string, document *html.Node) (string, error) {
	//STEP: PREPARE SCSS FOR % GLOBAL
	style = styleSelectorRegex.ReplaceAllStringFunc(style, func(s string) string {
		result := styleSelectorRegex.FindStringSubmatch(s)

		selector := strings.TrimLeftFunc(result[1], func(r rune) bool {
			return unicode.IsSpace(r)
		})

		if selector[0] == '%' {
			selector = MELT_INTERNAL_GLOBAL_PREFIX + selector[1:]
		}

		return selector + result[2]
	})

	// STEP: TRANSPILE SCSS WITH DART SASS
	transpiler, err := sass.Start(sass.Options{LogEventHandler: func(e sass.LogEvent) {
		fmt.Printf("[MELT] [SCSS] %v\n", e)
	}})

	if err != nil {
		fmt.Printf("[MELT] [SCSS] %v\n", err)
		return "", err
	}

	styleResult, err := transpiler.Execute(sass.Args{
		Source:       style,
		SourceSyntax: sass.SourceSyntaxSCSS,
		OutputStyle:  sass.OutputStyleCompressed,
	})

	if err != nil {
		panic(err)
	}

	transpiler.Close()

	//STEP: LOCALIZE CSS
	styleId := "melt-" + name
	selectors := make(map[string]string)

	foundStyles := styleSelectorRegex.FindAllStringSubmatch(styleResult.CSS, -1)

	for _, style := range foundStyles {
		if len(style) != 3 {
			continue
		}

		selectors[style[1]] = style[2]
	}

	style = ""
	for name, rules := range selectors {
		selector, err := css.Parse(name)

		if err != nil {
			fmt.Println("[MELT] [SCSS]", err)
			continue
		}

		results := selector.Select(document)

		name, found := strings.CutPrefix(name, MELT_INTERNAL_GLOBAL_PREFIX)

		if found {
			style += name + rules
			continue
		}

		if len(results) == 0 {
			continue
		}

		style += name + "." + styleId + rules

		for _, n := range results {
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

	return style, nil
}
