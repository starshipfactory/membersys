/*
 * (c) 2014, Caoimhe Chaos <caoimhechaos@protonmail.com>,
 *	     Starship Factory. All rights reserved.
 *
 * Redistribution and use in source  and binary forms, with or without
 * modification, are permitted  provided that the following conditions
 * are met:
 *
 * * Redistributions of  source code  must retain the  above copyright
 *   notice, this list of conditions and the following disclaimer.
 * * Redistributions in binary form must reproduce the above copyright
 *   notice, this  list of conditions and the  following disclaimer in
 *   the  documentation  and/or  other  materials  provided  with  the
 *   distribution.
 * * Neither  the name  of the Starship Factory  nor the  name  of its
 *   contributors may  be used to endorse or  promote products derived
 *   from this software without specific prior written permission.
 *
 * THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
 * "AS IS"  AND ANY EXPRESS  OR IMPLIED WARRANTIES  OF MERCHANTABILITY
 * AND FITNESS  FOR A PARTICULAR  PURPOSE ARE DISCLAIMED. IN  NO EVENT
 * SHALL THE COPYRIGHT OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT,
 * INDIRECT, INCIDENTAL, SPECIAL,  EXEMPLARY, OR CONSEQUENTIAL DAMAGES
 * (INCLUDING, BUT NOT LIMITED  TO, PROCUREMENT OF SUBSTITUTE GOODS OR
 * SERVICES; LOSS OF USE,  DATA, OR PROFITS; OR BUSINESS INTERRUPTION)
 * HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT,
 * STRICT  LIABILITY,  OR  TORT  (INCLUDING NEGLIGENCE  OR  OTHERWISE)
 * ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED
 * OF THE POSSIBILITY OF SUCH DAMAGE.
 */

package membersys

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/gocql/gocql"
	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
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

type MembershipDB struct {
	config *gocql.ClusterConfig
	sess   *gocql.Session
}

type MemberWithKey struct {
	Key string `json:"key"`
	Member
}

type MembershipAgreementWithKey struct {
	Key string `json:"key"`
	MembershipAgreement
}

var applicationPrefix string = "applicant:"
var applicationEnd string = "applicant;"
var queuePrefix string = "queue:"
var queueEnd string = "queue;"
var dequeuePrefix string = "dequeue:"
var dequeueEnd string = "dequeue;"
var archivePrefix string = "archive:"
var archiveEnd string = "archive;"
var memberPrefix string = "member:"
var memberEnd string = "member;"

// List of all relevant columns; used for a few copies here.
var allColumns [][]byte = [][]byte{
	[]byte("name"), []byte("street"), []byte("city"), []byte("zipcode"),
	[]byte("country"), []byte("email"), []byte("email_verified"),
	[]byte("phone"), []byte("fee"), []byte("username"), []byte("pwhash"),
	[]byte("fee_yearly"), []byte("has_key"), []byte("payments_caught_up_to"),
	[]byte("sourceip"), []byte("useragent"), []byte("metadata"),
	[]byte("pb_data"), []byte("application_pdf"), []byte("agreement_pdf"),
	[]byte("approval_ts"),
}

// Create a new connection to the membership database on the given "host".
// Will set the keyspace to "dbname".
func NewMembershipDB(hosts []string, dbname string, timeout time.Duration) (*MembershipDB, error) {
	var config *gocql.ClusterConfig
	var sess *gocql.Session
	var err error

	var cancel context.CancelFunc

	config = gocql.NewCluster(hosts...)
	config.Timeout = timeout
	config.ConnectTimeout = timeout
	config.RetryPolicy = &gocql.ExponentialBackoffRetryPolicy{
		NumRetries: 3,
		Min:        timeout / 10,
		Max:        timeout / 2,
	}
	config.Keyspace = dbname

	_, cancel = context.WithTimeout(context.Background(), timeout)
	defer cancel()

	sess, err = config.CreateSession()
	if err != nil {
		return nil, err
	}
	return &MembershipDB{
		config: config,
		sess:   sess,
	}, nil
}

