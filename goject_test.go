package goject_test

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/facebookgo/ensure"
	"github.com/imaramos/goject"

	injecttesta "github.com/imaramos/goject/injecttesta"
	injecttestb "github.com/imaramos/goject/injecttestb"
)

func init() {
	// we rely on math.Rand in Container.Objects() and this gives it some randomness.
	rand.Seed(time.Now().UnixNano())
}

type Answerable interface {
	Answer() int
}

type TypeAnswerStruct struct {
	answer  int
	private int
}

func (t *TypeAnswerStruct) Answer() int {
	return t.answer
}

type TypeNestedStruct struct {
	A *TypeAnswerStruct `inject:""`
}

func (t *TypeNestedStruct) Answer() int {
	return t.A.Answer()
}

func TestRequireTag(t *testing.T) {
	var v struct {
		A *TypeAnswerStruct
		B *TypeNestedStruct `inject:""`
	}

	if err := goject.Populate(&v); err != nil {
		t.Fatal(err)
	}
	if v.A != nil {
		t.Fatal("v.A is not nil")
	}
	if v.B == nil {
		t.Fatal("v.B is nil")
	}
}

type TypeWithNonPointerInject struct {
	A int `inject:""`
}

func TestErrorOnNonPointerInject(t *testing.T) {
	var a TypeWithNonPointerInject
	err := goject.Populate(&a)
	if err == nil {
		t.Fatalf("expected error for %+v", a)
	}

	const msg = "found inject tag on unsupported field A in type *goject_test.TypeWithNonPointerInject"
	if err.Error() != msg {
		t.Fatalf("expected:\n%s\nactual:\n%s", msg, err.Error())
	}
}

type TypeWithNonPointerStructInject struct {
	A *int `inject:""`
}

func TestErrorOnNonPointerStructInject(t *testing.T) {
	var a TypeWithNonPointerStructInject
	err := goject.Populate(&a)
	if err == nil {
		t.Fatalf("expected error for %+v", a)
	}

	const msg = "found inject tag on unsupported field A in type *goject_test.TypeWithNonPointerStructInject"
	if err.Error() != msg {
		t.Fatalf("expected:\n%s\nactual:\n%s", msg, err.Error())
	}
}

func TestInjectSimple(t *testing.T) {
	var v struct {
		A *TypeAnswerStruct `inject:""`
		B *TypeNestedStruct `inject:""`
	}

	if err := goject.Populate(&v); err != nil {
		t.Fatal(err)
	}
	if v.A == nil {
		t.Fatal("v.A is nil")
	}
	if v.B == nil {
		t.Fatal("v.B is nil")
	}
	if v.B.A == nil {
		t.Fatal("v.B.A is nil")
	}
	if v.A != v.B.A {
		t.Fatal("got different instances of A")
	}
}

func TestDoesNotOverwrite(t *testing.T) {
	a := &TypeAnswerStruct{}
	var v struct {
		A *TypeAnswerStruct `inject:""`
		B *TypeNestedStruct `inject:""`
	}
	v.A = a
	if err := goject.Populate(&v); err != nil {
		t.Fatal(err)
	}
	if v.A != a {
		t.Fatal("original A was lost")
	}
	if v.B == nil {
		t.Fatal("v.B is nil")
	}
}

func TestPrivate(t *testing.T) {
	var v struct {
		A *TypeAnswerStruct `inject:"private"`
		B *TypeNestedStruct `inject:""`
	}

	if err := goject.Populate(&v); err != nil {
		t.Fatal(err)
	}
	if v.A == nil {
		t.Fatal("v.A is nil")
	}
	if v.B == nil {
		t.Fatal("v.B is nil")
	}
	if v.B.A == nil {
		t.Fatal("v.B.A is nil")
	}
	if v.A == v.B.A {
		t.Fatal("got the same A")
	}
}

type TypeWithJustColon struct {
	A *TypeAnswerStruct `inject:`
}

func TestTagWithJustColon(t *testing.T) {
	var a TypeWithJustColon
	err := goject.Populate(&a)
	if err == nil {
		t.Fatalf("expected error for %+v", a)
	}

	const msg = "unexpected tag format `inject:` for field A in type *goject_test.TypeWithJustColon"
	if err.Error() != msg {
		t.Fatalf("expected:\n%s\nactual:\n%s", msg, err.Error())
	}
}

