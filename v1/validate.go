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
	"github.com/hashicorp/golang-lru"
)

var ErrUnsupportedType = fmt.Errorf("Unsupported type (use a struct)")

var cache *lru.Cache

func init() {
	size := 1024 // default cache size
	if v := os.Getenv("GO_VALIDATE_EXPR_CACHE_SIZE"); v != "" {
		var err error
		size, err = strconv.Atoi(v)
		if err != nil {
			panic(fmt.Errorf("validate: Expression cache is not an integer: %v", err))
		}
		if size < 0 {
			panic(fmt.Errorf("validate: Expression cache size makes no sense: %d", size))
		}
	}
	if size > 0 {
		var err error
		cache, err = lru.New(size) // in practice this cannot fail because we've checked that size > 0
		if err != nil {
			panic(fmt.Errorf("validate: Could not create cache: %v", err))
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
	v.validate("", s, errs)
	return errs.E
}

func (v Validator) validate(p string, s interface{}, errs *errorBuffer) bool {
	switch z := s.(type) {
	case IntrospectorV2: // prefer v2
		return v.validateIntrospectorV2(p, s, z, errs)
	case IntrospectorV1:
		return v.validateIntrospectorV1(p, s, z, errs)
	default:
		return v.validateFields(p, reflect.Indirect(reflect.ValueOf(s)), errs)
	}
}

func (v Validator) validateIntrospectorV1(p string, s interface{}, z IntrospectorV1, errs *errorBuffer) bool {
	if err := z.Validate(); err != nil {
		errs.Add(fieldErrors(p, err)...)
		return false
	}
	return true
}

func (v Validator) validateIntrospectorV2(p string, s interface{}, z IntrospectorV2, errs *errorBuffer) bool {
	err, cont := z.Validate(v)
	if err != nil {
		errs.Add(fieldErrors(p, err)...)
	}
	if cont {
		return v.validateFields(p, reflect.Indirect(reflect.ValueOf(s)), errs)
	} else {
		return true
	}
}

func (v Validator) validateFields(p string, z reflect.Value, errs *errorBuffer) bool {
	switch z.Kind() {
	case reflect.Invalid:
		return true
	case reflect.Struct:
		return v.validateStruct(p, z, errs)
	case reflect.Slice, reflect.Array:
		return v.validateSlice(p, z, errs)
	default:
		errs.Add(ErrUnsupportedType)
	}
	return false
}

func (v Validator) validateSlice(p string, z reflect.Value, errs *errorBuffer) bool {
	l := z.Len()
	valid := true
	for i := 0; i < l; i++ {
		if !v.validate(fmt.Sprintf("%s[%d]", p, i), z.Index(i).Interface(), errs) {
			valid = false
		}
	}
	return valid
}

func (v Validator) validateStruct(p string, z reflect.Value, errs *errorBuffer) bool {
	n := z.NumField()
	t := z.Type()

	valid := true
	for i := 0; i < n; i++ {
		field := t.Field(i)

		var path string
		if name := field.Tag.Get(v.nameTag); name != "" {
			path = keyPath(p, fieldName(name))
		} else {
			path = keyPath(p, field.Name)
		}

		msg := strings.TrimSpace(field.Tag.Get(v.errTag))
		src := strings.TrimSpace(getTag(field.Tag, v.checkTag))
		if src == "-" {
			continue
		} else if src == "" && !field.Anonymous {
			continue
		}

		// recurse to embedded fields unless they are explicitly skipped via
		// the check above: embed:"" or embed:"-"
		if field.Anonymous {
			v.validateFields(p, z.Field(i), errs)
			continue
		}

		var val interface{}
		if x := z.Field(i); x.CanInterface() {
			val = x.Interface()
		} else {
			panic(fmt.Errorf("Cannot validate unexported field: [%s] %v", p, field))
		}
		switch src {
		case "check":
			valid = v.validate(path, val, errs) && valid
		default:
			var expr *epl.Program
			if cache != nil {
				if v, ok := cache.Get(src); ok {
					expr = v.(*epl.Program)
				}
			}

			if expr == nil {
				var err error
				expr, err = epl.Compile(src)
				if err != nil {
					panic(fmt.Errorf("Could not compile expression: %v", err))
				}
				if cache != nil {
					cache.Add(src, expr)
				}
			}

			check := func(s interface{}) bool {
				return v.validate(path, s, errs)
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
			if z.CanInterface() {
				cxt["sup"] = z.Interface()
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
						if msg != "" {
							errs.Add(FieldError{path, msg})
						} else {
							errs.Add(FieldErrorf(path, "Constraint not satisfied: %s", src))
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

func fieldName(t string) string {
	if x := strings.Index(t, ","); x > 0 {
		return t[:x]
	} else {
		return t
	}
}