// Store the given membership request in the database.
func (m *MembershipDB) StoreMembershipRequest(ctx context.Context, req *FormInputData) (key string, err error) {
	var pb *MembershipAgreement = new(MembershipAgreement)
	var stmt *gocql.Query
	var bdata []byte
	var now = time.Now()
	var uuid gocql.UUID

	// First, let's generate an UUID for the new record.
	uuid = gocql.UUIDFromTime(now)
	key = hex.EncodeToString(uuid.Bytes())

	// Add the membership metadata.
	if req.Metadata.RequestTimestamp == nil {
		req.Metadata.RequestTimestamp = new(uint64)
		*req.Metadata.RequestTimestamp = uint64(now.Unix())
	}
	pb.MemberData = req.MemberData
	pb.Metadata = req.Metadata

	bdata, err = proto.Marshal(pb)
	if err != nil {
		return
	}

	// This is the perfect illustration of why SQL / CQL is not an appropriate way to exchange data.
	stmt = m.sess.Query("INSERT INTO application "+
		"(name, street, city, zipcode, country, email, email_verified, phone, fee, username, "+
		"pwhash, fee_yearly, sourceip, useragent, pb_data) VALUES "+
		"(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		req.MemberData.Name, req.MemberData.Street, req.MemberData.City,
		req.MemberData.Zipcode, req.MemberData.Country, req.MemberData.Email, false,
		req.MemberData.Phone, req.MemberData.GetFee(), req.MemberData.Username,
		req.MemberData.Pwhash, req.MemberData.GetFeeYearly(), req.Metadata.RequestSourceIp,
		req.Metadata.UserAgent, bdata).WithContext(ctx).Consistency(gocql.Quorum).Idempotent(true)
	defer stmt.Release()

	// Now execute the batch mutation.
	err = stmt.Exec()
	return
}

// Retrieve a specific members detailed membership data, but fetch it by the
// user name of the member.
func (m *MembershipDB) GetMemberDetailByUsername(
	ctx context.Context, username string) (*MembershipAgreement, error) {
	var member *MembershipAgreement = new(MembershipAgreement)
	var stmt *gocql.Query
	var encodedProto []byte
	var err error

	stmt = m.sess.Query(
		"SELECT pb_data FROM members WHERE username = ?", username).WithContext(ctx).Consistency(
		gocql.One)
	defer stmt.Release()

	err = stmt.Scan(&encodedProto)
	if err == gocql.ErrNotFound {
		return nil, grpc.Errorf(codes.NotFound, "No user found for %s: %s", username, err.Error())
	}
	if err != nil {
		return nil, grpc.Errorf(codes.Internal, "Error running query: %s", err.Error())
	}

	err = proto.Unmarshal(encodedProto, member)
	return member, err
}

// Retrieve a specific members detailed membership data.
func (m *MembershipDB) GetMemberDetail(ctx context.Context, id string) (
	*MembershipAgreement, error) {
	var member *MembershipAgreement = new(MembershipAgreement)
	var stmt *gocql.Query
	var encodedProto []byte
	var err error

	stmt = m.sess.Query(
		"SELECT pb_data FROM members WHERE key = ?", append([]byte(memberPrefix),
			[]byte(id)...)).WithContext(ctx).Consistency(gocql.One)
	defer stmt.Release()

	err = stmt.Scan(&encodedProto)
	if err == gocql.ErrNotFound {
		return nil, grpc.Errorf(codes.NotFound, "No member found for %s: %s", id, err.Error())
	}
	if err != nil {
		return nil, grpc.Errorf(codes.Internal, "Error running query: %s", err.Error())
	}

	err = proto.Unmarshal(encodedProto, member)
	return member, err
}

