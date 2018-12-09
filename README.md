# Expression-based Validation
Go Validate allows you to valdiate struct fields by defining an expression in the field's tag.

```go
type Example struct {
  A string          `check:"len(self) > 0 "`
  B int             `check:"self != 0"`
  C map[string]int  `check:"self != nil && self["some_key"] > 100"
}

func test(e Example) {
  validator := validate.New()
  errs := validator.Validate(e)
  if len(errs) > 0 {
    // The struct is invalid! Do something about that...
  }
}
```

## EPL
The validation expression is defined using [`EPL`](https://github.com/bww/epl), a special-purpose expression-only language written in Go.

The following variables are available to a validation expression.

| Ident | Value |
|-------|-------|
| `self` | The value of the field that is being validated, itself. |
| `check()` | A function which recurses to validate the fields of the argument (which does not happen by default). |


