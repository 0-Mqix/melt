package melt

import (
	"bytes"
	"encoding/base32"
	"encoding/hex"
	"fmt"
	"html/template"
	"io"
	"regexp"
	"strings"

	sass "github.com/bep/godartsass/v2"
	formatter "github.com/yosssi/gohtml"
	"golang.org/x/net/html"
)

var (
	componentRegex         = regexp.MustCompile(`(?m)<(?P<name>[A-Z][a-zA-Z0-9-_]+)(?P<attributes>(?:[^>"]+|"[^"]*")*|)\/>`)
	styleLastSelectorRegex = regexp.MustCompile(`(?m)([^{}]+)\s*(?:{)`)
	TemplateFunctionRegex  = regexp.MustCompile(`(?m){{\s*([^{}]+?)\s*?}}`)

	encoder = base32.NewEncoding("abcdefghijklmnopqrstuvwxyz123456").WithPadding(base32.NoPadding)
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
		data := componentRegex.FindStringSubmatch(string(b))

		name := encoder.EncodeToString([]byte(data[1]))
		attributes := encoder.EncodeToString([]byte(data[2]))

		replacement := fmt.Sprint("<melt-" + name + "-" + attributes + "></melt-" + name + "-" + attributes + ">")
		usedComponents = append(usedComponents, "<melt-"+name+"-"+attributes+">", "</melt-"+name+"-"+attributes+">")

		return []byte(replacement)
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

	fmt.Println(selectors)

	// applyStyleId(document, styleId, selectors)

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

	for _, component := range usedComponents {
		templateString = strings.ReplaceAll(templateString, component, "")
	}

	// STEP: CREATE RESULT
	for _, s := range styles {
		style += s
	}

	result := &Component{
		Style:    style,
		Template: template.New(name),
	}

	// STEP: EXTRACT NODES
	templateBuffer = bytes.NewBufferString(templateString)
	document, _ = html.Parse(templateBuffer)
	extractFromFakeBody(document, &result.Nodes)

	/// STEP: RENDER TEMPLATE FUNCTIONS
	templateString = TemplateFunctionRegex.ReplaceAllStringFunc(templateString, func(s string) string {
		value, _ := hex.DecodeString(s[2 : len(s)-2])
		return "{{" + string(value) + "}}"
	})

	// STEP: FORMAT HTML
	formatter.Condense = true
	templateString = formatter.Format(templateString)

	// STEP: PARSE TEMPLATE FINALY
	result.Template.Parse(templateString)

	fmt.Println(templateString)

	return result, nil
}