type TypeWithOpenQuote struct {
	A *TypeAnswerStruct `inject:"`
}

func TestTagWithOpenQuote(t *testing.T) {
	var a TypeWithOpenQuote
	err := goject.Populate(&a)
	if err == nil {
		t.Fatalf("expected error for %+v", a)
	}

	const msg = "unexpected tag format `inject:\"` for field A in type *goject_test.TypeWithOpenQuote"
	if err.Error() != msg {
		t.Fatalf("expected:\n%s\nactual:\n%s", msg, err.Error())
	}
}

func TestProvideWithFields(t *testing.T) {
	var g goject.Container
	a := &TypeAnswerStruct{}
	err := g.Provide(&goject.Object{Value: a, Fields: map[string]*goject.Object{}})
	ensure.NotNil(t, err)
	ensure.DeepEqual(t, err.Error(), "fields were specified on object *goject_test.TypeAnswerStruct when it was provided")
}

func TestProvideNonPointer(t *testing.T) {
	var g goject.Container
	var i int
	err := g.Provide(&goject.Object{Value: i})
	if err == nil {
		t.Fatal("expected error")
	}

	const msg = "expected unnamed object value to be a pointer to a struct but got type int with value 0"
	if err.Error() != msg {
		t.Fatalf("expected:\n%s\nactual:\n%s", msg, err.Error())
	}
}

func TestProvideNonPointerStruct(t *testing.T) {
	var g goject.Container
	var i *int
	err := g.Provide(&goject.Object{Value: i})
	if err == nil {
		t.Fatal("expected error")
	}

	const msg = "expected unnamed object value to be a pointer to a struct but got type *int with value <nil>"
	if err.Error() != msg {
		t.Fatalf("expected:\n%s\nactual:\n%s", msg, err.Error())
	}
}

func TestProvideTwoOfTheSame(t *testing.T) {
	var g goject.Container
	a := TypeAnswerStruct{}
	err := g.Provide(&goject.Object{Value: &a})
	if err != nil {
		t.Fatal(err)
	}

	err = g.Provide(&goject.Object{Value: &a})
	if err == nil {
		t.Fatal("expected error")
	}

	const msg = "provided two unnamed instances of type *github.com/imaramos/goject_test.TypeAnswerStruct"
	if err.Error() != msg {
		t.Fatalf("expected:\n%s\nactual:\n%s", msg, err.Error())
	}
}

func TestProvideTwoOfTheSameWithPopulate(t *testing.T) {
	a := TypeAnswerStruct{}
	err := goject.Populate(&a, &a)
	if err == nil {
		t.Fatal("expected error")
	}

	const msg = "provided two unnamed instances of type *github.com/imaramos/goject_test.TypeAnswerStruct"
	if err.Error() != msg {
		t.Fatalf("expected:\n%s\nactual:\n%s", msg, err.Error())
	}
}

func TestProvideTwoWithTheSameName(t *testing.T) {
	var g goject.Container
	const name = "foo"
	a := TypeAnswerStruct{}
	err := g.Provide(&goject.Object{Value: &a, Name: name})
	if err != nil {
		t.Fatal(err)
	}

	err = g.Provide(&goject.Object{Value: &a, Name: name})
	if err == nil {
		t.Fatal("expected error")
	}

	const msg = "provided two instances named foo"
	if err.Error() != msg {
		t.Fatalf("expected:\n%s\nactual:\n%s", msg, err.Error())
	}
}

func TestNamedInstanceWithDependencies(t *testing.T) {
	var g goject.Container
	a := &TypeNestedStruct{}
	if err := g.Provide(&goject.Object{Value: a, Name: "foo"}); err != nil {
		t.Fatal(err)
	}

	var c struct {
		A *TypeNestedStruct `inject:"foo"`
	}
	if err := g.Provide(&goject.Object{Value: &c}); err != nil {
		t.Fatal(err)
	}

	if err := g.Populate(); err != nil {
		t.Fatal(err)
	}

	if c.A.A == nil {
		t.Fatal("c.A.A was not injected")
	}
}

