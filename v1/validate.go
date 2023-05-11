package validate

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/bww/epl/v1"
	"github.com/bww/go-validate/v1/stdlib"
	lru "github.com/hashicorp/golang-lru/v2"
)

const dfltCache = 1024

var (
	exprCache *lru.Cache[string, *epl.Program]
	typeCache *lru.Cache[typeKey, *validatedType]
)

var (
	debug  = os.Getenv("VALIDATE_DEBUG") != ""
	strict = os.Getenv("VALIDATE_STRICT") != ""
)

func sizeFromEnv(n string, d int) int {
	if v := os.Getenv(n); v != "" {
		var err error
		d, err = strconv.Atoi(v)
		if err != nil {
			panic(fmt.Errorf("validate: Cache size is not an integer: %q, %v", n, err))
		}
		if d < 0 {
			panic(fmt.Errorf("validate: Cache size makes no sense: %d", d))
		}
	}
	return d
}

func init() {
	if size := sizeFromEnv("GO_VALIDATE_EXPR_CACHE_SIZE", dfltCache); size > 0 {
		var err error
		exprCache, err = lru.New[string, *epl.Program](size) // in practice this cannot fail because we've checked that size > 0
		if err != nil {
			panic(fmt.Errorf("validate: Could not create expression cache: %v", err))
		}
	}
	if size := sizeFromEnv("GO_VALIDATE_TYPE_CACHE_SIZE", dfltCache); size > 0 {
		var err error
		typeCache, err = lru.New[typeKey, *validatedType](size) // in practice this cannot fail because we've checked that size > 0
		if err != nil {
			panic(fmt.Errorf("validate: Could not create type cache: %v", err))
		}
	}
}

type errorBuffer struct {
	E []error
}

func (e *errorBuffer) Len() int {
	return len(e.E)
}

func (e *errorBuffer) Add(v ...error) {
	e.E = append(e.E, v...)
}

func keyPath(b, f string) string {
	if b != "" {
		return fmt.Sprintf("%s.%s", b, f)
	} else {
		return f
	}
}

func indexPath(f string, n int) string {
	return fmt.Sprintf("%s[%d]", f, n)
}

func altsPath(b string, f []string) string {
	return keyPath(b, fmt.Sprintf("{%s}", strings.Join(f, ",")))
}

type Context struct {
	Path string
}

// WithPath returns a new context based on the receiver with the Path
// field replaced by the provided value.
func (c Context) WithPath(p string) Context {
	return Context{Path: p}
}

// WithField returns a new context based on the receiver with the Path
// field replaced by the current path with the provided field appended.
func (c Context) WithField(f string) Context {
	return Context{Path: keyPath(c.Path, f)}
}

// WithFieldAlts returns a new context based on the receiver with the Path
// field replaced by the current path with the provided fields alternates
// appended.
func (c Context) WithFieldAlternates(f ...string) Context {
	return Context{Path: altsPath(c.Path, f)}
}

// WithField returns a new context based on the receiver with the Path
// field replaced by the current path with the provided index subscript
// appended.
func (c Context) WithIndex(v int) Context {
	return Context{Path: indexPath(c.Path, v)}
}

// FieldError creates a new field error from this context and the provided
// error.
func (c Context) FieldError(err error) *FieldError {
	return newFieldError(c.Path, err)
}

// FieldErrorf creates a new field error from this context and the provided
// error message.
func (c Context) FieldErrorf(m string, a ...interface{}) *FieldError {
	return FieldErrorf(c.Path, m, a...)
}

// IntrospectorV1 is a deprecated interface for validating types.
type IntrospectorV1 interface {
	Validate() error
}

// IntrospectorV2 is a deprecated interface for validating types.
type IntrospectorV2 interface {
	Validate(Validator) (error, bool)
}

// IntrospectorV3 can be implemented by a type to perform arbitrary
// custom validation.
type IntrospectorV3 interface {
	Validate(Validator, Context) (error, bool)
}

var (
	introspectorV1 = reflect.TypeOf((*IntrospectorV1)(nil)).Elem()
	introspectorV2 = reflect.TypeOf((*IntrospectorV2)(nil)).Elem()
	introspectorV3 = reflect.TypeOf((*IntrospectorV3)(nil)).Elem()
)

type Validator struct {
	checkTag, errTag, nameTag, basePath string
}

