package inject_test

import (
	"fmt"
	"net/http"
	"os"

	"github.com/daaku/go.inject"
)

// Our Awesome Application renders a message using two APIs in our fake world.
type AwesomeApp struct {
	NameAPI   *NameAPI   `inject:""`
	PlanetAPI *PlanetAPI `inject:""`
}

func (a *AwesomeApp) Render(id uint64) string {
	return fmt.Sprintf(
		"%s is from the %s planet.",
		a.NameAPI.Name(id),
		a.PlanetAPI.Planet(id),
	)
}

// Our fake Name API.
type NameAPI struct {
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
	// in the real world we would use f.HttpTransport and fetch the name
	return "Vulcan"
}

func Example() {
	// We Populate our world with two "seed" objects, one our empty AwesomeApp
	// instance which we're hoping to get filled out, and second our
	// DefaultTransport to satisfiy our HttpTransport dependency. We have to
	// provide the DefaultTransport because the dependency is defined in terms of
	// the http.RoundTripper interface, and since it is an interface the library
	// cannot create an instance for it. Instead it will use the given
	// DefaultTransport to satisfy the dependency since it satisfies the
	// interface.
	//
	// Here the inject call is creating instances of NameAPI & PlanetAPI, and
	// setting the HttpTransport on both of those to the http.DefaultTransport
	// provided below.
	var a AwesomeApp
	if err := inject.Populate(&a, http.DefaultTransport); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	fmt.Println(a.Render(42))

	// Output: Spock is from the Vulcan planet.
}