func TestTwoNamedInstances(t *testing.T) {
	var g goject.Container
	a := &TypeAnswerStruct{}
	b := &TypeAnswerStruct{}
	if err := g.Provide(&goject.Object{Value: a, Name: "foo"}); err != nil {
		t.Fatal(err)
	}

	if err := g.Provide(&goject.Object{Value: b, Name: "bar"}); err != nil {
		t.Fatal(err)
	}

	var c struct {
		A *TypeAnswerStruct `inject:"foo"`
		B *TypeAnswerStruct `inject:"bar"`
	}
	if err := g.Provide(&goject.Object{Value: &c}); err != nil {
		t.Fatal(err)
	}

	if err := g.Populate(); err != nil {
		t.Fatal(err)
	}

	if c.A != a {
		t.Fatal("did not find expected c.A")
	}
	if c.B != b {
		t.Fatal("did not find expected c.B")
	}
}

type TypeWithMissingNamed struct {
	A *TypeAnswerStruct `inject:"foo"`
}

func TestTagWithMissingNamed(t *testing.T) {
	var a TypeWithMissingNamed
	err := goject.Populate(&a)
	if err == nil {
		t.Fatalf("expected error for %+v", a)
	}

	const msg = "did not find object named foo required by field A in type *goject_test.TypeWithMissingNamed"
	if err.Error() != msg {
		t.Fatalf("expected:\n%s\nactual:\n%s", msg, err.Error())
	}
}

func TestCompleteProvides(t *testing.T) {
	var g goject.Container
	var v struct {
		A *TypeAnswerStruct `inject:""`
	}
	if err := g.Provide(&goject.Object{Value: &v, Complete: true}); err != nil {
		t.Fatal(err)
	}

	if err := g.Populate(); err != nil {
		t.Fatal(err)
	}
	if v.A != nil {
		t.Fatal("v.A was not nil")
	}
}

func TestCompleteNamedProvides(t *testing.T) {
	var g goject.Container
	var v struct {
		A *TypeAnswerStruct `inject:""`
	}
	if err := g.Provide(&goject.Object{Value: &v, Complete: true, Name: "foo"}); err != nil {
		t.Fatal(err)
	}

	if err := g.Populate(); err != nil {
		t.Fatal(err)
	}
	if v.A != nil {
		t.Fatal("v.A was not nil")
	}
}

type TypeInjectInterfaceMissing struct {
	Answerable Answerable `inject:""`
}

func TestInjectInterfaceMissing(t *testing.T) {
	var v TypeInjectInterfaceMissing
	err := goject.Populate(&v)
	if err == nil {
		t.Fatal("did not find expected error")
	}

	const msg = "found no assignable value for field Answerable in type *goject_test.TypeInjectInterfaceMissing"
	if err.Error() != msg {
		t.Fatalf("expected:\n%s\nactual:\n%s", msg, err.Error())
	}
}

type TypeInjectInterface struct {
	Answerable Answerable        `inject:""`
	A          *TypeAnswerStruct `inject:""`
}

func TestInjectInterface(t *testing.T) {
	var v TypeInjectInterface
	if err := goject.Populate(&v); err != nil {
		t.Fatal(err)
	}
	if v.Answerable == nil || v.Answerable != v.A {
		t.Fatalf(
			"expected the same but got Answerable = %T %+v / A = %T %+v",
			v.Answerable,
			v.Answerable,
			v.A,
			v.A,
		)
	}
}

type TypeWithInvalidNamedType struct {
	A *TypeNestedStruct `inject:"foo"`
}

func TestInvalidNamedInstanceType(t *testing.T) {
	var g goject.Container
	a := &TypeAnswerStruct{}
	if err := g.Provide(&goject.Object{Value: a, Name: "foo"}); err != nil {
		t.Fatal(err)
	}

	var c TypeWithInvalidNamedType
	if err := g.Provide(&goject.Object{Value: &c}); err != nil {
		t.Fatal(err)
	}

	err := g.Populate()
	if err == nil {
		t.Fatal("did not find expected error")
	}

	const msg = "object named foo of type *goject_test.TypeNestedStruct is not assignable to field A (*goject_test.TypeAnswerStruct) in type *goject_test.TypeWithInvalidNamedType"
	if err.Error() != msg {
		t.Fatalf("expected:\n%s\nactual:\n%s", msg, err.Error())
	}
}

