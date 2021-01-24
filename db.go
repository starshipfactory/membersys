package membersys

import (
	"context"
)

// Data used by the HTML template. Contains not just data entered so far,
// but also some error texts in case there was a problem submitting data.
type FormInputData struct {
	MemberData *Member
	Metadata   *MembershipMetadata
	Key        string
	CommonErr  string
	FieldErr   map[string]string
}

type MemberWithKey struct {
	Key string `json:"key"`
	Member
}

type MembershipAgreementWithKey struct {
	Key string `json:"key"`
	MembershipAgreement
}

type MembershipDB interface {
	StoreMembershipRequest(context.Context, *FormInputData) (string, error)
	GetMemberDetailByUsername(context.Context, string) (*MembershipAgreement, error)
	GetMemberDetail(context.Context, string) (*MembershipAgreement, error)
	SetMemberFee(context.Context, string, uint64, bool) error
	SetLongValue(context.Context, string, string, uint64) error
	SetBoolValue(context.Context, string, string, bool) error
	SetTextValue(context.Context, string, string, string) error
	GetMembershipRequest(context.Context, string) (*MembershipAgreement, error)
	StreamingEnumerateMembers(context.Context, string, int32, chan<- *Member, chan<- error)
	EnumerateMembers(context.Context, string, int32) ([]*Member, error)
	StreamingEnumerateMembershipRequests(context.Context, string, string, int32, chan<- *MembershipAgreementWithKey, chan<- error)
	EnumerateMembershipRequests(context.Context, string, string, int32) ([]*MembershipAgreementWithKey, error)
	EnumerateQueuedMembers(context.Context, string, int32) ([]*MemberWithKey, error)
	EnumerateDeQueuedMembers(context.Context, string, int32) ([]*MemberWithKey, error)
	EnumerateTrashedMembers(context.Context, string, int32) ([]*MemberWithKey, error)
	MoveMemberToTrash(context.Context, string, string, string) error
	MoveNewMemberToFullMember(context.Context, *MemberWithKey) error
	MoveDeletedMemberToArchive(context.Context, *MemberWithKey) error
	MoveApplicantToNewMember(context.Context, string, string) error
	MoveApplicantToTrash(context.Context, string, string) error
	MoveQueuedRecordToTrash(context.Context, string, string) error
	StoreMembershipAgreement(context.Context, string, []byte) error
}
