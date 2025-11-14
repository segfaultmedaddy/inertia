package inertia

import (
	"cmp"
	"errors"
	"fmt"
	"reflect"
	"strings"
)

const (
	TagInertia      = "inertia"
	TagInertiaGroup = "inertiagroup"
)

var (
	propTypeOptional = "optional" //nolint:gochecknoglobals
	propTypeDeferred = "deferred" //nolint:gochecknoglobals
	propTypeAlways   = "always"   //nolint:gochecknoglobals
)

var (
	propDiscard    = "-"          //nolint:gochecknoglobals
	propOmitEmpty  = "omitempty"  //nolint:gochecknoglobals
	propMergeable  = "mergeable"  //nolint:gochecknoglobals
	propConcurrent = "concurrent" //nolint:gochecknoglobals
)

var lazyType = reflect.TypeFor[Lazy]() //nolint:gochecknoglobals

// ParseStruct converts a struct into a Props collection using struct tags.
// It expects a struct pointer with JSON-encodable fields.
//
// Only fields tagged with "inertia" are included; untagged fields are ignored.
//
// Tag format: `inertia:"name[,type][,mergeable][,concurrent][,omitempty]"`
//
// Tag components:
//   - name: Prop name sent to client (required). Use "-" to skip the field.
//   - type: One of "optional", "deferred", "always", or empty (regular prop)
//   - mergeable: Include literal "mergeable" to enable merge behavior
//   - concurrent: Include literal "concurrent" for parallel resolution (deferred props only)
//   - omitempty: Include literal "omitempty" to skip zero-value fields
//
// Prop types:
//   - (empty): Regular prop, included on initial and partial renders
//   - "optional": Lazy prop, resolved only when explicitly requested
//   - "deferred": Lazy prop, loaded after initial render in named groups
//   - "always": Always included, ignores partial reload filters
//
// Deferred prop grouping:
//
//	Use `inertiagroup:"groupname"` to assign deferred props to named groups.
//	Props in the same group are resolved together. Defaults to "default" group.
//	Returns an error if inertiagroup is used on non-deferred props.
//
// Field value requirements:
//   - Optional/deferred fields must be Lazy or LazyFunc type
//   - Regular/always fields can be any JSON-serializable type
//
// Example:
//
//	type PageProps struct {
//	    UserID    int              `inertia:"user_id,always"`
//	    Posts     []Post           `inertia:"posts"`
//	    Analytics LazyFunc         `inertia:"analytics,deferred,concurrent" inertiagroup:"metrics"`
//	    Optional  LazyFunc         `inertia:"extra,optional,omitempty"`
//	}
func ParseStruct(v any) (Props, error) {
	val := reflect.ValueOf(v)
	if val.Kind() != reflect.Ptr {
		return nil, errors.New("msg must be a pointer")
	}

	val = val.Elem()
	if val.Kind() != reflect.Struct {
		return nil, errors.New("msg must be a struct")
	}

	typ := val.Type()
	numFields := typ.NumField()
	props := make(Props, 0, numFields)

	for i := range numFields {
		field := typ.Field(i)
		fieldVal := val.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		inertiaTag := field.Tag.Get(TagInertia)
		if inertiaTag == "" {
			continue
		}

		// Get the inertiaGroup tag, if any
		inertiaGroup := field.Tag.Get(TagInertiaGroup)

		fieldName := field.Name
		fieldType := ""
		mergeable := false
		concurrent := false

		// If tag is not empty, parse it
		if inertiaTag != "" {
			parts := strings.Split(inertiaTag, ",")

			if parts[0] != "" {
				fieldName = parts[0]
			}

			// Check if the field should be discarded.
			if fieldName == propDiscard {
				continue
			}

			// Second part is the field type (optional, deferred, always)
			if len(parts) > 1 {
				fieldType = parts[1]
			}

			// Third part is mergeable flag
			if len(parts) > 2 && parts[2] == propMergeable {
				mergeable = true
			}

			// Fourth part is concurrent flag
			if len(parts) > 3 && parts[3] == propConcurrent {
				concurrent = true
			}

			// Skip empty fields if omitempty is presented.
			if parts[len(parts)-1] == propOmitEmpty {
				if fieldVal.IsZero() {
					continue
				}
			}
		}

		// Check if field can be accessed
		if !fieldVal.CanInterface() {
			continue
		}

		// Add to the appropriate prop map
		if inertiaGroup != "" && fieldType != propTypeDeferred {
			return nil, errors.New("inertiaframe: cannot use group tag on non-deferred field")
		}

		var prop Prop

		switch fieldType {
		case propTypeOptional:
			fn, err := toLazy(fieldVal)
			if err != nil {
				return nil, err
			}

			prop = NewOptional(fieldName, fn)
		case propTypeDeferred:
			fn, err := toLazy(fieldVal)
			if err != nil {
				return nil, err
			}

			prop = NewDeferred(
				fieldName,
				fn,
				&DeferredOptions{
					Merge:      mergeable,
					Group:      cmp.Or(inertiaGroup, DefaultDeferredGroup),
					Concurrent: concurrent,
				},
			)
		case propTypeAlways:
			prop = NewAlways(fieldName, fieldVal.Interface())
		case "":
			prop = NewProp(
				fieldName,
				fieldVal.Interface(),
				&PropOptions{Merge: mergeable},
			)
		default:
			return nil, fmt.Errorf("inertiaframe: unknown field type %q", fieldType)
		}

		props = append(props, prop)
	}

	return props, nil
}

// toLazy converts a reflect.Value to an Lazy
// if the value is Lazy convertible.
func toLazy(v reflect.Value) (Lazy, error) {
	val := v.Interface()
	if v.Kind() == reflect.Interface && v.Type().Implements(lazyType) {
		lazy, ok := val.(Lazy)
		if !ok {
			return nil, errors.New("inertiaframe: invalid lazy value")
		}

		return lazy, nil
	}

	if v.Kind() == reflect.Func {
		lazyFn, ok := val.(LazyFunc)
		if !ok {
			return nil, errors.New("inertiaframe: invalid lazy function")
		}

		return lazyFn, nil
	}

	return nil, errors.New("inertiaframe: invalid lazy value")
}
