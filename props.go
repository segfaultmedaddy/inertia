package inertia

import (
	"cmp"
	"context"
)

var (
	_ Proper = (Props)(nil)
	_ Proper = (*Prop)(nil)
)

const DefaultDeferredGroup = "default"

// Prop represents a single property passed to an Inertia page component.
// Props control data visibility, lazy loading, merging behavior, and resolution timing.
//
// Create props using constructor functions:
//   - NewProp: Standard prop, included on initial render
//   - NewAlways: Always included, ignores partial reload filters
//   - NewOptional: Lazy-loaded, only resolved when explicitly requested
//   - NewDeferred: Lazy-loaded in groups, supports concurrent resolution
//
// Attach props to a page using WithProps option.
type Prop struct {
	val        any
	valFn      Lazy // optional, deferred
	key        string
	group      string // deferred
	mergeable  bool
	deferred   bool
	lazy       bool // optional, deferred
	ignorable  bool // false if always prop
	concurrent bool // deferred
}

// DeferredOptions configures the behavior of deferred props.
type DeferredOptions struct {
	// Group assigns this prop to a named deferred group.
	// Props in the same group are resolved together when requested by the client.
	// Defaults to DefaultDeferredGroup if not specified.
	Group string

	// Merge determines how updates are handled on partial reloads.
	// If true, the prop value is merged with the existing client-side value.
	// If false, the value is replaced entirely. Defaults to false.
	Merge bool

	// Concurrent enables parallel resolution for this prop.
	// When true, this prop can be resolved concurrently with other concurrent props
	// within the same request, up to the configured concurrency limit.
	Concurrent bool
}

type (
	// Lazy represents a prop value that is resolved on-demand rather than eagerly.
	// Used for optional and deferred props to avoid unnecessary computation.
	Lazy interface {
		// Value resolves and returns the prop's value.
		// The returned value must be JSON-serializable.
		Value(context.Context) (any, error)
	}

	// LazyFunc is a function adapter that implements the Lazy interface.
	// It allows using ordinary functions as lazy prop values.
	// The returned value must be JSON-serializable.
	LazyFunc func(context.Context) (any, error)
)

// Value calls `fn()`.
func (fn LazyFunc) Value(ctx context.Context) (any, error) { return fn(ctx) }

// NewDeferred creates a deferred prop that is lazy-loaded by the client after initial render.
// Deferred props reduce initial page load time by deferring expensive computations.
//
// The client can request deferred props individually or by group.
// If opts is nil, default options are used (default group, no merging, sequential resolution).
func NewDeferred(key string, fn Lazy, opts *DeferredOptions) Prop {
	//nolint:exhaustruct
	prop := Prop{
		deferred:   true, // important
		lazy:       true, // important
		ignorable:  true, // important
		key:        key,
		valFn:      fn,
		group:      DefaultDeferredGroup,
		concurrent: false,
	}

	if opts != nil {
		prop.group = cmp.Or(opts.Group, DefaultDeferredGroup)
		prop.mergeable = opts.Merge
		prop.concurrent = opts.Concurrent
	}

	return prop
}

// NewAlways creates a prop that is always included in responses.
// Unlike regular props, it ignores partial reload filters (X-Inertia-Partial-Data/Except headers).
// Use for critical data that must always be present, such as authentication state or global config.
func NewAlways(key string, value any) Prop {
	//nolint:exhaustruct
	return Prop{
		ignorable: false, // important
		key:       key,
		val:       value,
	}
}

// NewOptional creates a lazily-evaluated prop included only during partial reloads when explicitly requested.
// Useful for expensive computations that aren't needed on every render.
// The value function is only called when the client specifically requests this prop.
func NewOptional(key string, fn Lazy) Prop {
	//nolint:exhaustruct
	return Prop{
		ignorable: true, // important
		lazy:      true, // important
		key:       key,
		valFn:     fn,
	}
}

// PropOptions configures standard prop behavior.
type PropOptions struct {
	// Merge determines whether this prop's value is merged or replaced during partial reloads.
	Merge bool
}

// NewProp creates a standard prop included on initial page load and partial reloads.
// If opts is nil, default options are used (no merging).
func NewProp(key string, val any, opts *PropOptions) Prop {
	//nolint:exhaustruct
	prop := Prop{
		ignorable: true, // important
		key:       key,
		val:       val,
	}

	if opts != nil {
		prop.mergeable = opts.Merge
	}

	return prop
}

func (p Prop) Props() []Prop { return []Prop{p} }
func (p Prop) Len() int      { return 1 }

// value returns the prop value.
func (p Prop) value(ctx context.Context) (any, error) {
	if p.valFn != nil {
		v, err := p.valFn.Value(ctx)
		if err != nil {
			return nil, err //nolint:wrapcheck
		}

		return v, nil
	}

	return p.val, nil
}

// Proper represents a collection of props that can be attached to a render context.
// Implemented by both individual Prop and Props slice types.
type Proper interface {
	// Props returns the underlying prop slice.
	Props() []Prop

	// Len returns the number of props in the collection.
	Len() int
}

// Props is a collection of props.
type Props []Prop

func (p Props) Len() int      { return len(p) }
func (p Props) Props() []Prop { return p }
