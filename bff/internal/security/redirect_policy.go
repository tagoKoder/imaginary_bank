// bff\internal\security\redirect_policy.go
package security

import (
	"net/url"
	"path"
	"strings"
)

type RedirectPolicy struct {
	allowExact   map[string]struct{}
	allowPrefix  []string
	defaultRoute string
}

func NewRedirectPolicy(allowlist []string) *RedirectPolicy {
	p := &RedirectPolicy{
		allowExact:   make(map[string]struct{}),
		allowPrefix:  []string{},
		defaultRoute: "/",
	}

	for _, raw := range allowlist {
		s := normalizePath(raw)
		if s == "" {
			continue
		}
		// Convención: si termina en "/*" tratamos como prefijo.
		if strings.HasSuffix(s, "/*") {
			p.allowPrefix = append(p.allowPrefix, strings.TrimSuffix(s, "/*"))
			continue
		}
		p.allowExact[s] = struct{}{}
	}

	// Siempre permitir "/" por seguridad/UX
	p.allowExact["/"] = struct{}{}
	return p
}

// Safe valida y devuelve una ruta relativa segura (misma-origin) o "/".
func (p *RedirectPolicy) Safe(target string) string {
	if target == "" {
		return p.defaultRoute
	}

	// Prohíbe URLs absolutas o esquemas
	u, err := url.Parse(target)
	if err != nil {
		return p.defaultRoute
	}
	if u.IsAbs() || u.Scheme != "" || u.Host != "" {
		return p.defaultRoute
	}

	// Normaliza path y preserva query/fragment si aplica.
	clean := normalizePath(u.Path)
	if clean == "" {
		clean = "/"
	}

	if _, ok := p.allowExact[clean]; ok {
		u.Path = clean
		return u.String()
	}
	for _, pref := range p.allowPrefix {
		if pref == "" {
			continue
		}
		if clean == pref || strings.HasPrefix(clean, pref+"/") {
			u.Path = clean
			return u.String()
		}
	}

	return p.defaultRoute
}

func normalizePath(pth string) string {
	if pth == "" {
		return ""
	}
	// Debe ser relativo interno
	if !strings.HasPrefix(pth, "/") {
		return ""
	}
	// Evita // o cosas raras
	pth = path.Clean(pth)
	if !strings.HasPrefix(pth, "/") {
		return ""
	}
	return pth
}