// Update the membership fee for the given member.
func (m *MembershipDB) SetMemberFee(
	ctx context.Context, id string, fee uint64, yearly bool) error {
	var member *MembershipAgreement = new(MembershipAgreement)
	var batch *gocql.Batch
	var encodedProto []byte
	var stmt *gocql.Query
	var err error

	// Retrieve the protobuf with all data from Cassandra. Use a quorum read to make sure we aren't
	// missing any recent updates.
	stmt = m.sess.Query(
		"SELECT pb_data FROM members WHERE key = ?", append([]byte(memberPrefix),
			[]byte(id)...)).WithContext(ctx).Consistency(gocql.Quorum)
	defer stmt.Release()

	err = stmt.Scan(&encodedProto)
	if err == gocql.ErrNotFound {
		return fmt.Errorf("No member found for %s: %s", id, err.Error())
	}
	if err != nil {
		return fmt.Errorf("Error running query: %s", err.Error())
	}

	// Decode the protobuf which was written to the column.
	err = proto.Unmarshal(encodedProto, member)
	if err != nil {
		return err
	}

	member.MemberData.Fee = &fee
	member.MemberData.FeeYearly = &yearly

	encodedProto, err = proto.Marshal(member)
	if err != nil {
		return fmt.Errorf("Error parsing stored membership data: %s", err.Error())
	}

	// Write data columns and pb_data back.
	batch = m.sess.NewBatch(gocql.LoggedBatch).WithContext(ctx)
	batch.SetConsistency(gocql.Quorum)
	batch.Query(
		"UPDATE members SET pb_data = ?, fee = ?, fee_yearly = ? WHERE key = ?",
		encodedProto, fee, yearly, append([]byte(memberPrefix), []byte(id)...))
	batch.Query(
		"UPDATE member_agreements SET pb_data = ? WHERE key = ?",
		encodedProto, append([]byte(memberPrefix), []byte(id)...))
	err = m.sess.ExecuteBatch(batch)
	if err != nil {
		return fmt.Errorf("Error writing back membership data: %s", err.Error())
	}

	return nil
}

// Update the specified long field for the given member.
func (m *MembershipDB) SetLongValue(
	ctx context.Context, id string, field string, value uint64) error {
	var member *MembershipAgreement = new(MembershipAgreement)
	var batch *gocql.Batch
	var encodedProto []byte
	var stmt *gocql.Query
	var err error

	// Retrieve the protobuf with all data from Cassandra. Use a quorum read to make sure we aren't
	// missing any recent updates.
	stmt = m.sess.Query(
		"SELECT pb_data FROM members WHERE key = ?", append([]byte(memberPrefix),
			[]byte(id)...)).WithContext(ctx).Consistency(gocql.Quorum)
	defer stmt.Release()

	err = stmt.Scan(&encodedProto)
	if err == gocql.ErrNotFound {
		return fmt.Errorf("No member found for %s: %s", id, err.Error())
	}
	if err != nil {
		return fmt.Errorf("Error running query: %s", err.Error())
	}

	// Decode the protobuf which was written to the column.
	err = proto.Unmarshal(encodedProto, member)
	if err != nil {
		return err
	}

	if field == "payments_caught_up_to" {
		member.MemberData.PaymentsCaughtUpTo = proto.Uint64(value)
	} else {
		return fmt.Errorf("Unknown field specified: %s", field)
	}

	encodedProto, err = proto.Marshal(member)
	if err != nil {
		return fmt.Errorf("Error parsing stored membership data: %s", err.Error())
	}

	// Write data columns and pb_data back.
	batch = m.sess.NewBatch(gocql.LoggedBatch).WithContext(ctx)
	batch.SetConsistency(gocql.Quorum)
	batch.Query(
		"UPDATE members SET "+field+" = ?, pb_data = ? WHERE key = ?",
		value, encodedProto, append([]byte(memberPrefix), []byte(id)...))
	batch.Query(
		"UPDATE member_agreements SET pb_data = ? WHERE key = ?",
		encodedProto, append([]byte(memberPrefix), []byte(id)...))
	err = m.sess.ExecuteBatch(batch)
	if err != nil {
		return fmt.Errorf("Error writing back membership data: %s", err.Error())
	}

	return nil
}

