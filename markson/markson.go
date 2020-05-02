package markson

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gopkg.in/russross/blackfriday.v2"
)

// KeyValuePair represents a KV property pair
type KeyValuePair struct {
	Key   string
	Value string
}

// KeyValuePairs represents a slice of KVPairs
type KeyValuePairs []KeyValuePair

type headerScopedParser interface {
	Pairs() KeyValuePairs
	Walk(node *blackfriday.Node, entering bool) blackfriday.WalkStatus
}

////////////////////////////////////////////////////////////////////////////////
// Parse everything and accumulate it
////////////////////////////////////////////////////////////////////////////////
type userContentParser struct {
	key   string
	value strings.Builder
}

func (ucp *userContentParser) Pairs() KeyValuePairs {
	return KeyValuePairs{KeyValuePair{
		Key:   ucp.key,
		Value: ucp.value.String(),
	}}
}

func (ucp *userContentParser) writeTag(tagName string, entering bool) {
	if entering {
		fmt.Fprintf(&ucp.value, "<%s>", tagName)
	} else {
		fmt.Fprintf(&ucp.value, "</%s>", tagName)
	}
}

func (ucp *userContentParser) Walk(node *blackfriday.Node, entering bool) blackfriday.WalkStatus {
	switch node.Type {
	case blackfriday.List:
		listElement := "ul"
		if node.ListData.ListFlags&blackfriday.ListTypeOrdered != 0 {
			listElement = "ol"
		}
		ucp.writeTag(listElement, entering)
	case blackfriday.Item:
		ucp.writeTag("li", entering)
	case blackfriday.Paragraph:
		fmt.Fprintf(&ucp.value, "\n")
	case blackfriday.Heading:
		tag := fmt.Sprintf("h%d", node.HeadingData.Level)
		ucp.writeTag(tag, entering)
	case blackfriday.HorizontalRule:
		ucp.writeTag("hr", entering)
	case blackfriday.Emph:
		ucp.writeTag("i", entering)
	case blackfriday.Strong:
		ucp.writeTag("b", entering)
	case blackfriday.Del:
		ucp.writeTag("del", entering)
	case blackfriday.Link:
		if entering {
			fmt.Fprintf(&ucp.value, `<a href="%s">`, node.LinkData.Destination)
		} else {
			fmt.Fprintf(&ucp.value, "</a>")
		}
	case blackfriday.Softbreak:
		ucp.writeTag("p", entering)
	case blackfriday.Hardbreak:
		ucp.writeTag("br", entering)
	case blackfriday.Code:
		ucp.writeTag("code", entering)
	case blackfriday.HTMLSpan:
		ucp.writeTag("span", entering)
	case blackfriday.Table:
		ucp.writeTag("table", entering)
	case blackfriday.TableCell:
		ucp.writeTag("td", entering)
	case blackfriday.TableHead:
		ucp.writeTag("th", entering)
	case blackfriday.TableBody:
		ucp.writeTag("tbody", entering)
	case blackfriday.TableRow:
		ucp.writeTag("tr", entering)
	default:
		fmt.Fprintf(&ucp.value, string(node.Literal))
	}
	return blackfriday.GoToNext
}

////////////////////////////////////////////////////////////////////////////////
// Set of properties in a table
////////////////////////////////////////////////////////////////////////////////

type sectionScopedPropertyParser struct {
	pair        KeyValuePair
	pairs       KeyValuePairs
	inTableBody bool
}

func (sspp *sectionScopedPropertyParser) Pairs() KeyValuePairs {
	return sspp.pairs
}
func (sspp *sectionScopedPropertyParser) Walk(node *blackfriday.Node, entering bool) blackfriday.WalkStatus {

	switch node.Type {
	case blackfriday.TableBody:
		{
			sspp.inTableBody = entering
		}
	case blackfriday.TableRow:
		{
			if !entering && sspp.pair.Key != "" {
				if sspp.pairs == nil {
					sspp.pairs = KeyValuePairs{}
				}
				sspp.pairs = append(sspp.pairs, sspp.pair)
				sspp.pair.Key = ""
				sspp.pair.Value = ""
			}
		}
	case blackfriday.Text:
		{
			if sspp.inTableBody {
				value := string(node.Literal)
				if sspp.pair.Key == "" {
					sspp.pair.Key = strings.ToLower(value)
				} else if value != "" {
					sspp.pair.Value += value
				}
			}
		}
	}
	return blackfriday.GoToNext
}

