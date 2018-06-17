package main

import (
	"context"
	"github.com/starshipfactory/membersys"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

// EndUserService provides an RPC interface for end user centric requests to
// the user database.
type EndUserService struct {
	database *membersys.MembershipDB
}

// GetMemberDetailByUsername fetches the membership agreement for the
// user running the query.
func (e *EndUserService) GetMemberDetail(
	ctx context.Context, user *membersys.UserIdentifier) (
	*membersys.MembershipAgreement, error) {
	var agreement *membersys.MembershipAgreement
	var err error

	agreement, err = e.database.GetMemberDetailByUsername(user.GetUsername())
	// Generate a gRPC compatible error.
	if err != nil && grpc.Code(err) == codes.Unknown {
		err = grpc.Errorf(codes.Internal, err.Error())
	}
	return agreement, err
}