// Update the specified boolean field for the given member.
func (m *MembershipDB) SetBoolValue(
	ctx context.Context, id string, field string, value bool) error {
	var member *MembershipAgreement = new(MembershipAgreement)
	var batch *gocql.Batch
	var encodedProto []byte
	var stmt *gocql.Query
	var err error

	// Retrieve the protobuf with all data from Cassandra. Use a quorum read to make sure we aren't
	// missing any recent updates.
	stmt = m.sess.Query(
		"SELECT pb_data FROM members WHERE key = ?", append([]byte(memberPrefix),
			[]byte(id)...)).WithContext(ctx).Consistency(gocql.Quorum)
	defer stmt.Release()

	err = stmt.Scan(&encodedProto)
	if err == gocql.ErrNotFound {
		return fmt.Errorf("No member found for %s: %s", id, err.Error())
	}
	if err != nil {
		return fmt.Errorf("Error running query: %s", err.Error())
	}

	// Decode the protobuf which was written to the column.
	err = proto.Unmarshal(encodedProto, member)
	if err != nil {
		return err
	}

	if field == "has_key" {
		member.MemberData.HasKey = proto.Bool(value)
	} else {
		return fmt.Errorf("Unknown field specified: %s", field)
	}

	encodedProto, err = proto.Marshal(member)
	if err != nil {
		return fmt.Errorf("Error parsing stored membership data: %s", err.Error())
	}

	// Write data columns and pb_data back.
	batch = m.sess.NewBatch(gocql.LoggedBatch).WithContext(ctx)
	batch.SetConsistency(gocql.Quorum)
	batch.Query(
		"UPDATE members SET "+field+" = ?, pb_data = ? WHERE key = ?",
		value, encodedProto, append([]byte(memberPrefix), []byte(id)...))
	batch.Query(
		"UPDATE member_agreements SET pb_data = ? WHERE key = ?",
		encodedProto, append([]byte(memberPrefix), []byte(id)...))
	err = m.sess.ExecuteBatch(batch)
	if err != nil {
		return fmt.Errorf("Error writing back membership data: %s", err.Error())
	}

	return nil
}

// Update the specified text column on the membership data.
func (m *MembershipDB) SetTextValue(
	ctx context.Context, id string, field, value string) error {
	var member *MembershipAgreement = new(MembershipAgreement)
	var batch *gocql.Batch
	var encodedProto []byte
	var stmt *gocql.Query
	var err error

	// Retrieve the protobuf with all data from Cassandra. Use a quorum read to make sure we aren't
	// missing any recent updates.
	stmt = m.sess.Query(
		"SELECT pb_data FROM members WHERE key = ?", append([]byte(memberPrefix),
			[]byte(id)...)).WithContext(ctx).Consistency(gocql.Quorum)
	defer stmt.Release()

	err = stmt.Scan(&encodedProto)
	if err == gocql.ErrNotFound {
		return fmt.Errorf("No member found for %s: %s", id, err.Error())
	}
	if err != nil {
		return fmt.Errorf("Error running query: %s", err.Error())
	}

	// Decode the protobuf which was written to the column.
	err = proto.Unmarshal(encodedProto, member)
	if err != nil {
		return err
	}

	if field == "name" {
		member.MemberData.Name = proto.String(value)
	} else if field == "street" {
		member.MemberData.Street = proto.String(value)
	} else if field == "city" {
		member.MemberData.City = proto.String(value)
	} else if field == "zipcode" {
		member.MemberData.Zipcode = proto.String(value)
	} else if field == "country" {
		member.MemberData.Country = proto.String(value)
	} else if field == "phone" {
		member.MemberData.Phone = proto.String(value)
	} else if field == "username" {
		if member.MemberData.Username != nil && *member.MemberData.Username != "" {
			return errors.New("Cannot modify user name")
		}
		member.MemberData.Username = proto.String(value)
	} else {
		return fmt.Errorf("Unknown field specified: %s", field)
	}

	encodedProto, err = proto.Marshal(member)
	if err != nil {
		return fmt.Errorf("Error parsing stored membership data: %s", err.Error())
	}

	// Write data columns and pb_data back.
	batch = m.sess.NewBatch(gocql.LoggedBatch).WithContext(ctx)
	batch.SetConsistency(gocql.Quorum)
	batch.Query(
		"UPDATE members SET "+field+" = ?, pb_data = ? WHERE key = ?",
		value, encodedProto, append([]byte(memberPrefix), []byte(id)...))
	batch.Query(
		"UPDATE member_agreements SET pb_data = ? WHERE key = ?",
		encodedProto, append([]byte(memberPrefix), []byte(id)...))
	err = m.sess.ExecuteBatch(batch)
	if err != nil {
		return fmt.Errorf("Error writing back membership data: %s", err.Error())
	}

	return nil
}