type TypeWithInjectOnPrivateField struct {
	a *TypeAnswerStruct `inject:""`
}

func TestInjectOnPrivateField(t *testing.T) {
	var a TypeWithInjectOnPrivateField
	err := goject.Populate(&a)
	if err == nil {
		t.Fatal("did not find expected error")
	}

	const msg = "inject requested on unexported field a in type *goject_test.TypeWithInjectOnPrivateField"
	if err.Error() != msg {
		t.Fatalf("expected:\n%s\nactual:\n%s", msg, err.Error())
	}
}

type TypeWithInjectOnPrivateInterfaceField struct {
	a Answerable `inject:""`
}

func TestInjectOnPrivateInterfaceField(t *testing.T) {
	var a TypeWithInjectOnPrivateField
	err := goject.Populate(&a)
	if err == nil {
		t.Fatal("did not find expected error")
	}

	const msg = "inject requested on unexported field a in type *goject_test.TypeWithInjectOnPrivateField"
	if err.Error() != msg {
		t.Fatalf("expected:\n%s\nactual:\n%s", msg, err.Error())
	}
}

type TypeInjectPrivateInterface struct {
	Answerable Answerable        `inject:"private"`
	B          *TypeNestedStruct `inject:""`
}

func TestInjectPrivateInterface(t *testing.T) {
	var v TypeInjectPrivateInterface
	err := goject.Populate(&v)
	if err == nil {
		t.Fatal("did not find expected error")
	}

	const msg = "found private inject tag on interface field Answerable in type *goject_test.TypeInjectPrivateInterface"
	if err.Error() != msg {
		t.Fatalf("expected:\n%s\nactual:\n%s", msg, err.Error())
	}
}

type TypeInjectTwoSatisfyInterface struct {
	Answerable Answerable        `inject:""`
	A          *TypeAnswerStruct `inject:""`
	B          *TypeNestedStruct `inject:""`
}

func TestInjectTwoSatisfyInterface(t *testing.T) {
	var v TypeInjectTwoSatisfyInterface
	err := goject.Populate(&v)
	if err == nil {
		t.Fatal("did not find expected error")
	}

	const msg = "found two assignable values for field Answerable in type *goject_test.TypeInjectTwoSatisfyInterface. one type *goject_test.TypeAnswerStruct with value &{0 0} and another type *goject_test.TypeNestedStruct with value"
	if !strings.HasPrefix(err.Error(), msg) {
		t.Fatalf("expected prefix:\n%s\nactual:\n%s", msg, err.Error())
	}
}

type TypeInjectNamedTwoSatisfyInterface struct {
	Answerable Answerable        `inject:""`
	A          *TypeAnswerStruct `inject:""`
	B          *TypeNestedStruct `inject:""`
}

func TestInjectNamedTwoSatisfyInterface(t *testing.T) {
	var g goject.Container
	var v TypeInjectNamedTwoSatisfyInterface
	if err := g.Provide(&goject.Object{Name: "foo", Value: &v}); err != nil {
		t.Fatal(err)
	}

	err := g.Populate()
	if err == nil {
		t.Fatal("was expecting error")
	}

	const msg = "found two assignable values for field Answerable in type *goject_test.TypeInjectNamedTwoSatisfyInterface. one type *goject_test.TypeAnswerStruct with value &{0 0} and another type *goject_test.TypeNestedStruct with value"
	if !strings.HasPrefix(err.Error(), msg) {
		t.Fatalf("expected prefix:\n%s\nactual:\n%s", msg, err.Error())
	}
}

type TypeWithInjectNamedOnPrivateInterfaceField struct {
	a Answerable `inject:""`
}

func TestInjectNamedOnPrivateInterfaceField(t *testing.T) {
	var g goject.Container
	var v TypeWithInjectNamedOnPrivateInterfaceField
	if err := g.Provide(&goject.Object{Name: "foo", Value: &v}); err != nil {
		t.Fatal(err)
	}

	err := g.Populate()
	if err == nil {
		t.Fatal("was expecting error")
	}

	const msg = "inject requested on unexported field a in type *goject_test.TypeWithInjectNamedOnPrivateInterfaceField"
	if err.Error() != msg {
		t.Fatalf("expected:\n%s\nactual:\n%s", msg, err.Error())
	}
}

