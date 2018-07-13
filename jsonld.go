package jsonldnode

import (
	"fmt"
	"strconv"

	node "github.com/ipfs/go-ipld-format"
	jsonld "github.com/piprate/json-gold/ld"
)

var processor = jsonld.NewJsonLdProcessor()
var options = jsonld.NewJsonLdOptions("")

// Plugin wow
type Plugin struct{}

// Resolve yourself
func (n *Node) Resolve(path []string) (interface{}, []string, error) {
	// Return the top-level node if path is empty
	if len(path) == 0 {
		return n.obj, path, nil
	}

	// Get initial node, check if there's any path left
	init, path, context, id, err := initial(n.obj, path)
	if len(path) == 0 || err != nil {
		return init, path, err
	}

	// Flatten the document
	flattened, err := processor.Flatten(n.obj, context, options)

	if err != nil {
		return init, path, err
	}

	switch flattened := flattened.(type) {
	case map[interface{}]interface{}:
		switch graph := flattened["@graph"].(type) {
		case []map[interface{}]interface{}:
			value, path, err := resolve(graph, init, path, id)
			switch value := value.(type) {
			case map[string]string:
				_, hasLink := value["/"]
				if len(value) == 1 && hasLink {
					return value, path, err
				}
			}
			if len(path) > 0 {
				return value, path, fmt.Errorf("Could not resolve all the way through")
			}
			return value, path, err
		default:
			return init, path, fmt.Errorf("Invalid JSON-LD Document")
		}
	}

	return nil, nil, nil
}

func initial(doc interface{}, path []string) (interface{}, []string, interface{}, string, error) {
	var id = "@id"
	// Identify the three cases
	switch doc := doc.(type) {
	case map[interface{}]interface{}:
		context, hasContext := doc["@context"]
		_, hasGraph := doc["@graph"]

		// Get the ID key
		if hasContext {
			switch context.(type) {
			case string:
				id = "id"
			}
		}

		if hasContext && hasGraph && len(doc) == 2 {
			// Index by @id
			switch graph := doc["@graph"].(type) {
			case []map[interface{}]interface{}:
				for _, v := range graph {
					if v[id] == path[0] {
						return v, path[1:], context, id, nil
					}
				}
				return doc, path, nil, id, fmt.Errorf("Could not find top-level node with indexed id")
			default:
				return doc, path, nil, id, fmt.Errorf("Invalid JSON-LD Document")
			}
		} else {
			// Index by property
			value, hasValue := doc[path[0]]
			if hasValue {
				return value, path[1:], nil, id, nil
			}
			return doc, path, context, id, fmt.Errorf("Could not find top-level property with index")
		}
	case []interface{}:
		// Index by number
		index, err := strconv.ParseInt(path[0], 0, 64)
		if err != nil {
			return doc, path, nil, id, err
		} else if index < 0 || index >= int64(len(doc)) {
			return doc, path, nil, id, fmt.Errorf("Array index is out of bounds")
		}
		return doc[index], path[1:], nil, id, nil
	default:
		return nil, path, nil, id, fmt.Errorf("Could not parse top level document")
	}
}

func extract(graph []map[interface{}]interface{}, value interface{}, id string) (interface{}, error) {
	for value != nil {
		switch object := value.(type) {
		case map[interface{}]interface{}:
			if len(object) == 1 {
				list, hasList := object["@list"]
				set, hasSet := object["@set"]
				if hasList {
					value = list
					continue
				} else if hasSet {
					value = set
					continue
				}
			}
		}
		break
	}
	switch object := value.(type) {
	case map[interface{}]interface{}:
		v, hasValue := object["@value"]
		if hasValue {
			i, hasIndex := object["@index"]
			if hasIndex {
				value = map[interface{}]interface{}{
					i: v,
				}
			} else {
				value = v
			}
		}

		i, hasID := object[id]
		if hasID && len(object) == 1 {
			set := true
			for _, n := range graph {
				j, hasID := n[id]
				if hasID && i == j {
					value = n
					set = false
				}
			}
			if set {
				return nil, fmt.Errorf("Invliad id")
			}
		}
	}

	return value, nil
}

func resolve(graph []map[interface{}]interface{}, value interface{}, path []string, id string) (interface{}, []string, error) {
	// If nil, return
	if value == nil {
		return nil, path, nil
	}

	// If primitive, return
	switch value := value.(type) {
	case []interface{}:
	case map[interface{}]interface{}:
	default:
		return value, path, nil
	}

	if len(path) > 0 {
		part := path[0]
		switch value := value.(type) {
		case []interface{}:
			// Index by number
			index, err := strconv.ParseInt(part, 0, 64)
			if err != nil {
				return value, path, err
			} else if index < 0 || index >= int64(len(value)) {
				return value, path, fmt.Errorf("Array index is out of bounds")
			}
			result, err := extract(graph, value[index], id)
			if err != nil {
				return value, path, err
			}
			return resolve(graph, result, path[1:], id)
		case map[interface{}]interface{}:
			// Index by property
			thing, hasThing := value[part]
			if hasThing {
				result, err := extract(graph, thing, id)
				if err != nil {
					return value, path, err
				}
				return resolve(graph, result, path[1:], id)
			}
			return value, path, fmt.Errorf("Could not find top-level property with index")
		default:
			return value, path, fmt.Errorf("Invalid JSON-LD Document")
		}
	} else {
		return value, path, nil
	}
}

// ResolveLink to continue
func (n Node) ResolveLink(path []string) (*node.Link, []string, error) {
	obj, rest, err := n.Resolve(path)
	if err != nil {
		return nil, nil, err
	}

	lnk, ok := obj.(*node.Link)
	if ok {
		return lnk, rest, nil
	}

	return nil, rest, fmt.Errorf("found non-link at given path")
}

// MULTICODEC isn't real
const MULTICODEC = 0x77

// RegisterBlockDecoders for fun
func (plugin *Plugin) RegisterBlockDecoders(dec node.BlockDecoder) error {
	dec.Register(MULTICODEC, DecodeBlock)
	return nil
}

var _ node.DecodeBlockFunc = DecodeBlock
