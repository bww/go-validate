package validate

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/bww/epl/v1"
	"github.com/bww/go-validate/v1/stdlib"
)

var (
	ErrUnsupportedType = fmt.Errorf("Unsupported type (use a struct)")
)

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

type Introspector interface {
	Validate() error
}

type Validator struct {
	checkTag, errTag, nameTag string
}

func New() Validator {
	return NewWithTags("check", "invalid", "json")
}

func NewWithTags(check, err, name string) Validator {
	return Validator{check, err, name}
}

func (v Validator) Validate(s interface{}) Errors {
	errs := &errorBuffer{}
	v.validate("", s, errs)
	return errs.E
}

func (v Validator) validate(p string, s interface{}, errs *errorBuffer) bool {
	if c, ok := s.(Introspector); ok {
		if err := c.Validate(); err != nil {
			errs.Add(FieldErrorf(coalesce(p, "<entity>"), err.Error()))
			return false
		}
		return true
	}
	z := reflect.Indirect(reflect.ValueOf(s))
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
		src := strings.TrimSpace(field.Tag.Get(v.checkTag))
		if src == "" || src == "-" {
			continue
		}

		val := z.Field(i).Interface()
		switch src {
		case "check":
			valid = v.validate(path, val, errs) && valid
		default:
			expr, err := epl.Compile(src)
			if err != nil {
				errs.Add(FieldErrorf(path, "Could not compile expression: %v", err))
				valid = false
				continue
			}

			check := func(s interface{}) bool {
				return v.validate(path, s, errs)
			}
			date := func(y, m, d float64) time.Time {
				return time.Date(int(y), time.Month(m), int(d), 0, 0, 0, 0, time.UTC)
			}
			cxt := map[string]interface{}{
				"self":  val,
				"sup":   z.Interface(),
				"len":   v.len,
				"now":   time.Now,
				"date":  date,
				"check": check,
				"deref": stdlib.Indirect,
				"str":   stdlib.Strings{},
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