type TypeWithNonPointerNamedInject struct {
	A int `inject:"foo"`
}

func TestErrorOnNonPointerNamedInject(t *testing.T) {
	var g goject.Container
	if err := g.Provide(&goject.Object{Name: "foo", Value: 42}); err != nil {
		t.Fatal(err)
	}

	var v TypeWithNonPointerNamedInject
	if err := g.Provide(&goject.Object{Value: &v}); err != nil {
		t.Fatal(err)
	}

	if err := g.Populate(); err != nil {
		t.Fatal(err)
	}

	if v.A != 42 {
		t.Fatalf("expected v.A = 42 but got %d", v.A)
	}
}

func TestInjectInline(t *testing.T) {
	var v struct {
		Inline struct {
			A *TypeAnswerStruct `inject:""`
			B *TypeNestedStruct `inject:""`
		} `inject:"inline"`
	}

	if err := goject.Populate(&v); err != nil {
		t.Fatal(err)
	}
	if v.Inline.A == nil {
		t.Fatal("v.Inline.A is nil")
	}
	if v.Inline.B == nil {
		t.Fatal("v.Inline.B is nil")
	}
	if v.Inline.B.A == nil {
		t.Fatal("v.Inline.B.A is nil")
	}
	if v.Inline.A != v.Inline.B.A {
		t.Fatal("got different instances of A")
	}
}

func TestInjectInlineOnPointer(t *testing.T) {
	var v struct {
		Inline *struct {
			A *TypeAnswerStruct `inject:""`
			B *TypeNestedStruct `inject:""`
		} `inject:""`
	}

	if err := goject.Populate(&v); err != nil {
		t.Fatal(err)
	}
	if v.Inline.A == nil {
		t.Fatal("v.Inline.A is nil")
	}
	if v.Inline.B == nil {
		t.Fatal("v.Inline.B is nil")
	}
	if v.Inline.B.A == nil {
		t.Fatal("v.Inline.B.A is nil")
	}
	if v.Inline.A != v.Inline.B.A {
		t.Fatal("got different instances of A")
	}
}

func TestInjectInvalidInline(t *testing.T) {
	var v struct {
		A *TypeAnswerStruct `inject:"inline"`
	}

	err := goject.Populate(&v)
	if err == nil {
		t.Fatal("was expecting an error")
	}

	const msg = `inline requested on non inlined field A in type *struct { A *goject_test.TypeAnswerStruct "inject:\"inline\"" }`
	if err.Error() != msg {
		t.Fatalf("expected:\n%s\nactual:\n%s", msg, err.Error())
	}
}

func TestInjectInlineMissing(t *testing.T) {
	var v struct {
		Inline struct {
			B *TypeNestedStruct `inject:""`
		} `inject:""`
	}

	err := goject.Populate(&v)
	if err == nil {
		t.Fatal("was expecting an error")
	}

	const msg = `inline struct on field Inline in type *struct { Inline struct { B *goject_test.TypeNestedStruct "inject:\"\"" } "inject:\"\"" } requires an explicit "inline" tag`
	if err.Error() != msg {
		t.Fatalf("expected:\n%s\nactual:\n%s", msg, err.Error())
	}
}

type TypeWithInlineStructWithPrivate struct {
	Inline struct {
		A *TypeAnswerStruct `inject:""`
		B *TypeNestedStruct `inject:""`
	} `inject:"private"`
}

func TestInjectInlinePrivate(t *testing.T) {
	var v TypeWithInlineStructWithPrivate
	err := goject.Populate(&v)
	if err == nil {
		t.Fatal("was expecting an error")
	}

	const msg = "cannot use private inject on inline struct on field Inline in type *goject_test.TypeWithInlineStructWithPrivate"
	if err.Error() != msg {
		t.Fatalf("expected:\n%s\nactual:\n%s", msg, err.Error())
	}
}

type TypeWithStructValue struct {
	Inline TypeNestedStruct `inject:"inline"`
}

func TestInjectWithStructValue(t *testing.T) {
	var v TypeWithStructValue
	if err := goject.Populate(&v); err != nil {
		t.Fatal(err)
	}
	if v.Inline.A == nil {
		t.Fatal("v.Inline.A is nil")
	}
}

