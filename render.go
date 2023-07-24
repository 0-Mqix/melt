package melt

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"html/template"
	"io"
	"regexp"
	"strings"

	sass "github.com/bep/godartsass/v2"
	"golang.org/x/net/html"
)

var (
	transformedComponentRegex = regexp.MustCompile(`(?m)(?m)melt-component-(.*?)\((.*?)\)`)
	componentRegex            = regexp.MustCompile(`(?m)\<(?P<name>[A-Z][a-zA-Z0-9-_]+)(?P<attributes>\s+[^\>]*|)\/>`)
	styleLastSelectorRegex    = regexp.MustCompile(`(?m)([\w-]+)\s*{`)
	TemplateFunctionRegex     = regexp.MustCompile(`(?m){{\s*([^{}]+?)\s*?}}`)
)

type Import struct {
	Name string
	Path string
}

func getStyle(n *html.Node) (string, bool) {
	if n.Type == html.ElementNode && n.Data == "style" {
		return n.FirstChild.Data, true
	}

	return "", false
}

func applyStyleId(n *html.Node, styleId string, selectors map[string]bool) {
	found := false
	var class *html.Attribute
	var index int

	if n.Type == html.ElementNode && selectors[n.Data] {
		found = true
	}

	for i, a := range n.Attr {
		if !(a.Key == "class" || a.Key == "id") {
			continue
		}

		if a.Key == "class" {
			class = &a
			index = i
		}

		for _, k := range strings.Split(a.Val, " ") {

			if !selectors[k] {
				continue
			}

			found = true
		}
	}

	if found {
		replacement := html.Attribute{
			Key:       "class",
			Val:       styleId,
			Namespace: n.Namespace,
		}

		if class != nil {
			replacement.Val = class.Val + " " + styleId
			n.Attr[index] = replacement
		} else {
			n.Attr = append(n.Attr, replacement)
		}
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		applyStyleId(c, styleId, selectors)
	}
}

func getImports(n *html.Node) ([]*Import, bool) {
	result := make([]*Import, 0)

	if n.Type == html.ElementNode && n.Data == "imports" {

		for _, line := range strings.Split(n.FirstChild.Data, "\n") {
			line = strings.TrimSpace(line)
			data := strings.Split(line, " ")

			if len(data) < 2 {
				continue
			}

			result = append(result, &Import{
				Name: data[0],
				Path: data[1],
			})
		}
	}

	return result, false
}

func extractFromFakeBody(n *html.Node, result *[]*html.Node) {
	if n.Type == html.ElementNode && n.Data == "body" {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			*result = append(*result, c)
		}
		return
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		extractFromFakeBody(c, result)
	}
}

func (f *Furnace) Render(name string, file io.Reader) (*Component, error) {
	raw, err := io.ReadAll(file)

	if err != nil {
		return nil, err
	}

	/// STEP: CHANGE COMPONENT CALLS
	var usedComponents []string
	raw = componentRegex.ReplaceAllFunc(raw, func(b []byte) []byte {
		v := componentRegex.FindStringSubmatch(string(b))

		attributes := hex.EncodeToString([]byte(v[2]))

		id := "melt-component-" + v[1] + "(" + attributes + ")"
		usedComponents = append(usedComponents, id)

		return []byte(id)
	})

	/// STEP: HIDE TEMPLATING
	raw = TemplateFunctionRegex.ReplaceAllFunc(raw, func(b []byte) []byte {
		value := hex.EncodeToString(b[2 : len(b)-2])
		return []byte("{{" + value + "}}")
	})

	// STEP: PARSE
	modified := bytes.NewBuffer(raw)
	document, _ := html.Parse(modified)

	// STEP: GET AND DELETE STYLE & IMPORTS
	var nodes []*html.Node

	style := ""
	imports := make(map[string]*Import)

	extractFromFakeBody(document, &nodes)
	var melted []*html.Node

	for _, n := range nodes {
		if n.Type != html.ElementNode {
			melted = append(melted, n)
			continue
		}

		switch n.Data {
		case "imports":
			result, _ := getImports(n)
			for _, i := range result {
				imports[i.Name] = i
			}

		case "style":
			result, ok := getStyle(n)
			if ok {
				style += result
			}

		default:
			melted = append(melted, n)
		}
	}

	//STEP: RECREATE DOCUMENT WITHOUT IMPORTS & STYLE
	meltedBuffer := bytes.NewBufferString("")
	for _, n := range melted {
		html.Render(meltedBuffer, n)
	}
	document, _ = html.Parse(meltedBuffer)

	// STEP: TRANSPILE SCSS WITH DART SASS
	transpiler, err := sass.Start(sass.Options{LogEventHandler: func(e sass.LogEvent) {
		fmt.Printf("[MELT] [SCSS] %v\n", e)
	}})

	if err != nil {
		fmt.Printf("[MELT] [SCSS] %v\n", err)
		return nil, err
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
	selectors := make(map[string]bool)

	style = styleLastSelectorRegex.ReplaceAllStringFunc(styleResult.CSS, func(s string) string {
		selector := s[:len(s)-1]
		selectors[selector] = true

		return selector + "." + styleId + "{"
	})

	applyStyleId(document, styleId, selectors)

	//STEP: USE COMPONENTS
	styles := make(map[string]string)
	f.useComponents(document, imports, styles)

	//STEP: CLEAN HTML
	nodes = nil
	extractFromFakeBody(document, &nodes)

	templateBuffer := bytes.NewBufferString("")

	for _, n := range nodes {
		html.Render(templateBuffer, n)
	}

	templateString := templateBuffer.String()
	document, _ = html.Parse(templateBuffer)

	// STEP: CREATE RESULT
	for _, s := range styles {
		style += s
	}

	result := &Component{
		Style:    style,
		Template: template.New(name),
	}

	extractFromFakeBody(document, &result.Nodes)

	/// STEP: RENDER TEMPLATE FUNCTIONS
	templateString = TemplateFunctionRegex.ReplaceAllStringFunc(templateString, func(s string) string {
		value, _ := hex.DecodeString(s[2 : len(s)-2])
		return "{{" + string(value) + "}}"
	})

	// STEP: PARSE TEMPLATE FINALY
	result.Template.Parse(templateString)

	return result, nil
}
