package plugin

import (
	"fmt"
	"reflect"
	"sync"

	"fahrradverleih/framework/domain"
)

// Registry verwaltet die registrierten Verleiharten (Spezifikation 3.3). Sie ist threadsicher.
type Registry struct {
	mu      sync.RWMutex
	byKey   map[string]Plugin
	ordered []Plugin
}

func NewRegistry() *Registry { return &Registry{byKey: map[string]Plugin{}} }

// Register trägt ein Plugin ein, nachdem es auf Vollständigkeit geprüft wurde (AK-19).
func (r *Registry) Register(p Plugin) error {
	if err := check(p); err != nil {
		return err
	}
	key := p.Kind().Key
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.byKey[key]; exists {
		return fmt.Errorf("%w: %q", domain.ErrDuplicateKind, key)
	}
	r.byKey[key] = p
	r.ordered = append(r.ordered, p)
	return nil
}

// Get liefert das Plugin zu einem Schlüssel oder ErrUnknownKind.
func (r *Registry) Get(key string) (Plugin, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.byKey[key]
	if !ok {
		return nil, fmt.Errorf("%w: %q", domain.ErrUnknownKind, key)
	}
	return p, nil
}

// All liefert alle Plugins in Registrierungsreihenfolge.
func (r *Registry) All() []Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]Plugin(nil), r.ordered...)
}

func check(p Plugin) error {
	if p == nil || isNilPointer(p) {
		return fmt.Errorf("%w: nil", domain.ErrInvalidPlugin)
	}
	key := p.Kind().Key
	if !validKey(key) {
		return fmt.Errorf("%w: Schlüssel %q ist nicht URL-tauglich", domain.ErrInvalidPlugin, key)
	}
	if len(p.Categories()) == 0 {
		return fmt.Errorf("%w: %q hat keine Kategorien", domain.ErrInvalidPlugin, key)
	}
	if s := p.Pricing(); s == nil || isNilPointer(s) {
		return fmt.Errorf("%w: %q hat keine Preisstrategie", domain.ErrInvalidPlugin, key)
	}
	seen := map[string]bool{}
	for _, c := range p.Categories() {
		if !validKey(c.Key) || seen[c.Key] {
			return fmt.Errorf("%w: %q hat eine ungültige oder doppelte Kategorie %q", domain.ErrInvalidPlugin, key, c.Key)
		}
		seen[c.Key] = true
	}
	seen = map[string]bool{}
	for _, f := range p.Fields() {
		if f.Key == "" || seen[f.Key] {
			return fmt.Errorf("%w: %q hat ein leeres oder doppeltes Merkmal %q", domain.ErrInvalidPlugin, key, f.Key)
		}
		if f.Type == domain.FieldChoice && len(f.Options) == 0 {
			return fmt.Errorf("%w: Merkmal %q hat keine Optionen", domain.ErrInvalidPlugin, f.Key)
		}
		seen[f.Key] = true
	}
	return nil
}

// validKey erlaubt [a-z0-9-]+ und [a-z0-9_]+, damit Schlüssel in URLs und Formularen funktionieren.
func validKey(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' && r != '_' {
			return false
		}
	}
	return true
}

// isNilPointer erkennt einen typisierten nil-Zeiger in einem Interface.
func isNilPointer(v any) bool {
	rv := reflect.ValueOf(v)
	return rv.Kind() == reflect.Pointer && rv.IsNil()
}