// Retrieve an individual applicants data.
func (m *MembershipDB) GetMembershipRequest(
	ctx context.Context, id, table, prefix string) (
	*MembershipAgreement, int64, error) {
	var uuid gocql.UUID
	var member *MembershipAgreement = new(MembershipAgreement)
	var encodedProto []byte
	var stmt *gocql.Query
	var timestamp int64
	var err error

	if uuid, err = gocql.ParseUUID(id); err != nil {
		return nil, 0, fmt.Errorf("Cannot parse %s as an UUID: %s", id, err.Error())
	}

	// Retrieve the protobuf with all data from Cassandra.
	stmt = m.sess.Query(
		"SELECT pb_data, WRITETIME(pb_data) FROM "+table+" WHERE key = ?",
		append([]byte(prefix), uuid.Bytes()...)).WithContext(ctx).Consistency(gocql.One)
	defer stmt.Release()

	err = stmt.Scan(&encodedProto, &timestamp)
	if err == gocql.ErrNotFound {
		return nil, 0, fmt.Errorf("No member found for %s: %s", id, err.Error())
	}
	if err != nil {
		return nil, 0, fmt.Errorf("Error running query: %s", err.Error())
	}

	// Decode the protobuf which was written to the column.
	err = proto.Unmarshal(encodedProto, member)
	if err != nil {
		return nil, 0, fmt.Errorf("Unable to parse membership data: %s", err.Error())
	}

	return member, timestamp, err
}

// Get a list of all members currently in the database. Returns a set of
// "num" entries beginning after "prev".
// Returns a filled-out member structure and the timestamp when the
// membership was approved.
func (m *MembershipDB) EnumerateMembers(
	ctx context.Context, prev string, num int32) ([]*Member, error) {
	var stmt *gocql.Query
	var iter *gocql.Iter
	var rv []*Member
	var err error

	stmt = m.sess.Query(
		"SELECT name, street, city, country, email, phone, username, fee, fee_yearly, has_key, "+
			"payments_caught_up_to FROM members WHERE key > ? AND email > '' LIMIT "+
			strconv.Itoa(int(num))+" ALLOW FILTERING",
		append([]byte(memberPrefix), []byte(prev)...)).WithContext(ctx).Consistency(gocql.One)
	defer stmt.Release()

	iter = stmt.Iter()

	for {
		var member *Member = new(Member)
		var done bool

		done = iter.Scan(member.Name, member.Street, member.City, member.Country, member.Email,
			member.Phone, member.Username, member.Fee, member.FeeYearly, member.HasKey,
			member.PaymentsCaughtUpTo)
		if !done {
			rv = append(rv, member)
		}
	}

	err = iter.Close()
	if err != nil {
		return rv, fmt.Errorf("Error fetching member overview: %s", err.Error())
	}

	return rv, nil
}

