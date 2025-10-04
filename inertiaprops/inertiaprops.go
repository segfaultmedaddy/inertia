package inertiaprops

import "go.inout.gg/inertia"

var _ inertia.Proper = (*Map)(nil)

// Map is a convenient map-based Proper implementation for simple key-value props.
// All values are treated as regular props (not lazy, deferred, or always).
//
// For advanced prop options (lazy loading, merging, always-include), use inertia.NewProp
// or inertia.ParseStruct instead.
type Map map[string]any

func (m Map) Props() []inertia.Prop {
	props := make([]inertia.Prop, 0, len(m))
	for k, v := range m {
		props = append(props, inertia.NewProp(k, v, nil))
	}

	return props
}

func (m Map) Len() int { return len(m) }