type TypeWithNonpointerStructValue struct {
	Inline TypeNestedStruct `inject:"inline"`
}

func TestInjectWithNonpointerStructValue(t *testing.T) {
	var v TypeWithNonpointerStructValue
	var g goject.Container
	if err := g.Provide(&goject.Object{Value: &v}); err != nil {
		t.Fatal(err)
	}
	if err := g.Populate(); err != nil {
		t.Fatal(err)
	}
	if v.Inline.A == nil {
		t.Fatal("v.Inline.A is nil")
	}
	n := len(g.Objects())
	if n != 3 {
		t.Fatalf("expected 3 object in Container, got %d", n)
	}

}

func TestPrivateIsFollowed(t *testing.T) {
	var v struct {
		A *TypeNestedStruct `inject:"private"`
	}

	if err := goject.Populate(&v); err != nil {
		t.Fatal(err)
	}
	if v.A.A == nil {
		t.Fatal("v.A.A is nil")
	}
}

func TestDoesNotOverwriteInterface(t *testing.T) {
	a := &TypeAnswerStruct{}
	var v struct {
		A Answerable        `inject:""`
		B *TypeNestedStruct `inject:""`
	}
	v.A = a
	if err := goject.Populate(&v); err != nil {
		t.Fatal(err)
	}
	if v.A != a {
		t.Fatal("original A was lost")
	}
	if v.B == nil {
		t.Fatal("v.B is nil")
	}
}

func TestInterfaceIncludingPrivate(t *testing.T) {
	var v struct {
		A Answerable        `inject:""`
		B *TypeNestedStruct `inject:"private"`
		C *TypeAnswerStruct `inject:""`
	}
	if err := goject.Populate(&v); err != nil {
		t.Fatal(err)
	}
	if v.A == nil {
		t.Fatal("v.A is nil")
	}
	if v.B == nil {
		t.Fatal("v.B is nil")
	}
	if v.C == nil {
		t.Fatal("v.C is nil")
	}
	if v.A != v.C {
		t.Fatal("v.A != v.C")
	}
	if v.A == v.B {
		t.Fatal("v.A == v.B")
	}
}

func TestInjectMap(t *testing.T) {
	var v struct {
		A map[string]int `inject:"private"`
	}
	if err := goject.Populate(&v); err != nil {
		t.Fatal(err)
	}
	if v.A == nil {
		t.Fatal("v.A is nil")
	}
}

type TypeInjectWithMapWithoutPrivate struct {
	A map[string]int `inject:""`
}

func TestInjectMapWithoutPrivate(t *testing.T) {
	var v TypeInjectWithMapWithoutPrivate
	err := goject.Populate(&v)
	if err == nil {
		t.Fatalf("expected error for %+v", v)
	}

	const msg = "inject on map field A in type *goject_test.TypeInjectWithMapWithoutPrivate must be named or private"
	if err.Error() != msg {
		t.Fatalf("expected:\n%s\nactual:\n%s", msg, err.Error())
	}
}

type TypeForObjectString struct {
	A *TypeNestedStruct `inject:"foo"`
	B *TypeNestedStruct `inject:""`
}

func TestObjectString(t *testing.T) {
	var g goject.Container
	a := &TypeNestedStruct{}
	if err := g.Provide(&goject.Object{Value: a, Name: "foo"}); err != nil {
		t.Fatal(err)
	}

	var c TypeForObjectString
	if err := g.Provide(&goject.Object{Value: &c}); err != nil {
		t.Fatal(err)
	}

	if err := g.Populate(); err != nil {
		t.Fatal(err)
	}

	var actual []string
	for _, o := range g.Objects() {
		actual = append(actual, fmt.Sprint(o))
	}

	ensure.SameElements(t, actual, []string{
		"*goject_test.TypeForObjectString",
		"*goject_test.TypeNestedStruct",
		"*goject_test.TypeNestedStruct named foo",
		"*goject_test.TypeAnswerStruct",
	})
}

type TypeForContainerObjects struct {
	TypeNestedStruct `inject:"inline"`
	A                *TypeNestedStruct `inject:"foo"`
	E                struct {
		B *TypeNestedStruct `inject:""`
	} `inject:"inline"`
}