func parseMarkson(propertiesTableHeaderName string,
	properties *[]KeyValuePair) blackfriday.NodeVisitor {

	var curParser headerScopedParser
	headingName := ""
	inHeading := false

	nodeVisitor := func(node *blackfriday.Node, entering bool) blackfriday.WalkStatus {

		// If there's a parser, return that...
		if headingName == "" &&
			node.Type == blackfriday.Heading &&
			node.HeadingData.Level == 1 {
			inHeading = true

			if curParser != nil {
				*properties = append(*properties, curParser.Pairs()...)
			}
			curParser = nil
		} else if inHeading && node.Type == blackfriday.Text {
			headingName = string(node.Literal)
		} else if inHeading &&
			node.Type == blackfriday.Heading {
			canonicalName := strings.TrimSpace(strings.ToLower(headingName))
			switch canonicalName {
			case propertiesTableHeaderName:
				curParser = &sectionScopedPropertyParser{}
			default:
				curParser = &userContentParser{
					key: canonicalName,
				}
			}
			headingName = ""
			inHeading = false
			// Skip the rest...
			return blackfriday.GoToNext
		}
		if curParser != nil {
			nextNode := curParser.Walk(node, entering)
			if node.Next == nil &&
				node.Type == blackfriday.Document &&
				!entering {
				*properties = append(*properties, curParser.Pairs()...)
			}
			return nextNode
		}
		return blackfriday.GoToNext
	}
	return nodeVisitor
}

// UnmarshalMarkson unmarshals the Markdown definition of instance
// based on the contents of input. The propertyTableHeaderName is the reserved
// H1 - level element name whose content will be parsed as a KV table definition
// in a Markdown table. All other H1 level nodes will be treated as KV definitions
// The merged KV multimap will be JSON unmarshalled into instance.
func UnmarshalMarkson(input io.Reader,
	propertyTableHeaderName string,
	instance interface{},
	logger *logrus.Logger) error {
	data, dataErr := ioutil.ReadAll(input)
	if dataErr != nil {
		return dataErr
	}
	mdParser := blackfriday.New(blackfriday.WithExtensions(blackfriday.CommonExtensions))
	tree := mdParser.Parse([]byte(data))
	if tree == nil {
		return errors.Errorf("Failed to parse input")
	}

	properties := make([]KeyValuePair, 0)
	tree.Walk(parseMarkson(propertyTableHeaderName, &properties))

	// So we need to extract the content
	mapProperties := make(map[string]interface{})
	for _, eachPair := range properties {
		canonicalKeyName := strings.ToLower(eachPair.Key)

		// Set it...
		existingVal := mapProperties[canonicalKeyName]
		switch typedVal := existingVal.(type) {
		case string:
			mapProperties[canonicalKeyName] = []string{typedVal, eachPair.Value}
		case []string:
			mapProperties[canonicalKeyName] = append(typedVal, eachPair.Value)
		default:
			mapProperties[canonicalKeyName] = eachPair.Value
		}
	}
	// Great, marshal to json
	jsonBytes, jsonBytesErr := json.MarshalIndent(mapProperties, "", " ")
	if jsonBytesErr != nil {
		return jsonBytesErr
	}
	unmarshallErr := json.Unmarshal(jsonBytes, instance)
	if unmarshallErr != nil {
		return unmarshallErr
	}
	logger.WithFields(logrus.Fields{
		"jsonBytes":             string(jsonBytes),
		"configProps":           mapProperties,
		"unmarshaleledInstance": instance,
		"configType":            fmt.Sprintf("%#v", instance),
	}).Debug("Raw config props")

	return nil
}
