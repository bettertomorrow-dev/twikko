package schema

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"
)

type View struct {
	Mode       string `yaml:"mode"`
	ShowLabels *bool  `yaml:"show_labels"`
	Table      Table  `yaml:"table"`
	Cards      Cards  `yaml:"cards"`
}
type Table struct {
	Columns []Widget `yaml:"columns"`
}
type Cards struct {
	Header *Header `yaml:"header"`
	Body   *Body   `yaml:"body"`
}

func LoadView(viewPath, dataPath string) (*Document, error) {
	return LoadViewWithMode(viewPath, dataPath, "")
}

// LoadViewWithMode applies the launch override before checking editable values.
func LoadViewWithMode(viewPath, dataPath, mode string) (*Document, error) {
	b, err := os.ReadFile(viewPath)
	if err != nil {
		return nil, err
	}
	var root yaml.Node
	if err = decodeYAML(b, &root); err != nil {
		return nil, fmt.Errorf("%s: %w", viewPath, err)
	}
	if len(root.Content) != 1 {
		return nil, fmt.Errorf("%s: empty view", viewPath)
	}
	n := root.Content[0]
	if err = checkKeys(n, "view config", "version", "title", "description", "schema", "import", "view"); err != nil {
		return nil, err
	}
	if err = uniqueKeys(n); err != nil {
		return nil, err
	}
	version := MapValue(n, "version")
	if version == nil || version.Tag != "!!int" || version.Value != "1" {
		return nil, fmt.Errorf("version must be 1")
	}
	sn := MapValue(n, "schema")
	if sn == nil {
		return nil, fmt.Errorf("schema is required")
	}
	if err = checkKeys(sn, "schema", "id_field", "title_field", "fields"); err != nil {
		return nil, err
	}
	fields := MapValue(sn, "fields")
	if fields == nil || fields.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("schema.fields must be a list")
	}
	for _, f := range fields.Content {
		if err = checkKeys(f, "schema.fields", "name", "label", "type", "required", "readonly", "options"); err != nil {
			return nil, err
		}
	}
	var s Schema
	if err = sn.Decode(&s); err != nil {
		return nil, err
	}
	for i := range s.Fields {
		if s.Fields[i].Label == "" {
			s.Fields[i].Label = s.Fields[i].Name
		}
		if s.Fields[i].Name == s.IDField {
			if s.Fields[i].Type != Text && s.Fields[i].Type != Number {
				return nil, fmt.Errorf("id field must be text or number")
			}
			s.Fields[i].Readonly = true
		}
	}
	v := View{Mode: "cards"}
	vn := MapValue(n, "view")
	if vn != nil {
		if err = checkKeys(vn, "view", "mode", "show_labels", "table", "cards"); err != nil {
			return nil, err
		}
		if x := MapValue(vn, "mode"); x != nil {
			if err = x.Decode(&v.Mode); err != nil {
				return nil, err
			}
		}
		if x := MapValue(vn, "show_labels"); x != nil {
			if err = x.Decode(&v.ShowLabels); err != nil {
				return nil, err
			}
		}
		if t := MapValue(vn, "table"); t != nil {
			if err = checkKeys(t, "view.table", "columns"); err != nil {
				return nil, err
			}
			if c := MapValue(t, "columns"); c != nil {
				if c.Kind != yaml.SequenceNode {
					return nil, fmt.Errorf("view.table.columns must be a list")
				}
				for i, x := range c.Content {
					w, e := decodeWidget(x, fmt.Sprintf("view.table.columns[%d]", i))
					if e != nil {
						return nil, e
					}
					v.Table.Columns = append(v.Table.Columns, w)
				}
			}
		}
		if c := MapValue(vn, "cards"); c != nil {
			if err = checkKeys(c, "view.cards", "header", "body"); err != nil {
				return nil, err
			}
			if h := MapValue(c, "header"); h != nil {
				if h.Kind != yaml.SequenceNode {
					return nil, fmt.Errorf("view.cards.header must be a list")
				}
				v.Cards.Header = &Header{}
				if err = h.Decode(v.Cards.Header); err != nil {
					return nil, err
				}
			}
			if body := MapValue(c, "body"); body != nil {
				if body.Kind != yaml.SequenceNode {
					return nil, fmt.Errorf("view.cards.body must be a list")
				}
				v.Cards.Body = &Body{}
				if err = body.Decode(v.Cards.Body); err != nil {
					return nil, err
				}
			}
		}
	}
	if v.Mode != "cards" && v.Mode != "table" {
		return nil, fmt.Errorf("view.mode must be cards or table")
	}
	if mode != "" {
		if mode != "cards" && mode != "table" {
			return nil, fmt.Errorf("mode override must be cards or table")
		}
		v.Mode = mode
	}
	s.Header = v.Cards.Header
	s.Body = v.Cards.Body
	// Validate the layout without the legacy requirement that all required fields be in cards.
	temp := s
	temp.Fields = append([]Field(nil), s.Fields...)
	if temp.IDField == "" && len(temp.Fields) > 0 {
		temp.IDField = "__twikko_validation_id"
		for _, f := range temp.Fields {
			for temp.IDField == f.Name {
				temp.IDField += "_"
			}
		}
		temp.Fields = append(temp.Fields, Field{Name: temp.IDField, Type: Text, Readonly: true})
	}
	for i := range temp.Fields {
		temp.Fields[i].Required = false
	}
	if err = ValidateSchema(temp); err != nil {
		return nil, err
	}
	for i := range s.Fields {
		s.Fields[i].Choices = temp.Fields[i].Choices
	}
	tableSeen := map[string]bool{}
	declared := fieldMap(s.Fields)
	for i := range v.Table.Columns {
		w := &v.Table.Columns[i]
		if err = validateWidget(fmt.Sprintf("view.table.columns[%d]", i), w, declared); err != nil {
			return nil, err
		}
		if w.IsLine() {
			return nil, fmt.Errorf("table columns cannot contain lines")
		}
		if w.IsControl() {
			if tableSeen[w.Source] {
				return nil, fmt.Errorf("duplicate table control %q", w.Source)
			}
			tableSeen[w.Source] = true
			if w.Type == WidgetSelect && len(w.Options) > 0 {
				for j := range s.Fields {
					if s.Fields[j].Name == w.Source {
						if len(s.Fields[j].Choices) > 0 && strings.Join(s.Fields[j].Choices, "\x00") != strings.Join(w.Options, "\x00") {
							return nil, fmt.Errorf("conflicting options for %q", w.Source)
						}
						s.Fields[j].Choices = w.Options
					}
				}
			}
		}
	}
	delimiter := ','
	if imp := MapValue(n, "import"); imp != nil {
		if err = checkKeys(imp, "import", "csv"); err != nil {
			return nil, err
		}
		if c := MapValue(imp, "csv"); c != nil {
			if err = checkKeys(c, "import.csv", "delimiter"); err != nil {
				return nil, err
			}
			if d := MapValue(c, "delimiter"); d != nil {
				if d.Tag != "!!str" || utf8.RuneCountInString(d.Value) != 1 {
					return nil, fmt.Errorf("invalid CSV delimiter")
				}
				delimiter, _ = utf8.DecodeRuneInString(d.Value)
				if delimiter == '\r' || delimiter == '\n' || delimiter == '"' || delimiter == utf8.RuneError || delimiter == 0 {
					return nil, fmt.Errorf("invalid CSV delimiter")
				}
			}
		}
	}
	data, err := os.ReadFile(dataPath)
	if err != nil {
		return nil, err
	}
	var entities []*yaml.Node
	switch strings.ToLower(filepath.Ext(dataPath)) {
	case ".yaml", ".yml":
		var d yaml.Node
		if err = decodeYAML(data, &d); err == nil {
			if len(d.Content) != 1 || d.Content[0].Kind != yaml.SequenceNode {
				err = fmt.Errorf("data must be a sequence")
			} else {
				entities = d.Content[0].Content
			}
		}
	case ".json":
		dec := json.NewDecoder(bytes.NewReader(data))
		dec.UseNumber()
		var node *yaml.Node
		node, err = jsonNode(dec)
		if err == nil {
			if node.Kind != yaml.SequenceNode {
				err = fmt.Errorf("data must be an array")
			} else {
				entities = node.Content
			}
			if err == nil {
				if _, e := dec.Token(); e != io.EOF {
					err = fmt.Errorf("trailing JSON data")
				}
			}
		}
	case ".csv":
		entities, err = readCSV(data, delimiter, s.Fields)
	default:
		return nil, fmt.Errorf("unsupported data format %q", dataPath)
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", dataPath, err)
	}
	editable := map[string]bool{}
	if v.Mode == "cards" {
		if s.Body == nil {
			for _, f := range s.Fields {
				editable[f.Name] = true
			}
		}
		for _, w := range s.widgets() {
			if w.widget.IsControl() {
				editable[w.widget.Source] = true
			}
		}
	} else {
		if len(v.Table.Columns) == 0 {
			for _, f := range s.Fields {
				editable[f.Name] = true
			}
		}
		for _, w := range v.Table.Columns {
			if w.IsControl() {
				editable[w.Source] = true
			}
		}
	}
	ids := []Decimal{}
	textIDs := map[string]bool{}
	for i, e := range entities {
		if e.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("record %d must be a mapping", i)
		}
		if err = uniqueKeys(e); err != nil {
			return nil, fmt.Errorf("record %d: %w", i, err)
		}
		for _, f := range s.Fields {
			x := MapValue(e, f.Name)
			for x != nil && x.Kind == yaml.AliasNode {
				x = x.Alias
			}
			var problem error
			if x == nil || x.Tag == "!!null" || ((f.Type == Text || f.Type == LongText) && x.Kind == yaml.ScalarNode && x.Tag == "!!str" && x.Value == "") || (f.Type == MultiEnum && x.Kind == yaml.SequenceNode && len(x.Content) == 0) {
				if f.Required || f.Name == s.IDField {
					problem = fmt.Errorf("is required")
				}
			} else if f.Type == MultiEnum && x.Kind == yaml.SequenceNode {
				for _, item := range x.Content {
					for item.Kind == yaml.AliasNode {
						item = item.Alias
					}
					if item.Kind != yaml.ScalarNode || item.Tag != "!!str" {
						problem = fmt.Errorf("must contain only strings")
						break
					}
				}
				if problem == nil {
					_, problem = ParseNode(f, x)
				}
			} else if f.Type == Number {
				if x.Kind != yaml.ScalarNode || (x.Tag != "!!int" && x.Tag != "!!float") {
					problem = fmt.Errorf("must be a number")
				} else {
					_, problem = NodeDecimal(x)
				}
			} else if (f.Type == Text || f.Type == LongText || f.Type == Enum || f.Type == Date || f.Type == Time || f.Type == DateTime) && (x.Kind != yaml.ScalarNode || (x.Tag != "!!str" && x.Tag != "!!timestamp")) {
				problem = fmt.Errorf("must be text")
			} else {
				_, problem = ParseNode(f, x)
			}
			if problem != nil && (f.Readonly || !editable[f.Name]) {
				return nil, fmt.Errorf("record %d field %q: %w", i, f.Name, problem)
			}
			if f.Name == s.IDField {
				if problem != nil {
					return nil, fmt.Errorf("record %d ID: %w", i, problem)
				}
				if f.Type == Text {
					if x.Tag != "!!str" || textIDs[x.Value] {
						return nil, fmt.Errorf("record %d invalid or duplicate ID", i)
					}
					textIDs[x.Value] = true
				} else {
					d, _ := NodeDecimal(x)
					for _, old := range ids {
						if d.Equal(old) {
							return nil, fmt.Errorf("record %d duplicate ID", i)
						}
					}
					ids = append(ids, d)
				}
			}
		}
	}
	title, err := optionalText(n, "title")
	if err != nil {
		return nil, err
	}
	desc, err := optionalText(n, "description")
	if err != nil {
		return nil, err
	}
	return &Document{Title: title, Description: desc, Schema: s, View: &v, Entities: entities}, nil
}

