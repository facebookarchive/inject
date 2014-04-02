// Package inject provides a reflect based injector. A large application built
// with dependency injection in mind will typically involve the boring work of
// setting up the object graph. This library attempts to take care of this
// boring work by creating and connecting the various objects. Its use involves
// you seeding the object graph with some (possibly incomplete) objects, where
// the underlying types have been tagged for injection. Given this, the
// library will populate the objects creating new ones as necessary. It uses
// singletons by default, supports optional private instances as well as named
// instances.
//
// It works using Go's reflection package and is inherently limited in what it
// can do as opposed to a code-gen system with respect to private fields.
//
// The usage pattern for the library involves struct tags. It requires the tag
// format used by the various standard libraries, like json, xml etc. It
// involves tags in one of the three forms below:
//
//     `inject:""`
//     `inject:"private"`
//     `inject:"dev logger"`
//
// The first no value syntax is for the common case of a singleton dependency
// of the associated type. The second triggers creation of a private instance
// for the associated type. Finally the last form is asking for a named
// dependency called "dev logger".
package inject

import (
	"bytes"
	"fmt"
	"reflect"

	"github.com/facebookgo/structtag"
)

// Logger allows for simple logging as inject traverses and populates the
// object graph.
type Logger interface {
	Debugf(format string, v ...interface{})
}

// Populate is a short-hand for populating a graph with the given incomplete
// object values.
func Populate(values ...interface{}) error {
	var g Graph
	for _, v := range values {
		if err := g.Provide(&Object{Value: v}); err != nil {
			return err
		}
	}
	return g.Populate()
}

// An Object in the Graph.
type Object struct {
	Value        interface{}
	Name         string // Optional
	Complete     bool   // If true, the Value will be considered complete
	reflectType  reflect.Type
	reflectValue reflect.Value
	private      bool // If true, the Value will not be used and will only be populated
	level        int
	created      bool // If true, the Object was created by us
	embedded     bool // If true, the Object is an embedded struct provided internally
}

// String representation suitable for human consumption.
func (o *Object) String() string {
	var buf bytes.Buffer
	fmt.Fprint(&buf, o.reflectType)
	if o.Name != "" {
		fmt.Fprintf(&buf, " named %s", o.Name)
	}
	return buf.String()
}

// The Graph of Objects.
type Graph struct {
	Logger      Logger // Optional, will trigger debug logging.
	unnamed     []*Object
	unnamedType map[string]bool
	named       map[string]*Object
	maxLevel    int
	levels      [][]*Object
}

// Provide objects to the Graph. The Object documentation describes
// the impact of various fields.
func (g *Graph) Provide(objects ...*Object) error {
	for _, o := range objects {
		o.reflectType = reflect.TypeOf(o.Value)
		o.reflectValue = reflect.ValueOf(o.Value)

		if o.Name == "" {
			if !isStructPtr(o.reflectType) {
				return fmt.Errorf(
					"expected unnamed object value to be a pointer to a struct but got type %s "+
						"with value %v",
					o.reflectType,
					o.Value,
				)
			}

			if !o.private {
				if g.unnamedType == nil {
					g.unnamedType = make(map[string]bool)
				}

				key := fmt.Sprint(o.reflectType)
				if g.unnamedType[key] {
					return fmt.Errorf(
						"provided two unnamed instances of type %s",
						o.reflectType,
					)
				}
				g.unnamedType[key] = true
			}
			g.unnamed = append(g.unnamed, o)
		} else {
			if g.named == nil {
				g.named = make(map[string]*Object)
			}

			if g.named[o.Name] != nil {
				return fmt.Errorf("provided two instances named %s", o.Name)
			}
			g.named[o.Name] = o
		}

		if g.Logger != nil {
			if o.created {
				g.Logger.Debugf("created %s", o)
			} else if o.embedded {
				g.Logger.Debugf("provided embedded %s", o)
			} else {
				g.Logger.Debugf("provided %s", o)
			}
		}
	}
	return nil
}