// Get a list of all membership applications currently in the database.
// Returns a set of "num" entries beginning after "prev". If "criterion" is
// given, it will be compared against the name of the member.
func (m *MembershipDB) EnumerateMembershipRequests(
	ctx context.Context, criterion, prev string, num int32) (
	[]*MembershipAgreementWithKey, error) {
	var stmt *gocql.Query
	var iter *gocql.Iter
	var rv []*MembershipAgreementWithKey
	var startKey []byte
	var err error

	// Fetch the name, street, city and fee columns of the application column family.
	if len(prev) > 0 {
		var uuid gocql.UUID
		if uuid, err = gocql.ParseUUID(prev); err != nil {
			return rv, err
		}
		startKey = append([]byte(applicationPrefix), []byte(uuid.Bytes())...)
	} else {
		startKey = []byte(applicationPrefix)
	}

	stmt = m.sess.Query(
		"SELECT key, pb_data FROM application WHERE key > ? LIMIT "+strconv.Itoa(int(num))+
			" ALLOW FILTERING", startKey).WithContext(ctx).Consistency(gocql.One)
	defer stmt.Release()

	iter = stmt.Iter()

	for {
		var member *MembershipAgreementWithKey = new(MembershipAgreementWithKey)
		var agreement *MembershipAgreement = new(MembershipAgreement)
		var key []byte
		var encodedProto []byte
		var uuid gocql.UUID
		var done bool

		done = iter.Scan(&key, &encodedProto)
		if done {
			break
		}

		uuid, err = gocql.UUIDFromBytes(key[len(applicationPrefix):])
		if err != nil {
			// FIXME: We should bump some form of counter here.
			continue
		} else {
			member.Key = uuid.String()
		}

		err = proto.Unmarshal(encodedProto, agreement)
		if err != nil {
			// FIXME: We should bump some form of counter here.
			continue
		}
		proto.Merge(&member.MembershipAgreement, agreement)

		rv = append(rv, member)
	}

	err = iter.Close()
	if err != nil {
		return rv, fmt.Errorf("Error fetching applicant overview: %s", err.Error())
	}

	return rv, nil
}

// Get a list of all future members which are currently in the queue.
func (m *MembershipDB) EnumerateQueuedMembers(
	ctx context.Context, prev string, num int32) ([]*MemberWithKey, error) {
	return m.enumerateQueuedMembersIn(
		ctx, "membership_queue", queuePrefix, prev, num)
}

// Get a list of all future members which are currently in the departing queue.
func (m *MembershipDB) EnumerateDeQueuedMembers(
	ctx context.Context, prev string, num int32) ([]*MemberWithKey, error) {
	return m.enumerateQueuedMembersIn(
		ctx, "membership_dequeue", dequeuePrefix, prev, num)
}

func (m *MembershipDB) enumerateQueuedMembersIn(
	ctx context.Context, cf, prefix, prev string, num int32) (
	[]*MemberWithKey, error) {
	var stmt *gocql.Query
	var iter *gocql.Iter
	var rv []*MemberWithKey
	var startKey []byte
	var err error

	// Fetch the name, street, city and fee columns of the application column family.
	if len(prev) > 0 {
		var uuid gocql.UUID
		if uuid, err = gocql.ParseUUID(prev); err != nil {
			return rv, err
		}
		startKey = append([]byte(prefix), []byte(uuid.Bytes())...)
	} else {
		startKey = []byte(prefix)
	}

	stmt = m.sess.Query(
		"SELECT key, pb_data FROM "+cf+" WHERE key > ? LIMIT "+strconv.Itoa(int(num))+
			" ALLOW FILTERING", startKey).WithContext(ctx).Consistency(gocql.One)
	defer stmt.Release()

	iter = stmt.Iter()

	for {
		var member *MemberWithKey = new(MemberWithKey)
		var agreement *MembershipAgreement = new(MembershipAgreement)
		var key []byte
		var encodedProto []byte
		var uuid gocql.UUID
		var done bool

		done = iter.Scan(&key, &encodedProto)
		if done {
			break
		}

		uuid, err = gocql.UUIDFromBytes(key[len(applicationPrefix):])
		if err != nil {
			// FIXME: We should bump some form of counter here.
			continue
		} else {
			member.Key = uuid.String()
		}

		err = proto.Unmarshal(encodedProto, agreement)
		if err != nil {
			// FIXME: We should bump some form of counter here.
			continue
		}
		proto.Merge(&member.Member, agreement.GetMemberData())

		rv = append(rv, member)
	}

	err = iter.Close()
	if err != nil {
		return rv, fmt.Errorf("Error fetching applicant overview: %s", err.Error())
	}

	return rv, nil
}

