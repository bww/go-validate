package stdlib

import (
  "unicode"
)

func checkString(s string, f func(r rune)(bool)) bool {
  for _, r := range s {
    if !f(r) {
      return false
    }
  }
  return true
}

type Strings struct {}

func (v Strings) Alpha(s string) bool {
  return checkString(s, unicode.IsLetter)
}

func (v Strings) Numeric(s string) bool {
  return checkString(s, unicode.IsNumber)
}

func (v Strings) AlphaNumeric(s string) bool {
  return checkString(s, func(r rune) bool {
    return unicode.IsLetter(r) || unicode.IsNumber(r)
  })
}