func TestContainerObjects(t *testing.T) {
	var g goject.Container
	err := g.Provide(
		&goject.Object{Value: &TypeNestedStruct{}, Name: "foo"},
		&goject.Object{Value: &TypeForContainerObjects{}},
	)
	ensure.Nil(t, err)
	ensure.Nil(t, g.Populate())

	var actual []string
	for _, o := range g.Objects() {
		actual = append(actual, fmt.Sprint(o))
	}

	ensure.SameElements(t, actual, []string{
		"*goject_test.TypeAnswerStruct",
		"*goject_test.TypeForContainerObjects",
		"*goject_test.TypeNestedStruct named foo",
		"*goject_test.TypeNestedStruct",
		`*struct { B *goject_test.TypeNestedStruct "inject:\"\"" }`,
	})
}

type logger struct {
	Expected []string
	T        testing.TB
	next     int
}

func (l *logger) Debugf(f string, v ...interface{}) {
	actual := fmt.Sprintf(f, v...)
	if l.next == len(l.Expected) {
		l.T.Fatalf(`unexpected log "%s"`, actual)
	}
	expected := l.Expected[l.next]
	if actual != expected {
		l.T.Fatalf(`expected log "%s" got "%s"`, expected, actual)
	}
	l.next++
}

type TypeForLoggingInterface interface {
	Foo()
}

type TypeForLoggingCreated struct{}

func (t TypeForLoggingCreated) Foo() {}

type TypeForLoggingEmbedded struct {
	TypeForLoggingCreated      *TypeForLoggingCreated  `inject:""`
	TypeForLoggingInterface    TypeForLoggingInterface `inject:""`
	TypeForLoggingCreatedNamed *TypeForLoggingCreated  `inject:"name_for_logging"`
	Map                        map[string]string       `inject:"private"`
}

type TypeForLogging struct {
	TypeForLoggingEmbedded `inject:"inline"`
	TypeForLoggingCreated  *TypeForLoggingCreated `inject:""`
}