func decodeYAML(data []byte, node *yaml.Node) error {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(node); err != nil {
		return err
	}
	var extra yaml.Node
	if err := dec.Decode(&extra); err != io.EOF {
		if err != nil {
			return err
		}
		return fmt.Errorf("multiple YAML documents")
	}
	return nil
}

func uniqueKeys(n *yaml.Node) error {
	if n.Kind == yaml.MappingNode {
		seen := map[string]bool{}
		for i := 0; i < len(n.Content); i += 2 {
			k := n.Content[i]
			if k.Kind != yaml.ScalarNode || k.Tag != "!!str" {
				return fmt.Errorf("mapping key must be text")
			}
			if seen[k.Value] {
				return fmt.Errorf("duplicate key %q", k.Value)
			}
			seen[k.Value] = true
		}
	}
	for _, c := range n.Content {
		if err := uniqueKeys(c); err != nil {
			return err
		}
	}
	return nil
}
func jsonNode(d *json.Decoder) (*yaml.Node, error) {
	t, err := d.Token()
	if err != nil {
		return nil, err
	}
	n := &yaml.Node{}
	switch v := t.(type) {
	case json.Delim:
		switch v {
		case '{':
			n.Kind = yaml.MappingNode
			n.Tag = "!!map"
			seen := map[string]bool{}
			for d.More() {
				k, e := d.Token()
				if e != nil {
					return nil, e
				}
				key, ok := k.(string)
				if !ok || seen[key] {
					return nil, fmt.Errorf("duplicate or invalid JSON key %v", k)
				}
				seen[key] = true
				child, e := jsonNode(d)
				if e != nil {
					return nil, e
				}
				n.Content = append(n.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, child)
			}
			_, err = d.Token()
		case '[':
			n.Kind = yaml.SequenceNode
			n.Tag = "!!seq"
			for d.More() {
				child, e := jsonNode(d)
				if e != nil {
					return nil, e
				}
				n.Content = append(n.Content, child)
			}
			_, err = d.Token()
		default:
			return nil, fmt.Errorf("unexpected JSON delimiter")
		}
	case string:
		n.Kind = yaml.ScalarNode
		n.Tag = "!!str"
		n.Value = v
	case json.Number:
		n.Kind = yaml.ScalarNode
		n.Tag = "!!float"
		n.Value = string(v)
		if _, err = ParseDecimal(n.Value); err != nil {
			return nil, err
		}
		if !strings.ContainsAny(n.Value, ".eE") {
			n.Tag = "!!int"
		}
	case bool:
		n.Kind = yaml.ScalarNode
		n.Tag = "!!bool"
		n.Value = fmt.Sprint(v)
	case nil:
		n.Kind = yaml.ScalarNode
		n.Tag = "!!null"
		n.Value = "null"
	default:
		return nil, fmt.Errorf("invalid JSON value")
	}
	return n, err
}
func readCSV(data []byte, delimiter rune, fields []Field) ([]*yaml.Node, error) {
	r := csv.NewReader(bytes.NewReader(data))
	r.Comma = delimiter
	header, err := r.Read()
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, h := range header {
		if h == "" || seen[h] {
			return nil, fmt.Errorf("empty or duplicate CSV header %q", h)
		}
		seen[h] = true
	}
	declared := fieldMap(fields)
	out := []*yaml.Node{}
	for row := 2; ; row++ {
		values, e := r.Read()
		if e == io.EOF {
			break
		}
		if e != nil {
			return nil, fmt.Errorf("CSV row %d: %w", row, e)
		}
		obj := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		for i, value := range values {
			name := header[i]
			f, known := declared[name]
			x := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
			if known {
				if value == "" {
					switch f.Type {
					case Text, LongText:
					case MultiEnum:
						x = &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
					default:
						x.Tag = "!!null"
						x.Value = "null"
					}
				} else {
					switch f.Type {
					case Number:
						if _, e := ParseDecimal(value); e != nil {
							break
						}
						x.Tag = "!!float"
						if !strings.ContainsAny(value, ".eE") {
							x.Tag = "!!int"
							sign := ""
							digits := value
							if strings.HasPrefix(digits, "-") || strings.HasPrefix(digits, "+") {
								sign, digits = digits[:1], digits[1:]
							}
							if _, e := ParseDecimal(value); e == nil {
								digits = strings.TrimLeft(digits, "0")
								if digits == "" {
									digits = "0"
								}
								x.Value = sign + digits
							}
						}
					case Bool:
						if value == "true" || value == "false" {
							x.Tag = "!!bool"
						}
					case MultiEnum:
						var a []json.RawMessage
						if err = json.Unmarshal([]byte(value), &a); err != nil || a == nil {
							return nil, fmt.Errorf("CSV row %d field %q: invalid array", row, name)
						}
						x = &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
						for _, raw := range a {
							var s string
							if len(raw) == 0 || raw[0] != '"' || json.Unmarshal(raw, &s) != nil {
								return nil, fmt.Errorf("CSV row %d field %q: invalid array", row, name)
							}
							x.Content = append(x.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: s})
						}
					}
				}
			}
			SetMapValue(obj, name, x)
		}
		out = append(out, obj)
	}
	return out, nil
}