// Get a list of all members which are currently in the trash.
func (m *MembershipDB) EnumerateTrashedMembers(
	ctx context.Context, prev string, num int32) ([]*MemberWithKey, error) {
	return m.enumerateQueuedMembersIn(ctx, "membership_archive", archivePrefix, prev, num)
}

// Move a member record to the queue for getting their user account removed
// (e.g. when they leave us). Set the retention to 2 years instead of just
// 6 months, since they have been a member.
func (m *MembershipDB) MoveMemberToTrash(
	ctx context.Context, id, initiator, reason string) error {
	var now time.Time = time.Now()
	var now_long uint64 = uint64(now.Unix())
	var uuid gocql.UUID
	var member *MembershipAgreement
	var qstmt *gocql.Query
	var batch *gocql.Batch
	var encodedProto []byte

	var err error

	qstmt = m.sess.Query(
		"SELECT pb_data FROM members WHERE key = ?", append([]byte(memberPrefix),
			[]byte(id)...)).WithContext(ctx).Consistency(gocql.Quorum)
	defer qstmt.Release()

	err = qstmt.Scan(&encodedProto)

	uuid = gocql.UUIDFromTime(now)

	member = new(MembershipAgreement)
	err = proto.Unmarshal(encodedProto, member)
	if err != nil {
		return grpc.Errorf(codes.DataLoss, "Error parsing member data: %s", err.Error())
	}

	member.Metadata.GoodbyeInitiator = &initiator
	member.Metadata.GoodbyeTimestamp = &now_long
	member.Metadata.GoodbyeReason = &reason

	encodedProto, err = proto.Marshal(member)
	if err != nil {
		return grpc.Errorf(codes.Internal, "Error encoding member data for deletion: %s",
			err.Error())
	}

	batch = gocql.NewBatch(gocql.LoggedBatch).WithContext(ctx)
	batch.SetConsistency(gocql.Quorum)
	batch.Query("INSERT INTO membership_dequeue (key, pb_data) VALUES (?, ?)",
		uuid, encodedProto)
	batch.Query("DELETE FROM members WHERE key = ?", append([]byte(memberPrefix),
		[]byte(id)...))

	err = m.sess.ExecuteBatch(batch)
	if err != nil {
		return grpc.Errorf(codes.Internal,
			"Error moving membership record to trash in Cassandra database: %s", err.Error())
	}

	return nil
}

// Move the record of the given applicant to the queue of new users to be
// processed. The approver will be set to "initiator".
func (m *MembershipDB) MoveApplicantToNewMember(
	ctx context.Context, id, initiator string) error {
	return m.moveRecordToTable(ctx, id, initiator, "application",
		applicationPrefix, "membership_queue", queuePrefix, 0)
}

// Move the record of the given applicant to a temporary archive of deleted
// applications. The deleter will be set to "initiator".
func (m *MembershipDB) MoveApplicantToTrash(
	ctx context.Context, id, initiator string) error {
	return m.moveRecordToTable(ctx, id, initiator, "application",
		applicationPrefix, "membership_archive", archivePrefix,
		int32(6*30*24*60*60))
}

