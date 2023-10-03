package melt

import (
	"bytes"
	"encoding/base32"
	"encoding/hex"
	"fmt"
	"html/template"
	"io"
	"path/filepath"
	"regexp"
	"strings"

	formatter "github.com/yosssi/gohtml"
	"golang.org/x/net/html"
)

var (
	componentRegex        = regexp.MustCompile(`(?m)<(?P<closing>[/-]?)(?P<name>(?:[A-Z]|slot|global)(?:[a-zA-Z0-9-_]?)+)(?P<attributes>(?:[^>"/]+|"[^"]*")*|)(?P<self_closing>/?)>`)
	TemplateFunctionRegex = regexp.MustCompile(`(?m){{\s*([^{}]+?)\s*?}}(\n?)`)
	CommentRegex          = regexp.MustCompile(`(?m)<!--(.*?)-->`)
	ImportRegex           = regexp.MustCompile(`(?m)(?s)(<import>)(.+?) (.+?)(<\/import>)`)
	TypeRegex             = regexp.MustCompile(`(?m)@type\("([^"]+)"(?:,\s?"([^"]+)")?\)`)

	encoder = base32.NewEncoding("abcdefghijklmnopqrstuvwxyz123456").WithPadding(base32.NoPadding)
)

type Pair struct {
	Key   string
	Value string
}

func formatPath(path string) string {
	path = strings.ToLower(path)
	path = filepath.Clean(path)
	path = filepath.ToSlash(path)

	return path
}

func getStyle(n *html.Node) (string, bool) {
	if n.Type == html.ElementNode && n.Data == "style" {
		return n.FirstChild.Data, true
	}

	return "", false
}

func getPair(n *html.Node) (*Pair, bool) {
	if n.FirstChild == nil || n.FirstChild.Type != html.TextNode {
		return nil, false
	}

	line := strings.TrimSpace(n.FirstChild.Data)
	data := strings.Split(line, " ")

	if len(data) < 2 {
		return nil, false
	}

	return &Pair{
		Key:   data[0],
		Value: data[1],
	}, true
}

func extractFromBody(n *html.Node, result *[]*html.Node) {
	if n.Type == html.ElementNode && n.Data == "body" {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			*result = append(*result, c)
		}
		return
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		extractFromBody(c, result)
	}
}

