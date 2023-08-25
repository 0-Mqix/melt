package melt

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strings"
	text "text/template"
	"unicode"

	"golang.org/x/net/html"
)

func SplitIgnoreString(s string, sep rune) []string {
	var result []string
	var part string
	var in bool

	for _, c := range s {

		if c == '"' {
			in = !in
			part += string(c)
			continue
		}

		if !in && sep == ' ' && unicode.IsSpace(c) {
			result = append(result, part)
			part = ""
			continue
		}

		if !in && c == sep {
			result = append(result, part)
			part = ""
			continue
		}

		part += string(c)
	}

	result = append(result, part)

	return result
}

type Argument struct {
	Value string
	Type  int
}

const (
	ArgumentTypeVariable = iota
	ArgumentTypeConstant
)

func (f *Furnace) pasteComponent(
	n *html.Node,
	component *Component,
	attributes []string,
	partials map[string]string,
) {
	buffer := bytes.NewBufferString("")
	declarations := ""

	arguments := make(map[string]Argument)

	for _, attribute := range attributes {
		attribute = strings.TrimSpace(attribute)

		if len(attribute) == 0 {
			continue
		}

		pair := SplitIgnoreString(attribute, '=')
		if len(pair) != 2 {
			continue
		}

		name := pair[0]
		value := pair[1]

		switch name[0] {

		case '.', '$':

			//prefix so variable localization works
			prefix := fmt.Sprintf("$%s_", component.Name)
			name = prefixTemplateVariables(string(name), "$", prefix)

			if !(value[0] == '.' || value[0] == '$') {
				value = strings.Trim(value, "\"")
				value = TemplateFunctionRegex.ReplaceAllStringFunc(value, func(s string) string {
					return "{{" + hex.EncodeToString([]byte(s[2:len(s)-2])) + "}}"
				})

				arguments[name] = Argument{Value: value, Type: ArgumentTypeConstant}
				continue
			}

			id := f.lastArgumentId.Load()
			argument := fmt.Sprintf("$arg%d", id)
			arguments[name] = Argument{Value: argument, Type: ArgumentTypeVariable}

			declaration := fmt.Sprintf("- %s := %s", argument, value)
			encoded := hex.EncodeToString([]byte(declaration))
			declarations += fmt.Sprintf("{{%s}}", encoded)

			f.lastArgumentId.Add(1)

		}

	}

	argumented := TemplateFunctionRegex.ReplaceAllStringFunc(component.partialsTemplate, func(s string) string {
		result := TemplateFunctionRegex.FindStringSubmatch(s)

		b, _ := hex.DecodeString(result[1])

		content := strings.TrimSpace(string(b))

		argument, ok := arguments[content]

		if ok && argument.Type == ArgumentTypeConstant {
			return argument.Value
		}

		content = replaceTemplateVariables(content, arguments)
		content = hex.EncodeToString([]byte(content))

		return "{{" + content + "}}"
	})

	partialized := f.pastePartials(component.Name, argumented, partials)

	//prepend the declarations to the arugmented and the pasted partialized component html
	buffer = bytes.NewBufferString(declarations + partialized)

	nodes := make([]*html.Node, 0)
	element, _ := html.Parse(buffer)
	extractFromBody(element, &nodes)

	for _, part := range nodes {
		part.Parent.RemoveChild(part)
		n.AppendChild(part)
	}
}

func (f *Furnace) useComponents(n *html.Node, self *Component, imports map[string]*componentImport) {
	if n.Type == html.ElementNode && strings.Index(n.Data, "melt-") == 0 {

		data := strings.Split(n.Data, "-")

		if data[1] == "slot" || data[1] == "partial" {
			goto Next
		}

		result, _ := encoder.DecodeString(data[1])
		name := string(result)

		result, _ = encoder.DecodeString(n.Attr[0].Val)
		attributes := SplitIgnoreString(string(result), ' ')

		childeren := bytes.NewBufferString("")
		partials := make(map[string]string)

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode && strings.Index(c.Data, "melt-") == 0 {

				data := strings.Split(c.Data, "-")

				if data[1] != "partial" {
					html.Render(childeren, c)
					defer n.RemoveChild(c)
					continue
				}

				result, _ := encoder.DecodeString(data[2])
				name := string(result)

				partial := bytes.NewBufferString("")

				for pc := c.FirstChild; pc != nil; pc = pc.NextSibling {
					html.Render(partial, pc)
					defer c.RemoveChild(pc)
				}

				partials[name] = partial.String()

			} else {
				html.Render(childeren, c)
				defer n.RemoveChild(c)
			}
		}

		partials["Slot"] = childeren.String()

		source, ok := imports[name]

		if !ok {
			goto Next
		}

		component, ok := f.GetComponent(source.Path, false)
		f.AddDependency(source.Path, self.Path)

		if !ok {
			goto Next
		}

		if f.ComponentComments {
			n.AppendChild(&html.Node{
				Type:      html.CommentNode,
				Data:      fmt.Sprintf(" + %s: %s ", name, source.Path),
				Namespace: n.Namespace,
			})
		}

		f.pasteComponent(n, component, attributes, partials)

		if f.ComponentComments {
			n.AppendChild(&html.Node{
				Type:      html.CommentNode,
				Data:      fmt.Sprintf(" - %s ", name),
				Namespace: n.Namespace,
			})
		}
	}
Next:
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		f.useComponents(c, self, imports)
	}
}

func (f *Furnace) pastePartials(name, raw string, partials map[string]string) string {
	template := text.New(name).
		Delims("[[", "]]").
		Funcs(text.FuncMap{
			"partial_component": func(partials map[string]string, name string) string {
				return partials[name]
			},
		})

	// STEP: MAKE COMPONENT PARTIALS TEMPLATE
	template.Parse(raw)

	var data struct {
		Childeren string
		Partials  map[string]string
	}

	for n, s := range partials {
		s = TemplateFunctionRegex.ReplaceAllStringFunc(s, func(s string) string {

			result := TemplateFunctionRegex.FindStringSubmatch(s)
			raw, _ := hex.DecodeString(result[1])

			value := string(raw)
			prefix := fmt.Sprintf("$%s_", name)
			value = prefixTemplateVariables(value, "%", prefix)

			return "{{" + hex.EncodeToString([]byte(value)) + "}}" + result[2]
		})

		if f.ComponentComments {
			s = fmt.Sprintf("<!-- + %s -->\n%s<!-- - %s -->\n", n, s, n)
		}

		partials[n] = s
	}

	data.Childeren = partials["Slot"]
	delete(partials, "Slot")
	data.Partials = partials

	result := bytes.NewBufferString("")
	err := template.Execute(result, data)

	if err != nil {
		panic(err)
	}

	return result.String()
}
