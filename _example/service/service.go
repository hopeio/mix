/*
 * Copyright 2024 hopeio. All rights reserved.
 * Licensed under the MIT License that can be found in the LICENSE file.
 * @Created by jyb
 */

package service

import (
	"context"
	"strconv"

	pb "github.com/hopeio/mix/_example/proto"
	"github.com/hopeio/mix"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type UserService struct {
	user.UnimplementedUserServiceServer
}

func (u *UserService) Signup(ctx context.Context, req *pb.SignupReq) (*wrapperspb.StringValue, error) {

	if req.Mail == "" && req.Phone == "" {
		return nil, mix.InvalidArgument.Msg("please fill in the email or phone number")
	}

	return &wrapperspb.StringValue{Value: "success"}, nil
}

func (u *UserService) GetUser(ctx context.Context, req *pb.GetUserReq) (*pb.User, error) {
	return &user.User{Id: req.Id, ActivatedAt: timestamppb.Now()}, nil
}
