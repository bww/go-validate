# Expression-based Validation
_Go Validate_ allows you to valdiate struct fields by defining an expression in the field's tag that evaluates to `true` when the field is valid.

```go
type First struct {
  A int    `check:"self >= 0" invalid:"Must be >= zero"` // A must be greater than zero
  B string `check:"str.Alpha(self)"`                     // B must be alphanumeric (or empty)
}

type Second struct {
  A string         `check:"len(self) > 0 "`                     // A must have a length greather than zero
  B int            `check:"self != 0"`                          // B must not be the value zero
  C map[string]int `check:"self != nil && self.some_key > 100"` // C must have a key 'some_key' whose value is greater than 100
  D *First         `check:"self != nil && check(self)"`         // D must not be nil and must itself be valid
}

func example(e Second) {
  errs := validate.New().Validate(e)
  if len(errs) > 0 {
    // The struct is invalid! Do something about that...
  }
}
```

## Additional Validation
You can perform additional type-level validation by implementing one of two supported interfaces. New code should perfer `IntrospectorV2`.

```go
type IntrospectorV1 interface {
  // Perform custom validation of the receiver and suppress any
  // checks that are defined on individual fields.
  Validate() error
}

type IntrospectorV2 interface {
  // Perform custom validation of the receiver and then perform
  // checks that are defined on individual fields if the second
  // return value is true.
  Validate(Validator) (error, bool)
}

type IntrospectorV3 interface {
  // Perform custom validation of the receiver and then perform
  // checks that are defined on individual fields if the second
  // return value is true.
  Validate(Validator, Context) (error, bool)
}
```

When validating a type that conforms to either of these interfaces, Go Validate will invoke the `Validate` method first, before checking individual fields.

If the type implements `IntrospectorV2` or `IntrospectorV3` and `Validate` returns `true` for the second return value, Go Validate will continue on to validate individual fields. If `false` is returned or if the type implements `IntrospectorV1` instead, the individual fields will not be automatically validated.

## Supported Tags
Struct tags are used to control how Go Validate does its validation. The following tags are supported, and their names can be changed if you like.

| Tag | Description |
|-----|-------------|
| `check` | The expression that will be evaluated. It is common to use different tag names for different "modes". See below. |
| `invalid` | The error message that should be used when `check` fails. You may omit this if you don't mind a generic message and you may specify `-` if you want no error to be reported when validation fails. This can be used when a sub-type is expected to generate all the errors required. |
| `json` | The name of the field, which will be referenced in errors. |

You may change the name of these tags by either using `NewWithConfig` or providing config options to `New` (see below).

## Using Modes
Often, when you are validating input, the definition of "valid" is different based on the mode you're in: create, update, or maybe something else. Go Validate addresses this by allowing you to set the name of the `check` tag so that you can validate differently depending on your mode. For example:

```go
type First struct {
  A string `create:"len(self) > 0"`
  B string `create,update:len(self) > 0"`
}

func create(v First) {
  errs := validate.New(validate.Mode("create")).Validate(v)
  // ...
}

func update(v First) {
  errs := validate.New(validate.Mode("update")).Validate(v)
  // ...
}
```

In this example, when using "create" mode (which just means "use the tag `create` as the check when validating") we assert that both fields `A` and `B` have a non-zero length. When using "update" mode we only assert that `B` is non-zero.

You may have noticed that field `B` uses an unusual tag name. In order to avoid having to repeat the same expression multiple times for use with different modes, Go Validate supports a nonstandard tag notation: you may combine multiple comma-delimited tag names for use with a shared value. For example, the tag:

```
`create,update:"len(self) > 0"`
```

Is equivalent to this more verbose representation:

```
`create:"len(self) > 0" update:"len(self) > 0"`
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