func New(opts ...Option) Validator {
	return NewWithConfig(Config{
		CheckTag: "check",
		ErrorTag: "invalid",
		FieldTag: "json",
		BasePath: "",
	}.WithOptions(opts))
}

func NewWithConfig(conf Config) Validator {
	return Validator{
		checkTag: conf.CheckTag,
		errTag:   conf.ErrorTag,
		nameTag:  conf.FieldTag,
		basePath: conf.BasePath,
	}
}

func (v Validator) WithOptions(opts ...Option) Validator {
	return NewWithConfig(Config{
		CheckTag: v.checkTag,
		ErrorTag: v.errTag,
		FieldTag: v.nameTag,
		BasePath: v.basePath,
	}.WithOptions(opts))
}

func (v Validator) Validate(s interface{}) Errors {
	errs := &errorBuffer{}
	v.validate(v.basePath, reflect.ValueOf(s), errs)
	return errs.E
}

func (v Validator) validate(p string, s reflect.Value, errs *errorBuffer) bool {
	s = reflect.Indirect(s)
	t := s.Type()
	switch {
	case t.Implements(introspectorV3):
		return v.validateIntrospectorV3(p, s, errs)
	case t.Implements(introspectorV2):
		return v.validateIntrospectorV2(p, s, errs)
	case t.Implements(introspectorV1):
		return v.validateIntrospectorV1(p, s, errs)
	default:
		return v.validateFields(p, s, errs)
	}
}

func (v Validator) validateIntrospectorV1(p string, s reflect.Value, errs *errorBuffer) bool {
	r := s.MethodByName("Validate").Call([]reflect.Value{})
	if err := unwrapError(r[0]); err != nil {
		errs.Add(fieldErrors(p, err)...)
		return false
	}
	return true
}

func (v Validator) validateIntrospectorV2(p string, s reflect.Value, errs *errorBuffer) bool {
	var valid bool
	r := s.MethodByName("Validate").Call([]reflect.Value{reflect.ValueOf(v)})
	if err := unwrapError(r[0]); err != nil {
		errs.Add(fieldErrors(p, err)...)
	} else {
		valid = true
	}
	if r[1].Bool() {
		return v.validateFields(p, s, errs) && valid
	} else {
		return valid
	}
}

func (v Validator) validateIntrospectorV3(p string, s reflect.Value, errs *errorBuffer) bool {
	var valid bool
	c := Context{Path: p}
	r := s.MethodByName("Validate").Call([]reflect.Value{reflect.ValueOf(v), reflect.ValueOf(c)})
	if err := unwrapError(r[0]); err != nil {
		errs.Add(fieldErrors(p, err)...)
	} else {
		valid = true
	}
	if r[1].Bool() {
		return v.validateFields(p, s, errs) && valid
	} else {
		return valid
	}
}

func (v Validator) validateFields(p string, s reflect.Value, errs *errorBuffer) bool {
	switch s.Kind() {
	case reflect.Interface, reflect.Pointer:
		return v.validateFields(p, s.Elem(), errs)
	case reflect.Struct:
		return v.validateStruct(p, s, errs)
	case reflect.Slice, reflect.Array:
		return v.validateSlice(p, s, errs)
	case // primitive is always valid when it's not a field, except through introspection
		reflect.Invalid,
		reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64,
		reflect.Complex64, reflect.Complex128,
		reflect.String:
		return true
	default: // anything else cannot be validated, to varying degress of concern
		if strict {
			panic(fmt.Errorf("validate: Unsupported type: %v", s.Type())) // this is a configuration error in strict mode
		}
		fmt.Printf("validate: [%s] ignoring unsupported type: %s\n", p, s.Type().Name())
		return true // we don't support this type, so just ignore it
	}
}

func (v Validator) validateSlice(p string, s reflect.Value, errs *errorBuffer) bool {
	valid, l := true, s.Len()
	for i := 0; i < l; i++ {
		if !v.validate(fmt.Sprintf("%s[%d]", p, i), s.Index(i), errs) {
			valid = false
		}
	}
	return valid
}