// Populate the incomplete Objects.
func (g *Graph) Populate() error {
	// We append and modify our slice as we go along, so we don't use a standard
	// range loop, and do a single pass thru each object in our graph.
	i := 0
	for {
		if i == len(g.unnamed) {
			break
		}

		o := g.unnamed[i]
		i++

		if o.Complete {
			continue
		}

		if err := g.populateExplicit(o); err != nil {
			return err
		}
	}

	for _, o := range g.named {
		if o.Complete {
			continue
		}

		if err := g.populateExplicit(o); err != nil {
			return err
		}
	}

	// A Second pass handles injecting Interface values to ensure we have created
	// all concrete types first.
	for _, o := range g.unnamed {
		if o.Complete {
			continue
		}

		if err := g.populateUnnamedInterface(o); err != nil {
			return err
		}
	}

	for _, o := range g.named {
		if o.Complete {
			continue
		}

		if err := g.populateUnnamedInterface(o); err != nil {
			return err
		}
	}

	// Finally we build the levels.
	g.levels = make([][]*Object, g.maxLevel+1)
	for _, o := range g.unnamed {
		g.levels[o.level] = append(g.levels[o.level], o)
	}
	for _, o := range g.named {
		g.levels[o.level] = append(g.levels[o.level], o)
	}

	return nil
}

func (g *Graph) populateExplicit(o *Object) error {
	// Ignore named value types.
	if o.Name != "" && !isStructPtr(o.reflectType) {
		return nil
	}

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

		// Cannot be used with unexported fields.
		if !field.CanSet() {
			return fmt.Errorf(
				"inject requested on unexported field %s in type %s",
				o.reflectType.Elem().Field(i).Name,
				o.reflectType,
			)
		}

		// Don't overwrite existing values.
		if !isNilOrZero(field, fieldType) {
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

			if !existing.reflectType.AssignableTo(fieldType) {
				return fmt.Errorf(
					"object named %s of type %s is not assignable to field %s in type %s",
					tag.Name,
					fieldType,
					o.reflectType.Elem().Field(i).Name,
					o.reflectType,
				)
			}

			field.Set(reflect.ValueOf(existing.Value))
			if g.Logger != nil {
				g.Logger.Debugf(
					"assigned %s to field %s in %s",
					existing,
					o.reflectType.Elem().Field(i).Name,
					o,
				)
			}
			g.updateLevel(o, existing)
			continue StructLoop
		}

		// Inline struct values indicate we want to traverse into it, but not
		// inject itself.
		if fieldType.Kind() == reflect.Struct {
			if tag == injectPrivate {
				return fmt.Errorf(
					"cannot use private inject on inline struct on field %s in type %s",
					o.reflectType.Elem().Field(i).Name,
					o.reflectType,
				)
			}

			newLevel := o.level + 1
			if g.maxLevel < newLevel {
				g.maxLevel = newLevel
			}

			err := g.Provide(&Object{
				Value:    field.Addr().Interface(),
				private:  true,
				level:    newLevel,
				embedded: true,
			})
			if err != nil {
				return err
			}
			continue
		}

		// Interface injection is handled in a second pass.
		if fieldType.Kind() == reflect.Interface {
			continue
		}

		// Maps are created and required to be private.
		if fieldType.Kind() == reflect.Map {
			if tag != injectPrivate {
				return fmt.Errorf(
					"inject on map field %s in type %s must be named or private",
					o.reflectType.Elem().Field(i).Name,
					o.reflectType,
				)
			}

			field.Set(reflect.MakeMap(fieldType))
			if g.Logger != nil {
				g.Logger.Debugf(
					"made map for field %s in %s",
					o.reflectType.Elem().Field(i).Name,
					o,
				)
			}
			continue
		}

		// Can only inject Pointers from here on.
		if !isStructPtr(fieldType) {
			return fmt.Errorf(
				"found inject tag on unsupported field %s in type %s",
				o.reflectType.Elem().Field(i).Name,
				o.reflectType,
			)
		}

		// Unless it's a private inject, we'll look for an existing instance of the
		// same type.
		if tag != injectPrivate {
			for _, existing := range g.unnamed {
				if existing.private {
					continue
				}
				if existing.reflectType.AssignableTo(fieldType) {
					field.Set(reflect.ValueOf(existing.Value))
					if g.Logger != nil {
						g.Logger.Debugf(
							"assigned existing %s to field %s in %s",
							existing,
							o.reflectType.Elem().Field(i).Name,
							o,
						)
					}
					g.updateLevel(o, existing)
					continue StructLoop
				}
			}
		}

		// Did not find an existing Object of the type we want or injectPrivate,
		// we'll create one.
		newValue := reflect.New(fieldType.Elem())
		newLevel := o.level + 1
		if g.maxLevel < newLevel {
			g.maxLevel = newLevel
		}
		newObject := &Object{
			Value:   newValue.Interface(),
			private: tag == injectPrivate,
			level:   newLevel,
			created: true,
		}

		// Add the newly ceated object to the known set of objects.
		err = g.Provide(newObject)
		if err != nil {
			return err
		}

		// Finally assign the newly created object to our field.
		field.Set(newValue)
		if g.Logger != nil {
			g.Logger.Debugf(
				"assigned newly created %s to field %s in %s",
				newObject,
				o.reflectType.Elem().Field(i).Name,
				o,
			)
		}
	}
	return nil
}