// Move a member from the queue to the trash (e.g. if they can't be processed).
func (m *MembershipDB) MoveQueuedRecordToTrash(
	ctx context.Context, id, initiator string) error {
	return m.moveRecordToTable(ctx, id, initiator, "membership_queue",
		queuePrefix, "membership_archive", archivePrefix,
		int32(6*30*24*60*60))
}

// Move the record of the given applicant to a different column family.
func (m *MembershipDB) moveRecordToTable(
	ctx context.Context,
	id, initiator, src_table, src_prefix, dst_table, dst_prefix string,
	ttl int32) error {
	var member *MembershipAgreement
	var qstmt *gocql.Query
	var batch *gocql.Batch
	var encodedProto []byte

	var err error

	qstmt = m.sess.Query(
		"SELECT pb_data FROM "+src_table+" WHERE key = ?", append([]byte(src_prefix),
			[]byte(id)...)).WithContext(ctx).Consistency(gocql.Quorum)
	defer qstmt.Release()

	err = qstmt.Scan(&encodedProto)

	member = new(MembershipAgreement)
	err = proto.Unmarshal(encodedProto, member)
	if err != nil {
		return grpc.Errorf(codes.DataLoss, "Error parsing member data: %s", err.Error())
	}

	if dst_table == "membership_queue" && len(member.AgreementPdf) == 0 {
		return errors.New("No membership agreement scan has been uploaded")
	}

	// Fill in details concerning the approval.
	member.Metadata.ApproverUid = proto.String(initiator)
	member.Metadata.ApprovalTimestamp = proto.Uint64(uint64(time.Now().Unix()))

	encodedProto, err = proto.Marshal(member)
	if err != nil {
		return grpc.Errorf(codes.Internal, "Error encoding member data for deletion: %s",
			err.Error())
	}

	batch = gocql.NewBatch(gocql.LoggedBatch).WithContext(ctx)
	batch.SetConsistency(gocql.Quorum)
	batch.Query("INSERT INTO "+dst_table+" (key, pb_data) VALUES (?, ?)",
		append([]byte(dst_prefix), []byte(id)...), encodedProto)
	batch.Query("DELETE FROM "+src_table+" WHERE key = ?", append([]byte(src_prefix),
		[]byte(id)...))

	err = m.sess.ExecuteBatch(batch)
	if err != nil {
		return grpc.Errorf(codes.Internal,
			"Error moving membership record to trash in Cassandra database: %s", err.Error())
	}

	return nil
}

// Add the membership agreement form scan to the given membership request
// record.
func (m *MembershipDB) StoreMembershipAgreement(
	ctx context.Context, id string, agreement_data []byte) error {
	var agreement *MembershipAgreement
	var batch *gocql.Batch
	var uuid gocql.UUID
	var buuid []byte
	var value []byte
	var err error

	uuid, err = gocql.ParseUUID(id)
	if err != nil {
		return err
	}
	buuid = append([]byte(applicationPrefix), uuid.Bytes()...)

	agreement, _, err = m.GetMembershipRequest(ctx, id, "application",
		applicationPrefix)
	if err != nil {
		return err
	}

	agreement.AgreementPdf = agreement_data
	value, err = proto.Marshal(agreement)
	if err != nil {
		return grpc.Errorf(codes.Internal, "Error encoding updated membership agreement: %s",
			err.Error())
	}

	batch = gocql.NewBatch(gocql.LoggedBatch).WithContext(ctx)
	batch.SetConsistency(gocql.Quorum)
	batch.Query("UPDATE application SET pb_data = ?, application_pdf = ? WHERE key = ?",
		value, agreement_data, buuid)

	err = m.sess.ExecuteBatch(batch)
	if err != nil {
		return grpc.Errorf(codes.Internal,
			"Error moving membership record to trash in Cassandra database: %s", err.Error())
	}

	return nil
}