func TestInjectLogging(t *testing.T) {
	g := goject.Container{
		Logger: &logger{
			Expected: []string{
				"provided *goject_test.TypeForLoggingCreated named name_for_logging",
				"provided *goject_test.TypeForLogging",
				"provided embedded *goject_test.TypeForLoggingEmbedded",
				"created *goject_test.TypeForLoggingCreated",
				"assigned newly created *goject_test.TypeForLoggingCreated to field TypeForLoggingCreated in *goject_test.TypeForLogging",
				"assigned existing *goject_test.TypeForLoggingCreated to field TypeForLoggingCreated in *goject_test.TypeForLoggingEmbedded",
				"assigned *goject_test.TypeForLoggingCreated named name_for_logging to field TypeForLoggingCreatedNamed in *goject_test.TypeForLoggingEmbedded",
				"made map for field Map in *goject_test.TypeForLoggingEmbedded",
				"assigned existing *goject_test.TypeForLoggingCreated to interface field TypeForLoggingInterface in *goject_test.TypeForLoggingEmbedded",
			},
			T: t,
		},
	}
	var v TypeForLogging

	err := g.Provide(
		&goject.Object{Value: &TypeForLoggingCreated{}, Name: "name_for_logging"},
		&goject.Object{Value: &v},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := g.Populate(); err != nil {
		t.Fatal(err)
	}
}

type TypeForNamedWithUnnamedDepSecond struct{}

type TypeForNamedWithUnnamedDepFirst struct {
	TypeForNamedWithUnnamedDepSecond *TypeForNamedWithUnnamedDepSecond `inject:""`
}

type TypeForNamedWithUnnamed struct {
	TypeForNamedWithUnnamedDepFirst *TypeForNamedWithUnnamedDepFirst `inject:""`
}

func TestForNamedWithUnnamed(t *testing.T) {
	var g goject.Container
	var v TypeForNamedWithUnnamed

	err := g.Provide(
		&goject.Object{Value: &v, Name: "foo"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := g.Populate(); err != nil {
		t.Fatal(err)
	}
	if v.TypeForNamedWithUnnamedDepFirst == nil {
		t.Fatal("expected TypeForNamedWithUnnamedDepFirst to be populated")
	}
	if v.TypeForNamedWithUnnamedDepFirst.TypeForNamedWithUnnamedDepSecond == nil {
		t.Fatal("expected TypeForNamedWithUnnamedDepSecond to be populated")
	}
}

func TestForSameNameButDifferentPackage(t *testing.T) {
	var g goject.Container
	err := g.Provide(
		&goject.Object{Value: &injecttesta.Foo{}},
		&goject.Object{Value: &injecttestb.Foo{}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := g.Populate(); err != nil {
		t.Fatal(err)
	}
}

func TestResolveWithInterfaceVariable(t *testing.T) {
	var g goject.Container
	a := &TypeAnswerStruct{answer: 1, private: 2}
	g.Provide(&goject.Object{Value: a})

	var result Answerable

	err := g.Resolve(&result)

	ensure.Nil(t, err)
	ensure.DeepEqual(t, result, a)
}

func TestResolveWithStructVariable(t *testing.T) {
	var g goject.Container
	a := &TypeAnswerStruct{answer: 1, private: 2}
	g.Provide(&goject.Object{Value: a})

	var result TypeAnswerStruct

	err := g.Resolve(&result)

	ensure.Nil(t, err)
	ensure.DeepEqual(t, &result, a)
}

func TestResolveNoProvidedInstanceOfSameType(t *testing.T) {
	var g goject.Container
	l := &logger{}
	g.Provide(&goject.Object{Value: l})

	var result TypeAnswerStruct

	err := g.Resolve(&result)

	ensure.NotNil(t, err)
	ensure.DeepEqual(t, err.Error(), "No provided object is assignable to dst")
}

func TestResolveWithNoPointer(t *testing.T) {
	var g goject.Container
	a := &TypeAnswerStruct{answer: 1, private: 2}
	g.Provide(&goject.Object{Value: a})

	var result TypeAnswerStruct

	err := g.Resolve(result)

	ensure.NotNil(t, err)
	ensure.DeepEqual(t, err.Error(), "dst its not a pointer")
}

func TestResolveByNameWithInterfaceVariable(t *testing.T) {
	var g goject.Container
	a := &TypeAnswerStruct{answer: 1, private: 2}
	const name = "name"
	g.Provide(&goject.Object{Name: name, Value: a})

	var result Answerable

	err := g.ResolveByName(&result, name)

	ensure.Nil(t, err)
	ensure.DeepEqual(t, result, a)
}

func TestResolveByNameWithStructVariable(t *testing.T) {
	var g goject.Container
	a := &TypeAnswerStruct{answer: 1, private: 2}
	const name = "name"
	g.Provide(&goject.Object{Name: name, Value: a})

	var result TypeAnswerStruct

	err := g.ResolveByName(&result, name)

	ensure.Nil(t, err)
	ensure.DeepEqual(t, &result, a)
}

func TestResolveByNameNoProvidedInstanceOfSameType(t *testing.T) {
	var g goject.Container
	l := &logger{}
	const name = "name"
	g.Provide(&goject.Object{Name: name, Value: l})

	var result TypeAnswerStruct

	err := g.ResolveByName(&result, name)

	ensure.NotNil(t, err)
	ensure.DeepEqual(t, err.Error(), "No provided object is assignable to dst")
}

func TestResolveByNameNoProvidedInstanceWithSameName(t *testing.T) {
	var g goject.Container
	a := &TypeAnswerStruct{answer: 1, private: 2}
	const name = "name"
	g.Provide(&goject.Object{Value: a})

	var result TypeAnswerStruct

	err := g.ResolveByName(&result, name)

	ensure.NotNil(t, err)
	ensure.DeepEqual(t, err.Error(), fmt.Sprintf("No provided object with the name: %s", name))
}

func TestResolveByNameWithNoPointer(t *testing.T) {
	var g goject.Container
	a := &TypeAnswerStruct{answer: 1, private: 2}
	const name = "name"
	g.Provide(&goject.Object{Name: name, Value: a})

	var result TypeAnswerStruct

	err := g.ResolveByName(result, name)

	ensure.NotNil(t, err)
	ensure.DeepEqual(t, err.Error(), "dst its not a pointer")
}