func (f *Furnace) Render(name string, reader io.Reader, path string) (*Component, error) {

	if f.productionMode {
		return nil, fmt.Errorf("[MELT] rendering is currently not suported in production mode")
	}

	raw, err := io.ReadAll(reader)

	if err != nil {
		return nil, err
	}

	global := false

	/// STEP: CHANGE COMPONENT CALLS
	var usedComponents []string

	raw = componentRegex.ReplaceAllFunc(raw, func(b []byte) []byte {
		data := componentRegex.FindStringSubmatch(string(b))

		if data[2] == "global" {
			global = true
			return []byte("")
		}

		if data[2] == "slot" {
			return []byte("[[ .Childeren ]]")
		}

		var childeren string
		var attributes string

		for _, a := range SplitIgnoreString(data[3], ' ') {
			if strings.Index(a, "-") != 0 {
				attributes += a + " "
				continue
			}

			split := SplitIgnoreString(a, '=')

			if len(split) < 2 {
				continue
			}

			name := encoder.EncodeToString([]byte(split[0][1:]))

			tagName := fmt.Sprintf("melt-partial-%s", name)
			startTag := fmt.Sprintf("<%s>", tagName)
			closingTag := fmt.Sprintf("</%s>", tagName)

			raw := strings.TrimFunc(split[1], func(r rune) bool {
				if r == '"' || r == '\'' {
					return true
				}

				return false
			})

			childeren += startTag + raw + closingTag
			usedComponents = append(usedComponents, startTag, closingTag)
		}

		name := encoder.EncodeToString([]byte(data[2]))
		attributes = encoder.EncodeToString([]byte(attributes))

		if data[1] == "-" {
			return []byte(fmt.Sprintf("[[ partial_component .Partials `%s` ]]", data[2]))
		}

		closing := data[1] == "/"
		selfClosing := data[4] == "/"

		var replacement string

		tagName := fmt.Sprintf("melt-%s", name)
		startTag := fmt.Sprintf("<%s melt-attributes=\"%s\">", tagName, attributes)
		closingTag := fmt.Sprintf("</%s>", tagName)

		if !closing && !selfClosing {
			replacement = startTag + childeren
			usedComponents = append(usedComponents, startTag)
		} else if closing {
			replacement = closingTag
			usedComponents = append(usedComponents, closingTag)
		} else if selfClosing {
			replacement = startTag + childeren + closingTag
			usedComponents = append(usedComponents, startTag, closingTag)
		}

		return []byte(replacement)
	})

	fmt.Printf("%s global = %v\n", name, global)

	/// STEP: HIDE TEMPLATING AND PREFIX VARIABLES
	raw = TemplateFunctionRegex.ReplaceAllFunc(raw, func(b []byte) []byte {
		result := TemplateFunctionRegex.FindSubmatch(b)

		prefix := fmt.Sprintf("$%s_", name)
		value := string(result[1])
		value = prefixTemplateVariables(value, "$", prefix)

		replacement := encodeForTemplate(value)
		return append([]byte(replacement), result[2]...)
	})

	// STEP: PARSE
	modified := bytes.NewBuffer(raw)
	document, _ := html.Parse(modified)

	// STEP: GET AND DELETE STYLE & IMPORTS
	var nodes []*html.Node

	style := ""
	imports := make(map[string]*Pair)
	defaults := make(map[string]string)

	extractFromBody(document, &nodes)
	var melted []*html.Node

	directory := filepath.Dir(path)

	for _, n := range nodes {
		if n.Type != html.ElementNode {
			melted = append(melted, n)
			continue
		}

		switch n.Data {
		case "import":
			result, ok := getPair(n)

			if ok {
				path := filepath.Join(directory, result.Value)
				result.Value = filepath.ToSlash(path)
				imports[result.Key] = result
			}

		case "default":
			result, ok := getPair(n)

			if ok {

				if result.Key[0] != '$' {
					fmt.Printf("[MELT] default is only supported for $ variables\n-> %s -> <default>%s %s</default>\n", path, result.Key, result.Value)
					break
				}

				defaults[result.Key] = result.Value
			}

		case "style":
			result, ok := getStyle(n)
			if ok {
				if !f.Style {
					fmt.Println("[MELT] [SCSS] is not enabled\n->", path)
					break
				}
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

	var scoped []string

	if f.Style {
		style, scoped, _ = f.style(name, style, document)
	}

	//STEP: CREATE RESULT
	component := &Component{
		Name:     name,
		Path:     path,
		Style:    style,
		global:   global,
		Template: template.New(name).Funcs(Functions).Funcs(f.ComponentFuncMap),

		defaults: defaults,
	}

	//STEP: USE COMPONENTS
	f.useComponents(document, component, imports)

	if f.Style {
		f.addScopedMeltSelectors(name, scoped, document)
	}

	//STEP: CLEAN HTML
	nodes = nil
	extractFromBody(document, &nodes)

	templateBuffer := bytes.NewBufferString("")

	for _, n := range nodes {
		html.Render(templateBuffer, n)
	}

	templateString := templateBuffer.String()

	for _, component := range usedComponents {
		templateString = strings.ReplaceAll(templateString, component, "")
	}

	//STEP: ADD PARTIALS TEMPLATE TO COMPONENT
	component.partialsTemplate = templateString

	// STEP: RENDER TEMPLATE FUNCTIONS
	templateString = TemplateFunctionRegex.ReplaceAllStringFunc(templateString, func(s string) string {
		result := TemplateFunctionRegex.FindStringSubmatch(s)
		value, _ := hex.DecodeString(result[1])
		return "{{" + string(value) + "}}" + result[2]
	})

	// STEP: COMMENT WORK AROUND
	templateString = CommentRegex.ReplaceAllStringFunc(templateString, func(s string) string {
		content := CommentRegex.FindStringSubmatch(s)
		return fmt.Sprintf("{{comment \"%s\" }}", content[1])
	})

	//STEP: DATA TYPE GENERATION & TEMPLATE MODIFICATION
	if f.GenerationOutputFile != "" {
		component.generationData, templateString = extractGenerationData(templateString)
	}

	// STEP: FORMAT HTML
	formatter.Condense = true
	templateString = formatter.Format(templateString)

	component.String = templateString

	// STEP: PARSE TEMPLATE FINALY
	component.Template.Parse(templateString)

	if f.PrintRenderOutput {
		fmt.Printf("*** template: %s ***\n%s\n*** end template ***\n", name, templateString)
	}

	return component, nil
}
