// Package inject provides a reflect based injector. It works using Go's
// reflection package and is inherently limited in what it can do as opposed to
// a code-gen system.
//
// Struct Tags:
//
//     `inject`
//     `inject:"dev logger"`
//     `inject:"private"`
package inject

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
)

// Short-hand for populating a graph with the given object values.
func Populate(values ...interface{}) error {
	var g Graph
	for _, v := range values {
		if err := g.Provide(Object{Value: v}); err != nil {
			return err
		}
	}
	return g.Populate()
}

// An Object in the Graph.
type Object struct {
	Value         interface{}
	Name          string // Optional
	Complete      bool   // If true, the Value will be considered complete
	reflectType   reflect.Type
	reflectValue  reflect.Value
	assignedCount uint
}

// The Graph of Objects.
type Graph struct {
	unnamed     []*Object
	unnamedType map[string]bool
	named       map[string]*Object
}

// Provide an Object to the Graph. The Object documentation describes
// the impact of various fields.
func (g *Graph) Provide(o Object) error {
	o.reflectType = reflect.TypeOf(o.Value)
	o.reflectValue = reflect.ValueOf(o.Value)

	if !isStructPtr(o.reflectType) {
		return fmt.Errorf(
			"expected object value to be a pointer to a struct but got type %s "+
				"with value %v",
			o.reflectType,
			o.Value,
		)
	}

	if o.Name == "" {
		if g.unnamedType == nil {
			g.unnamedType = make(map[string]bool)
		}

		key := fmt.Sprint(o.reflectType)
		if g.unnamedType[key] {
			return fmt.Errorf("provided two unnamed instances of type %s", o.reflectType)
		}
		g.unnamedType[key] = true
		g.unnamed = append(g.unnamed, &o)
	} else {
		if g.named == nil {
			g.named = make(map[string]*Object)
		}

		if g.named[o.Name] != nil {
			return fmt.Errorf("provided two instances named %s", o.Name)
		}
		g.named[o.Name] = &o
	}
	return nil
}

// Populate the incomplete Objects.
func (g *Graph) Populate() error {
	// we append and modify our slice as we go along, so we don't use a standard
	// range loop, and do a single pass thru each object in our graph.
	i := 0
	for {
		if i == len(g.unnamed) {
			return nil
		}

		o := g.unnamed[i]
		i++

		if o.Complete {
			continue
		}

		if err := g.populate(o); err != nil {
			return err
		}
	}
}

func (g *Graph) populate(o *Object) error {
StructLoop:
	for i := 0; i < o.reflectValue.Elem().NumField(); i++ {
		field := o.reflectValue.Elem().Field(i)
		fieldType := field.Type()
		fieldTag := o.reflectType.Elem().Field(i).Tag
		tag, err := parseTag(string(fieldTag))
		if err != nil {
			return fmt.Errorf(
				"unexpected tag format `%s` for field %s in type %s",
				string(fieldTag),
				o.reflectType.Elem().Field(i).Name,
				o.reflectType,
			)
		}

		// Skip fields without a tag.
		if tag == nil {
			continue
		}

		// Can only inject Pointers.
		if !isStructPtr(fieldType) {
			return fmt.Errorf(
				"found inject tag on non-pointer field %s in type %s",
				o.reflectType.Elem().Field(i).Name,
				o.reflectType,
			)
		}

		// Don't overwrite existing values.
		if !field.IsNil() {
			continue
		}

		// Named injects must have been explicitly provided.
		if tag.Name != "" {
			existing := g.named[tag.Name]
			if existing == nil {
				return fmt.Errorf(
					"did not find object named %s required by field %s in type %s",
					tag.Name,
					o.reflectType.Elem().Field(i).Name,
					o.reflectType,
				)
			}
			existing.assignedCount += 1
			field.Set(reflect.ValueOf(existing.Value))
			continue StructLoop
		}

		// Unless it's a private inject, we'll look for an existing instance.
		if tag != injectPrivate {
			// Check for an existing Object of the same type.
			for _, existing := range g.unnamed {
				if existing.reflectType.AssignableTo(fieldType) {
					existing.assignedCount += 1
					field.Set(reflect.ValueOf(existing.Value))
					continue StructLoop
				}
			}
		}

		// Did not find an existing Object of the type we want or injectPrivate,
		// we'll create one.
		newValue := reflect.New(fieldType.Elem())
		field.Set(newValue)

		// Add the newly ceated object to the known set of objects unless it's
		// private or named.
		if tag == injectOnly {
			if err := g.Provide(Object{Value: newValue.Interface()}); err != nil {
				return err
			}
		}
	}
	return nil
}

var (
	injectOnly    = &tag{}
	injectPrivate = &tag{Private: true}
)

type tag struct {
	Name    string
	Private bool
}

func parseTag(t string) (*tag, error) {
	found, value, err := extractTag(t)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}
	if value == "" {
		return injectOnly, nil
	}
	if value == "private" {
		return injectPrivate, nil
	}
	return &tag{Name: value}, nil
}

var errInvalidTag = errors.New("invalid tag")

func extractTag(tag string) (bool, string, error) {
	for tag != "" {
		// skip leading space
		i := 0
		for i < len(tag) && tag[i] == ' ' {
			i++
		}
		tag = tag[i:]
		if tag == "" {
			break
		}

		// scan to colon.
		// a space or a quote is a syntax error
		i = 0
		for i < len(tag) && tag[i] != ' ' && tag[i] != ':' && tag[i] != '"' {
			i++
		}
		if i+1 >= len(tag) || tag[i] != ':' || tag[i+1] != '"' {
			return false, "", errInvalidTag
		}
		name := string(tag[:i])
		tag = tag[i+1:]

		// scan quoted string to find value
		i = 1
		for i < len(tag) && tag[i] != '"' {
			if tag[i] == '\\' {
				i++
			}
			i++
		}
		if i >= len(tag) {
			return false, "", errInvalidTag
		}
		qvalue := string(tag[:i+1])
		tag = tag[i+1:]

		if "inject" == name {
			value, err := strconv.Unquote(qvalue)
			if err != nil {
				return false, "", err
			}
			return true, value, nil
		}
	}
	return false, "", nil
}

func isStructPtr(t reflect.Type) bool {
	return t.Kind() == reflect.Ptr && t.Elem().Kind() == reflect.Struct
}
