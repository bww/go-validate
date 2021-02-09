package validate

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"time"

	"github.com/bww/epl/v1"
	"github.com/bww/go-validate/v1/stdlib"
	"github.com/hashicorp/golang-lru"
)

var (
	exprCache *lru.Cache
	typeCache *lru.Cache
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
	if size := sizeFromEnv("GO_VALIDATE_EXPR_CACHE_SIZE", 512); size > 0 {
		var err error
		exprCache, err = lru.New(size) // in practice this cannot fail because we've checked that size > 0
		if err != nil {
			panic(fmt.Errorf("validate: Could not create expression cache: %v", err))
		}
	}
	if size := sizeFromEnv("GO_VALIDATE_TYPE_CACHE_SIZE", 512); size > 0 {
		var err error
		typeCache, err = lru.New(size) // in practice this cannot fail because we've checked that size > 0
		if err != nil {
			panic(fmt.Errorf("validate: Could not create type cache: %v", err))
		}
	}
}

type errorBuffer struct {
	E []error
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

type IntrospectorV1 interface {
	Validate() error
}
type IntrospectorV2 interface {
	Validate(Validator) (error, bool)
}

var (
	introspectorV1 = reflect.TypeOf((*IntrospectorV1)(nil)).Elem()
	introspectorV2 = reflect.TypeOf((*IntrospectorV2)(nil)).Elem()
)

type Validator struct {
	checkTag, errTag, nameTag string
}

func New(opts ...Option) Validator {
	conf := Config{
		CheckTag: "check",
		ErrorTag: "invalid",
		FieldTag: "json",
	}
	for _, opt := range opts {
		conf = opt(conf)
	}
	return NewWithConfig(conf)
}

func NewWithConfig(conf Config) Validator {
	return Validator{
		checkTag: conf.CheckTag,
		errTag:   conf.ErrorTag,
		nameTag:  conf.FieldTag,
	}
}

func (v Validator) Validate(s interface{}) Errors {
	errs := &errorBuffer{}
	v.validate("", reflect.ValueOf(s), errs)
	return errs.E
}

func (v Validator) validate(p string, s reflect.Value, errs *errorBuffer) bool {
	s = reflect.Indirect(s)
	t := s.Type()
	switch {
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
	if !r[0].IsNil() {
		errs.Add(fieldErrors(p, r[0].Interface().(error))...)
		return false
	}
	return true
}

func (v Validator) validateIntrospectorV2(p string, s reflect.Value, errs *errorBuffer) bool {
	var valid bool
	r := s.MethodByName("Validate").Call([]reflect.Value{reflect.ValueOf(v)})
	if !r[0].IsNil() {
		errs.Add(fieldErrors(p, r[0].Interface().(error))...)
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
	case reflect.Invalid:
		return true
	case reflect.Struct:
		return v.validateStruct(p, s, errs)
	case reflect.Slice, reflect.Array:
		return v.validateSlice(p, s, errs)
	default:
		errs.Add(fmt.Errorf("Unsupported type: %v", s.Type()))
	}
	return false
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
			vt = v.(*validatedType)
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
				panic(fmt.Errorf("Cannot validate unexported field: [%s] %v", e.Name, e.Field))
			}
			val := f.Interface()

			var expr *epl.Program
			if exprCache != nil {
				if v, ok := exprCache.Get(e.Expr); ok {
					expr = v.(*epl.Program)
				}
			}

			if expr == nil {
				var err error
				expr, err = epl.Compile(e.Expr)
				if err != nil {
					panic(fmt.Errorf("Could not compile expression: %v", err))
				}
				if exprCache != nil {
					exprCache.Add(e.Expr, expr)
				}
			}

			check := func(s interface{}) bool {
				return v.validate(path, reflect.ValueOf(s), errs)
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
				cxt["sup"] = s.Interface()
			}

			res, err := expr.Exec(cxt)
			if err != nil {
				errs.Add(FieldErrorf(path, "Could not evaluate expression: %v", err))
				valid = false
				continue
			}

			if res != nil {
				switch c := res.(type) {
				case nil: // no error
				case error:
					errs.Add(c)
				case []error:
					errs.Add(c...)
				case bool:
					if !c {
						if e.Message != "" {
							errs.Add(FieldError{path, e.Message})
						} else {
							errs.Add(FieldErrorf(path, "Constraint not satisfied: %s", e.Expr))
						}
						valid = false
					}
				default:
					errs.Add(FieldErrorf(path, "Invalid expression result: %T (expected %T) in %v", res, []error{}, res))
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
		panic(fmt.Errorf("Type does not have a length: %T", s))
	}
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
		FieldErrorf(coalesce(p, "<entity>"), err.Error()),
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
