package user

import (
	mix "github.com/hopeio/mix"
)

func (x *User) Validate() error {
	if x.CreatedAt != nil {
		if err := mix.ValidateStruct(x.CreatedAt); err != nil {
			return err
		}
	}
	if x.ActivatedAt != nil {
		if err := mix.ValidateStruct(x.ActivatedAt); err != nil {
			return err
		}
	}
	if x.DeletedAt != nil {
		if err := mix.ValidateStruct(x.DeletedAt); err != nil {
			return err
		}
	}
	return nil
}
func (x *SignupReq) Validate() error {
	if !(len(x.Password) > 5) {
		return mix.NewErrResp(mix.InvalidArgument, "auth.err.pwdMinLength", map[string]string{"length_gt": "5"})
	}
	return nil
}
func (x *GetUserReq) Validate() error {
	return nil
}
