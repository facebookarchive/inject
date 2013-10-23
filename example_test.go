package inject_test

import (
	"fmt"
	"net/http"
	"os"

	"github.com/daaku/go.inject"
)

// Our Awesome Application renders a message using two APIs in our fake world.
type AwesomeApp struct {
	// The tags below indicate to the inject library that these fields are
	// eligible for injection. They do not specify any options, and will result
	// in a singleton instance created for each of the APIs.

	NameAPI   *NameAPI   `inject:""`
	PlanetAPI *PlanetAPI `inject:""`
}

func (a *AwesomeApp) Render(id uint64) string {
	return fmt.Sprintf(
		"%s is from the planet %s.",
		a.NameAPI.Name(id),
		a.PlanetAPI.Planet(id),
	)
}

// Our fake Name API.
type NameAPI struct {
	// Here and below in PlanetAPI we add the tag to an interface value. This
	// value cannot automatically be created (by definition) and hence must be
	// explicitly provided to the graph.

	HttpTransport http.RoundTripper `inject:""`
}

func (n *NameAPI) Name(id uint64) string {
	// in the real world we would use f.HttpTransport and fetch the name
	return "Spock"
}

// Our fake Planet API.
type PlanetAPI struct {
	HttpTransport http.RoundTripper `inject:""`
}

func (p *PlanetAPI) Planet(id uint64) string {
	// in the real world we would use f.HttpTransport and fetch the planet
	return "Vulcan"
}

func Example() {
	// Typically an application will have exactly one object graph per
	// "application" or "server". Traditionally you will create the graph and use
	// it within a main function:
	var g inject.Graph

	// We Populate our world with two "seed" objects, one our empty AwesomeApp
	// instance which we're hoping to get filled out:
	var a AwesomeApp
	if err := g.Provide(inject.Object{Value: &a}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// And second our DefaultTransport to satisfiy our HttpTransport dependency.
	// We have to provide the DefaultTransport because the dependency is defined
	// in terms of the http.RoundTripper interface, and since it is an interface
	// the library cannot create an instance for it. Instead it will use the
	// given DefaultTransport to satisfy the dependency since it satisfies the
	// interface:
	if err := g.Provide(inject.Object{Value: http.DefaultTransport}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// Here the Populate call is creating instances of NameAPI & PlanetAPI, and
	// setting the HttpTransport on both of those to the http.DefaultTransport
	// provided above:
	if err := g.Populate(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// There is a shorthand API for the simple case which combines the three
	// calls above and is available as inject.Populate. The above API also allows
	// the use of named instances for more complex scenarios.

	fmt.Println(a.Render(42))

	// Output: Spock is from the planet Vulcan.
}
