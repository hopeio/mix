/*
 * Copyright 2024 hopeio. All rights reserved.
 * Licensed under the MIT License that can be found in the LICENSE file.
 * @Created by jyb
 */

package mix

import (
	"errors"
	"reflect"
	"regexp"
	"strings"
	"unicode"

	"github.com/go-playground/validator/v10"
)

var i18nKeyPattern = regexp.MustCompile(`^[a-z][\w-]*(\.[\w-]+)+$`)

var DefaultValidate *validator.Validate

func init() {
	DefaultValidate = validator.New()
	DefaultValidate.SetTagName("validate")
	DefaultValidate.RegisterTagNameFunc(fieldNameForValidate)
}

func fieldNameForValidate(sf reflect.StructField) string {
	if comment := strings.TrimSpace(sf.Tag.Get("comment")); comment != "" && i18nKeyPattern.MatchString(comment) {
		return comment
	}
	return "field." + jsonFieldName(sf)
}

func jsonFieldName(sf reflect.StructField) string {
	if json := sf.Tag.Get("json"); json != "" && json != "-" {
		name := json
		if idx := strings.IndexByte(name, ','); idx >= 0 {
			name = name[:idx]
		}
		if name != "" {
			return name
		}
	}
	return lowerFirstRune(sf.Name)
}

func lowerFirstRune(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToLower(r[0])
	return string(r)
}

// Validator is implemented by proto-generated Validate() methods.
type Validator interface {
	Validate() error
}

// ValidatorAll validates with an all-fields flag.
type ValidatorAll interface {
	Validate(all bool) error
}

// ValidateStruct runs custom Validate() when present, otherwise go-playground tags.
// Tag failures become *ErrResp with i18n keys for client-side rendering.
func ValidateStruct(o any) error {
	switch v := o.(type) {
	case ValidatorAll:
		return v.Validate(true)
	case Validator:
		return v.Validate()
	}
	if err := DefaultValidate.Struct(o); err != nil {
		return errFromPlayground(err)
	}
	return nil
}

func errFromPlayground(err error) error {
	var ve validator.ValidationErrors
	if !errors.As(err, &ve) || len(ve) == 0 {
		return NewErrResp(InvalidArgument, "validator.invalid", nil)
	}
	key, data := playgroundFieldError(ve[0])
	return NewErrResp(InvalidArgument, key, data)
}

func playgroundFieldError(fe validator.FieldError) (string, map[string]string) {
	tag := fe.Tag()
	data := map[string]string{"field": fe.Field()}
	if param := fe.Param(); param != "" {
		data[tag] = param
	}
	return "validator." + tag, data
}
