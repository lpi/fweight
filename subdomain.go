package fweight

import (
	"net/http"
	"strings"
)

/*
	Internally, the domain system is based on the DomainRouter interface.
	A DomainRouter will check an assertion that the Router that is recieved
	from the Subdomain function is a DomainRouter. If is, the RouteHTTP
	function continues down the trie until this is not the case.
*/
type DomainRouter interface {
	Subdomain(subpath string) (s Router, remainingDomain string)
	Router
}

var _ DomainRouter = &SubdomainRouter{}
var _ Router = &SubdomainRouter{}
var _ Router = &wildcardDomain{}
var _ DomainRouter = &wildcardDomain{}

/*
	Type wildcardDomain is a DomainRouter that
	simply removes the highest level domain
	from the current subdomain string and then returns
	its embedded Subdomain's decision on the rest of
	the domain, allowing urls like documents.* or even
	documents.*.* by nesting wildcardDomains.
*/
type wildcardDomain struct {
	SubdomainRouter
}

//	Function AnyDomainThen wraps wildcardDomain:
//	if Router d represented the DomainRouter "a.b",
//	AnyDomainThen represents domains matching
//	"*.a.b".
//
//	Correspondigly, if d represented the PathRouter
//	"a/b", AnyDomainThen(d) represents "*/a/b"
func AnyDomainThen(s SubdomainRouter) DomainRouter {
	return wildcardDomain{
		s,
	}
}

/*
	Because of the way the DNS works, domains such as ., .. or ... are valid,
	coming from addresses like "google.com.", "google.com.." and "google.com..."
	respecitvely. These domains are said to be "empty" because they do not resolve
	any further than "google.com".
*/
func SubdomainEmpty(subdomain string) bool {
	if subdomain == "" || strings.TrimRight(subdomain, ".") == "" {
		return true
	}
	return false
}

func (w wildcardDomain) RouteHTTP(rq *http.Request) Router {
	rq.Host, _ = popLevel(rq.Host)
	return w.SubdomainRouter.RouteHTTP(rq)
}

func (w wildcardDomain) Subdomain(subpath string) (s Router, remainingDomain string) {

	//then we stop here, this is the last child.
	subpath, _ = popLevel(subpath)
	return w.SubdomainRouter, subpath
}

//function removeLevel removes the highest level domain from a domain name
func popLevel(domain string) (newDomain, oldLevel string) {
	var lastdot uint
	for i, v := range domain {
		if v == '.' {
			lastdot = uint(i)
		}
	}
	return domain[:lastdot], domain[lastdot+1:]
}

/*
	Subdomain implements the DomainRouter interface and
	is used to route requests to subdomain trees and the paths
	below them.

	The empty subdomain ("") is used when the route terminates here.
*/
type SubdomainRouter map[string]Router

const termHere = ""

func (s SubdomainRouter) Here(r Router) {
	s[termHere] = r
}

func (s SubdomainRouter) here() Router {
	return s[termHere]
}

func removeSubdomain(subd, path string) (s string) {
	s = strings.TrimSuffix(path, subd)
	if len(s) > 0 && s[0] == '.' {
		s = s[1:]
	}
	return
}

// isSubdomin returns the value of the assertion
// `if r implements SubdomainRouter`. If the assertion is true,
// sd is set to the DomainRouter of r.
func isSubdomain(r Router, sd DomainRouter) (b bool) {
	sd, b = r.(DomainRouter)
	return
}

/*
	RouteHTTP completes a route down a series of DomainRouters.
	Once the router is no longer a DomainRouter, it is returned.

*/
func (s SubdomainRouter) RouteHTTP(rq *http.Request) Router {
	currentSubdomain, currentRouter := s, Router(nil)
	var domain string
	//Initially, the host provides the domain steing.
	for currentRouter, domain = s.Subdomain(rq.Host);
	/*
		Effectively
			currentSubdomain, result = currentRouter.(DomainRouter)
			return result
	*/
	isSubdomain(currentRouter, currentSubdomain); currentRouter, domain = currentSubdomain.Subdomain(domain) {
	}
	return currentRouter
}

func (s SubdomainRouter) Subdomain(subpath string) (Router, string) {
	//If the subpath is "empty", then we return this Subdomain's PathRouter
	if SubdomainEmpty(subpath) {
		return s[termHere], ""
	}

	//Check if we have bound a handler for the entire remaining route.
	if sD, ok := s[subpath]; ok {
		//Nothing left.
		return sD, ""
	}

	//Check if the next node is present
	var cSubpath, cLevel string
	cSubpath, cLevel = popLevel(subpath)
	if rT, ok := s[cLevel]; ok {
		return rT, cSubpath
	}

	//If the requested domain is the suffix of the current domain
	//strip off that component as per its map.
	for subDomain, router := range s {
		if strings.HasSuffix(subpath, subDomain) {
			return router, removeSubdomain(subDomain, subpath)
		}
	}
	return nil, subpath
}

func (s SubdomainRouter) Domain(name string, r Router) SubdomainRouter {
	if s == nil {
		s = map[string]Router{
			name: r,
		}
	} else {
		s[name] = r
	}
	return s
}
