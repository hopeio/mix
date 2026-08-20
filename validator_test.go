/*
 * Copyright 2024 hopeio. All rights reserved.
 * Licensed under the MIT License that can be found in the LICENSE file.
 * @Created by jyb
 */

package mix

import (
	"errors"
	"reflect"
	"testing"
)

type testValidate struct {
	Name string `validate:"required" json:"name"`
	Age  int    `validate:"min=1" json:"age"`
}

func TestValidateStruct(t *testing.T) {
	tv := testValidate{Name: "", Age: 0}
	err := ValidateStruct(tv)
	if err == nil {
		t.Fatal("ValidateStruct() should return error for invalid struct")
	}
	er := ErrRespFrom(err)
	if er.Msg != "validator.required" || er.Data["field"] != "field.name" {
		t.Fatalf("ValidateStruct() = %+v, want validator.required on field.name", er)
	}
}

func TestValidateStruct_Valid(t *testing.T) {
	tv := testValidate{Name: "test", Age: 10}
	if err := ValidateStruct(tv); err != nil {
		t.Fatalf("ValidateStruct() error for valid struct: %v", err)
	}
}

type customValidator struct {
	err error
}

func (c customValidator) Validate() error {
	return c.err
}

func TestValidateStruct_CustomValidator(t *testing.T) {
	cv := customValidator{err: errors.New("custom error")}
	err := ValidateStruct(cv)
	if err == nil || err.Error() != "custom error" {
		t.Fatalf("ValidateStruct() = %v, want custom error", err)
	}

	cv2 := customValidator{err: nil}
	if err := ValidateStruct(cv2); err != nil {
		t.Fatalf("ValidateStruct() error: %v", err)
	}
}

type customValidatorAll struct {
	err error
}

func (c customValidatorAll) Validate(all bool) error {
	return c.err
}

func TestValidateStruct_CustomValidatorAll(t *testing.T) {
	cv := customValidatorAll{err: errors.New("all error")}
	err := ValidateStruct(cv)
	if err == nil || err.Error() != "all error" {
		t.Fatalf("ValidateStruct() = %v, want all error", err)
	}
}

func TestValidateStruct_MinTag(t *testing.T) {
	tv := testValidate{Name: "ok", Age: 0}
	err := ValidateStruct(tv)
	if err == nil {
		t.Fatal("expected min validation error")
	}
	er := ErrRespFrom(err)
	if er.Msg != "validator.min" || er.Data["field"] != "field.age" || er.Data["min"] != "1" {
		t.Fatalf("ValidateStruct() = %+v, want validator.min on field.age", er)
	}
}

func TestFieldNameForValidate(t *testing.T) {
	type tagged struct {
		Password string `json:"password" comment:"密码"`
		Label    string `json:"label" i18nkey:"auth.password"`
		Bare     string
	}
	if got := fieldNameForValidate(reflect.TypeOf(tagged{}).Field(0)); got != "field.password" {
		t.Fatalf("password = %q, want field.password", got)
	}
	if got := fieldNameForValidate(reflect.TypeOf(tagged{}).Field(1)); got != "auth.password" {
		t.Fatalf("label = %q, want auth.password", got)
	}
	if got := fieldNameForValidate(reflect.TypeOf(tagged{}).Field(2)); got != "field.bare" {
		t.Fatalf("bare = %q, want field.bare", got)
	}
}
