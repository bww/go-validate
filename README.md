# Expression-based Validation
Go Validate allows you to valdiate struct fields by defining an expression in the field's tag that evaluates to `true` when the field is valid.

```go
type First struct {
  A int             `check:"self >= 0"` // A must be greater than zero
  B string          `check:"str.Alpha(self)"` // B must be alphanumeric (or empty)
}

type Second struct {
  A string          `check:"len(self) > 0 "` // A must have a length greather than zero
  B int             `check:"self != 0"` // B must not be the value zero
  C map[string]int  `check:"self != nil && self.some_key > 100"` // C must have a key 'some_key' whose value is greater than 100
  D *First          `check:"self != nil && check(self)"` // D must not be nil and must itself be valid
}

func example(e Second) {
  validator := validate.New()
  errs := validator.Validate(e)
  if len(errs) > 0 {
    // The struct is invalid! Do something about that...
  }
}
```

## Project Goals

* **Expressiveness and flexibility**, arbitrarily complex expressions may be used to define validity of fields, 
* **Strong locality**, the requirements of a field are described near the definition of the field, 
* **Terseness and readabilitiy**, validity is not described by a syntax that is [fundamentally dissimilar from Go](https://godoc.org/gopkg.in/go-playground/validator.v9).


## EPL
Validation expressions are defined using [`EPL`](https://github.com/bww/epl), a special-purpose expression-only language written in Go. Refer to the EPL project for details on usage of the language.

The following variables are available to a validation expression.

| Ident | Value |
|-------|-------|
| `self` | The value of the field that is being validated, itself. |
| `check()` | A function which recurses to validate the fields of the argument (which does not happen by default). |