func (g *Graph) populateUnnamedInterface(o *Object) error {
	// Ignore named value types.
	if o.Name != "" && !isStructPtr(o.reflectType) {
		return nil
	}

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

		// We only handle interface injection here. Other cases including errors
		// are handled in the first pass when we inject pointers.
		if fieldType.Kind() != reflect.Interface {
			continue
		}

		// Interface injection can't be private because we can't instantiate new
		// instances of an interface.
		if tag == injectPrivate {
			return fmt.Errorf(
				"found private inject tag on interface field %s in type %s",
				o.reflectType.Elem().Field(i).Name,
				o.reflectType,
			)
		}

		// Don't overwrite existing values.
		if !isNilOrZero(field, fieldType) {
			continue
		}

		// Named injects must have already been handled in populateExplicit.
		if tag.Name != "" {
			panic(fmt.Sprintf("unhandled named instance with name %s", tag.Name))
		}

		// Find one, and only one assignable value for the field.
		var found *Object
		for _, existing := range g.unnamed {
			if existing.private {
				continue
			}
			if existing.reflectType.AssignableTo(fieldType) {
				if found != nil {
					return fmt.Errorf(
						"found two assignable values for field %s in type %s. one type "+
							"%s with value %v and another type %s with value %v",
						o.reflectType.Elem().Field(i).Name,
						o.reflectType,
						found.reflectType,
						found.Value,
						existing.reflectType,
						existing.reflectValue,
					)
				}
				found = existing
				field.Set(reflect.ValueOf(existing.Value))
				if g.Logger != nil {
					g.Logger.Debugf(
						"assigned existing %s to interface field %s in %s",
						existing,
						o.reflectType.Elem().Field(i).Name,
						o,
					)
				}
				g.updateLevel(o, existing)
			}
		}

		// If we didn't find an assignable value, we're missing something.
		if found == nil {
			return fmt.Errorf(
				"found no assignable value for field %s in type %s",
				o.reflectType.Elem().Field(i).Name,
				o.reflectType,
			)
		}
	}
	return nil
}

func (g *Graph) updateLevel(use *Object, dep *Object) {
	newLevel := use.level + 1
	if newLevel <= dep.level {
		return
	}
	dep.level = newLevel
	if g.maxLevel < newLevel {
		g.maxLevel = newLevel
	}
}

// Levels returns a slice of levels of objects of the Object Graph.
func (g *Graph) Levels() [][]*Object {
	// TODO: copy?
	return g.levels
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
	found, value, err := structtag.Extract("inject", t)
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

func isStructPtr(t reflect.Type) bool {
	return t.Kind() == reflect.Ptr && t.Elem().Kind() == reflect.Struct
}

func isNilOrZero(v reflect.Value, t reflect.Type) bool {
	switch v.Kind() {
	default:
		return reflect.DeepEqual(v.Interface(), reflect.Zero(t).Interface())
	case reflect.Interface, reflect.Ptr:
		return v.IsNil()
	}
}