func (v Validator) validateStruct(p string, s reflect.Value, errs *errorBuffer) bool {
	typ := s.Type()
	tkey := newTypeKey(typ, v)

	var vt *validatedType
	if typeCache != nil {
		if v, ok := typeCache.Get(tkey); ok {
			vt = v
		}
	}
	if vt == nil {
		vt = newType(typ, v)
		if typeCache != nil {
			typeCache.Add(tkey, vt)
		}
	}

	valid := true
	for _, e := range vt.Fields {
		f := s.Field(e.Index)
		path := keyPath(p, e.Name)

		// recurse to embedded fields unless they are explicitly skipped via
		// the check above: embed:"" or embed:"-"
		if e.Field.Anonymous {
			// we don't allow introspection on embedded fields, this has already been
			// done on the containing struct since it inherits embedded methods and
			// therefore embedded interface conformance
			valid = v.validateFields(path, f, errs) && valid
			continue
		}

		switch e.Expr {
		case "check":
			valid = v.validate(path, f, errs) && valid
		default:
			if !f.CanInterface() {
				panic(fmt.Errorf("validate: Cannot validate unexported field: [%s] %v", e.Name, e.Field))
			}
			val := f.Interface()

			var expr *epl.Program
			if exprCache != nil {
				if v, ok := exprCache.Get(e.Expr); ok {
					expr = v
				}
			}

			if expr == nil {
				var err error
				expr, err = epl.Compile(e.Expr)
				if err != nil {
					panic(fmt.Errorf("validate: Could not compile expression: %v", err)) // this is a configuration error
				}
				if exprCache != nil {
					exprCache.Add(e.Expr, expr)
				}
			}

			check := func(x interface{}) bool {
				return v.validate(path, reflect.ValueOf(x), errs)
			}
			date := func(y, m, d float64) time.Time {
				return time.Date(int(y), time.Month(m), int(d), 0, 0, 0, 0, time.UTC)
			}
			cxt := map[string]interface{}{
				"self":  val,
				"len":   v.len,
				"now":   time.Now,
				"date":  date,
				"check": check,
				"str":   stdlib.Strings{},
			}
			if s.CanInterface() {
				v := s.Interface()
				cxt["super"] = v
				cxt["sup"] = v
			}

			res, err := expr.Exec(cxt)
			if err != nil {
				panic(fmt.Errorf("validate: Could not evaluate expression: %v", err)) // this is a configuration error
			}

			if res != nil {
				switch c := res.(type) {
				case nil: // no error
				case error:
					if c != nil {
						if !e.Noerr {
							errs.Add(c)
						}
						valid = false
					}
				case []error:
					if len(c) > 0 {
						if !e.Noerr {
							errs.Add(c...)
						}
						valid = false
					}
				case bool:
					if !c {
						if !e.Noerr {
							if e.Message != "" {
								errs.Add(&FieldError{Field: path, Message: e.Message})
							} else {
								errs.Add(FieldErrorf(path, "Constraint not satisfied: %s", e.Expr))
							}
						}
						valid = false
					}
				default:
					if !e.Noerr {
						errs.Add(FieldErrorf(path, "Invalid expression result: %T (expected %T) in %v", res, []error{}, res))
					}
					valid = false
				}
			}
		}
	}

	return valid
}

func (v Validator) len(s interface{}) int {
	z := reflect.ValueOf(s)
	switch z.Kind() {
	case reflect.Invalid:
		return 0
	case reflect.Array, reflect.Chan, reflect.Map, reflect.Slice, reflect.String:
		return z.Len()
	default:
		panic(fmt.Errorf("validate: Type does not have a length: %T", s))
	}
}

func unwrapError(val reflect.Value) error {
	if val.IsNil() {
		return nil
	}
	switch v := val.Interface().(type) {
	case []error:
		if len(v) > 0 {
			return Errors(v)
		} else {
			return nil
		}
	case Errors:
		if len(v) > 0 {
			return v
		} else {
			return nil
		}
	case error:
		return v
	}
	return fmt.Errorf("Expected an error; got %v: %v", val.Type(), val.Interface())
}

func fieldErrors(p string, err error) []error {
	var suberrs Errors
	if errors.As(err, &suberrs) {
		return suberrs
	}
	var fielderr *FieldError
	if errors.As(err, &fielderr) {
		return []error{fielderr}
	}
	return []error{
		newFieldError(coalesce(p, "<entity>"), err),
	}
}

func coalesce(t ...string) string {
	for _, e := range t {
		if e != "" {
			return e
		}
	}
	return ""
}
