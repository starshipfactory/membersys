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

package db

import (
	"context"
	"encoding/hex"
	"log"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/gocql/gocql"
	"github.com/golang/protobuf/proto"
	"github.com/starshipfactory/membersys"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

type CassandraDB struct {
	config *gocql.ClusterConfig
	sess   *gocql.Session
}

var applicationPrefix string = "applicant:"
var applicationEnd string = "applicant;"
var queuePrefix string = "queue:"
var dequeuePrefix string = "dequeue:"
var archivePrefix string = "archive:"
var memberPrefix string = "member:"

// castString extracts the data for the string with the given key from the
// map, and returns nil if there is no such data.
func castString(input map[string]interface{}, key string) *string {
	var intf interface{}
	var rv string
	var ok bool

	intf, ok = input[key]
	if !ok {
		return nil
	}

	rv, ok = intf.(string)
	if !ok {
		log.Print("WARNING: type mismatch for field ", key,
			". Expected string, found ", reflect.TypeOf(intf).Name())
		return nil
	}

	return proto.String(rv)
}

// castBool extracts the data for the boolean with the given key from the
// map, and returns nil if there is no such data.
func castBool(input map[string]interface{}, key string) *bool {
	var intf interface{}
	var rv bool
	var ok bool

	intf, ok = input[key]
	if !ok {
		return nil
	}

	rv, ok = intf.(bool)
	if !ok {
		log.Print("WARNING: type mismatch for field ", key,
			". Expected bool, found ", reflect.TypeOf(intf).Name())
		return nil
	}

	return proto.Bool(rv)
}

// castInt64AsUint64 extracts the data for the int64 with the given key from
// the map, converting it to uint64, and returns nil if there is no such data.
func castInt64AsUint64(input map[string]interface{}, key string) *uint64 {
	var intf interface{}
	var rv int64
	var ok bool

	intf, ok = input[key]
	if !ok {
		return nil
	}

	rv, ok = intf.(int64)
	if !ok {
		log.Print("WARNING: type mismatch for field ", key,
			". Expected int64, found ", reflect.TypeOf(intf).Name())
		return nil
	}

	return proto.Uint64(uint64(rv))
}

// castBytes extracts the raw bytes from the given key in the map, and returns
// an empty slice if there is no such data.
func castBytes(input map[string]interface{}, key string) []byte {
	var intf interface{}
	var rv []byte
	var ok bool

	intf, ok = input[key]
	if !ok {
		return nil
	}

	rv, ok = intf.([]byte)
	if !ok {
		log.Print("WARNING: type mismatch for field ", key,
			". Expected []byte, found ", reflect.TypeOf(intf).Name())
		return nil
	}

	return rv
}

// memberFromRow extracts the Member protocol buffer from the data in the
// given row, and returns it.
func memberFromRow(row map[string]interface{}) *membersys.Member {
	return &membersys.Member{
		Id:            castInt64AsUint64(row, "id"),
		Name:          castString(row, "name"),
		Street:        castString(row, "street"),
		City:          castString(row, "city"),
		Zipcode:       castString(row, "zipcode"),
		Country:       castString(row, "country"),
		Email:         castString(row, "email"),
		EmailVerified: castBool(row, "email_verified"),
		Phone:         castString(row, "phone"),
		Fee:           castInt64AsUint64(row, "fee"),
		Username:      castString(row, "username"),
		Pwhash:        castString(row, "pwhash"),
		FeeYearly:     castBool(row, "fee_yearly"),
		HasKey:        castBool(row, "has_key"),
		PaymentsCaughtUpTo: castInt64AsUint64(
			row, "payments_caught_up_to"),
	}
}

// Create a new connection to the membership database on the given "host".
// Will set the keyspace to "dbname".
func NewCassandraDB(hosts []string, dbname string, timeout time.Duration) (
	*CassandraDB, error) {
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
	return &CassandraDB{
		config: config,
		sess:   sess,
	}, nil
}

// Store the given membership request in the database.
func (m *CassandraDB) StoreMembershipRequest(
	ctx context.Context, req *membersys.FormInputData) (
	key string, err error) {
	var pb *membersys.MembershipAgreement = new(membersys.MembershipAgreement)
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

	// This is the perfect illustration of why SQL / CQL is not an appropriate
	// way to exchange data.
	stmt = m.sess.Query("INSERT INTO application "+
		"(name, street, city, zipcode, country, email, email_verified, "+
		"phone, fee, username, pwhash, fee_yearly, sourceip, useragent, "+
		"pb_data) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		req.MemberData.Name, req.MemberData.Street, req.MemberData.City,
		req.MemberData.Zipcode, req.MemberData.Country, req.MemberData.Email,
		false, req.MemberData.Phone, req.MemberData.GetFee(),
		req.MemberData.Username, req.MemberData.Pwhash,
		req.MemberData.GetFeeYearly(), req.Metadata.RequestSourceIp,
		req.Metadata.UserAgent, bdata).WithContext(ctx).
		Consistency(gocql.Quorum).Idempotent(true)
	defer stmt.Release()

	// Now execute the batch mutation.
	err = stmt.Exec()
	return
}

// Retrieve a specific members detailed membership data, but fetch it by the
// user name of the member.
func (m *CassandraDB) GetMemberDetailByUsername(
	ctx context.Context, username string) (
	*membersys.MembershipAgreement, error) {
	var member *membersys.MembershipAgreement = new(membersys.MembershipAgreement)
	var stmt *gocql.Query
	var encodedProto []byte
	var err error

	stmt = m.sess.Query(
		"SELECT pb_data FROM members WHERE username = ?", username).
		WithContext(ctx).Consistency(
		gocql.One)
	defer stmt.Release()

	err = stmt.Scan(&encodedProto)
	if err == gocql.ErrNotFound {
		return nil, grpc.Errorf(codes.NotFound, "No user found for %s: %s",
			username, err.Error())
	}
	if err != nil {
		return nil, grpc.Errorf(codes.Internal, "Error running query: %s",
			err.Error())
	}

	err = proto.Unmarshal(encodedProto, member)
	return member, err
}

// Retrieve a specific members detailed membership data.
func (m *CassandraDB) GetMemberDetail(ctx context.Context, id string) (
	*membersys.MembershipAgreement, error) {
	var member *membersys.MembershipAgreement = new(membersys.MembershipAgreement)
	var stmt *gocql.Query
	var encodedProto []byte
	var err error

	stmt = m.sess.Query(
		"SELECT pb_data FROM members WHERE key = ?",
		append([]byte(memberPrefix), []byte(id)...)).WithContext(ctx).
		Consistency(gocql.One)
	defer stmt.Release()

	err = stmt.Scan(&encodedProto)
	if err == gocql.ErrNotFound {
		return nil, grpc.Errorf(codes.NotFound, "No member found for %s: %s",
			id, err.Error())
	}
	if err != nil {
		return nil, grpc.Errorf(codes.Internal, "Error running query: %s",
			err.Error())
	}

	err = proto.Unmarshal(encodedProto, member)
	return member, err
}

// Update the membership fee for the given member.
func (m *CassandraDB) SetMemberFee(
	ctx context.Context, id string, fee uint64, yearly bool) error {
	var member *membersys.MembershipAgreement = new(membersys.MembershipAgreement)
	var batch *gocql.Batch
	var encodedProto []byte
	var stmt *gocql.Query
	var err error

	// Retrieve the protobuf with all data from Cassandra. Use a quorum read to make sure we aren't
	// missing any recent updates.
	stmt = m.sess.Query(
		"SELECT pb_data FROM members WHERE key = ?",
		append([]byte(memberPrefix), []byte(id)...)).WithContext(ctx).
		Consistency(gocql.Quorum)
	defer stmt.Release()

	err = stmt.Scan(&encodedProto)
	if err == gocql.ErrNotFound {
		return grpc.Errorf(codes.NotFound, "No member found for %s: %s", id,
			err.Error())
	}
	if err != nil {
		return grpc.Errorf(codes.Internal, "Error running query: %s",
			err.Error())
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
		return grpc.Errorf(codes.DataLoss,
			"Error parsing stored membership data: %s", err.Error())
	}

	// Write data columns and pb_data back.
	batch = m.sess.NewBatch(gocql.LoggedBatch).WithContext(ctx)
	batch.SetConsistency(gocql.Quorum)
	batch.Query(
		"UPDATE members SET pb_data = ?, fee = ?, fee_yearly = ? WHERE key = ?",
		encodedProto, fee, yearly, append([]byte(memberPrefix),
			[]byte(id)...))
	batch.Query(
		"UPDATE member_agreements SET pb_data = ? WHERE key = ?",
		encodedProto, append([]byte(memberPrefix), []byte(id)...))
	err = m.sess.ExecuteBatch(batch)
	if err != nil {
		return grpc.Errorf(codes.Internal,
			"Error writing back membership data: %s", err.Error())
	}

	return nil
}

// Update the specified long field for the given member.
func (m *CassandraDB) SetLongValue(
	ctx context.Context, id string, field string, value uint64) error {
	var member *membersys.MembershipAgreement = new(membersys.MembershipAgreement)
	var batch *gocql.Batch
	var encodedProto []byte
	var stmt *gocql.Query
	var err error

	// Retrieve the protobuf with all data from Cassandra. Use a quorum read
	// to make sure we aren't missing any recent updates.
	stmt = m.sess.Query(
		"SELECT pb_data FROM members WHERE key = ?",
		append([]byte(memberPrefix), []byte(id)...)).WithContext(ctx).
		Consistency(gocql.Quorum)
	defer stmt.Release()

	err = stmt.Scan(&encodedProto)
	if err == gocql.ErrNotFound {
		return grpc.Errorf(codes.NotFound, "No member found for %s: %s", id,
			err.Error())
	}
	if err != nil {
		return grpc.Errorf(codes.Internal, "Error running query: %s",
			err.Error())
	}

	// Decode the protobuf which was written to the column.
	err = proto.Unmarshal(encodedProto, member)
	if err != nil {
		return err
	}

	if field == "payments_caught_up_to" {
		member.MemberData.PaymentsCaughtUpTo = proto.Uint64(value)
	} else {
		return grpc.Errorf(codes.NotFound, "Unknown field specified: %s",
			field)
	}

	encodedProto, err = proto.Marshal(member)
	if err != nil {
		return grpc.Errorf(codes.Internal,
			"Error parsing stored membership data: %s", err.Error())
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
		return grpc.Errorf(codes.Internal,
			"Error writing back membership data: %s", err.Error())
	}

	return nil
}

// Update the specified boolean field for the given member.
func (m *CassandraDB) SetBoolValue(
	ctx context.Context, id string, field string, value bool) error {
	var member *membersys.MembershipAgreement = new(membersys.MembershipAgreement)
	var batch *gocql.Batch
	var encodedProto []byte
	var stmt *gocql.Query
	var err error

	// Retrieve the protobuf with all data from Cassandra. Use a quorum
	// read to make sure we aren't missing any recent updates.
	stmt = m.sess.Query(
		"SELECT pb_data FROM members WHERE key = ?",
		append([]byte(memberPrefix), []byte(id)...)).WithContext(ctx).
		Consistency(gocql.Quorum)
	defer stmt.Release()

	err = stmt.Scan(&encodedProto)
	if err == gocql.ErrNotFound {
		return grpc.Errorf(codes.NotFound, "No member found for %s: %s", id,
			err.Error())
	}
	if err != nil {
		return grpc.Errorf(codes.Internal, "Error running query: %s",
			err.Error())
	}

	// Decode the protobuf which was written to the column.
	err = proto.Unmarshal(encodedProto, member)
	if err != nil {
		return err
	}

	if field == "has_key" {
		member.MemberData.HasKey = proto.Bool(value)
	} else {
		return grpc.Errorf(codes.NotFound, "Unknown field specified: %s",
			field)
	}

	encodedProto, err = proto.Marshal(member)
	if err != nil {
		return grpc.Errorf(codes.Internal,
			"Error parsing stored membership data: %s", err.Error())
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
		return grpc.Errorf(codes.Internal,
			"Error writing back membership data: %s", err.Error())
	}

	return nil
}

// Update the specified text column on the membership data.
func (m *CassandraDB) SetTextValue(
	ctx context.Context, id string, field, value string) error {
	var member *membersys.MembershipAgreement = new(membersys.MembershipAgreement)
	var batch *gocql.Batch
	var encodedProto []byte
	var stmt *gocql.Query
	var err error

	// Retrieve the protobuf with all data from Cassandra. Use a quorum
	// read to make sure we aren't missing any recent updates.
	stmt = m.sess.Query(
		"SELECT pb_data FROM members WHERE key = ?",
		append([]byte(memberPrefix), []byte(id)...)).WithContext(ctx).
		Consistency(gocql.Quorum)
	defer stmt.Release()

	err = stmt.Scan(&encodedProto)
	if err == gocql.ErrNotFound {
		return grpc.Errorf(codes.NotFound, "No member found for %s: %s", id,
			err.Error())
	}
	if err != nil {
		return grpc.Errorf(codes.Internal, "Error running query: %s",
			err.Error())
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
			return grpc.Errorf(codes.Internal, "Cannot modify user name")
		}
		member.MemberData.Username = proto.String(value)
	} else {
		return grpc.Errorf(codes.NotFound, "Unknown field specified: %s",
			field)
	}

	encodedProto, err = proto.Marshal(member)
	if err != nil {
		return grpc.Errorf(codes.Internal,
			"Error parsing stored membership data: %s", err.Error())
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
		return grpc.Errorf(codes.Internal,
			"Error writing back membership data: %s", err.Error())
	}

	return nil
}

// Retrieve an individual applicants data.
func (m *CassandraDB) GetMembershipRequest(ctx context.Context, id string) (
	*membersys.MembershipAgreement, error) {
	var uuid gocql.UUID
	var member *membersys.MembershipAgreement = new(membersys.MembershipAgreement)
	var encodedProto []byte
	var stmt *gocql.Query
	var err error

	if uuid, err = gocql.ParseUUID(id); err != nil {
		return nil, grpc.Errorf(codes.Internal,
			"Cannot parse %s as an UUID: %s", id, err.Error())
	}

	// Retrieve the protobuf with all data from Cassandra.
	stmt = m.sess.Query(
		"SELECT pb_data FROM application WHERE key = ?",
		append([]byte(applicationPrefix), uuid.Bytes()...)).WithContext(ctx).
		Consistency(gocql.One)
	defer stmt.Release()

	err = stmt.Scan(&encodedProto)
	if err == gocql.ErrNotFound {
		return nil, grpc.Errorf(codes.NotFound, "No member found for %s: %s",
			id, err.Error())
	}
	if err != nil {
		return nil, grpc.Errorf(codes.Internal, "Error running query: %s",
			err.Error())
	}

	// Decode the protobuf which was written to the column.
	err = proto.Unmarshal(encodedProto, member)
	if err != nil {
		return nil, grpc.Errorf(codes.Internal,
			"Unable to parse membership data: %s", err.Error())
	}

	return member, err
}

// Get a list of all members currently in the database. Returns a set of
// "num" entries beginning after "prev".
// Returns a filled-out member structure.
func (m *CassandraDB) EnumerateMembers(
	ctx context.Context, prev string, num int32) ([]*membersys.Member,
	error) {
	// TODO: invoke StreamingEnumerateMembers().
	var query string
	var stmt *gocql.Query
	var iter *gocql.Iter
	var rv []*membersys.Member
	var err error

	query = "SELECT name, street, city, country, email, phone, username, " +
		"fee, fee_yearly, has_key, payments_caught_up_to FROM members " +
		"WHERE key > ? AND email > ''"

	if num > 0 {
		query += " LIMIT " + strconv.Itoa(int(num))
	}

	query += " ALLOW FILTERING"

	stmt = m.sess.Query(query,
		append([]byte(memberPrefix), []byte(prev)...)).WithContext(ctx).
		Consistency(gocql.One)
	defer stmt.Release()

	iter = stmt.Iter()

	for {
		var row map[string]interface{} = make(map[string]interface{})
		if iter.MapScan(row) {
			rv = append(rv, memberFromRow(row))
		} else {
			break
		}
	}

	err = iter.Close()
	if err != nil {
		return rv, grpc.Errorf(codes.Internal,
			"Error fetching member overview: %s", err.Error())
	}

	return rv, nil
}

// Streams a list of all members currently in the database to the channel
// specified. Returns a set of "num" entries beginning after "prev".
// Returns a filled-out member structure.
func (m *CassandraDB) StreamingEnumerateMembers(
	ctx context.Context, prev string, num int32,
	members chan<- *membersys.Member, errors chan<- error) {
	var query string
	var stmt *gocql.Query
	var iter *gocql.Iter
	var err error

	defer close(members)
	defer close(errors)

	query = "SELECT name, street, city, country, email, phone, username, " +
		"fee, fee_yearly, has_key, payments_caught_up_to FROM members " +
		"WHERE key > ? AND email > ''"

	if num > 0 {
		query += " LIMIT " + strconv.Itoa(int(num))
	}

	query += " ALLOW FILTERING"

	stmt = m.sess.Query(query,
		append([]byte(memberPrefix), []byte(prev)...)).WithContext(ctx).
		Consistency(gocql.One)
	defer stmt.Release()

	iter = stmt.Iter()

	for {
		var row map[string]interface{} = make(map[string]interface{})
		if iter.MapScan(row) {
			members <- memberFromRow(row)
		} else {
			break
		}
	}

	err = iter.Close()
	if err != nil {
		errors <- grpc.Errorf(codes.Internal,
			"Error fetching member overview: %s", err.Error())
	}
}

func (m *CassandraDB) StreamingEnumerateMembershipRequests(
	ctx context.Context, criterion, prev string, num int32,
	agreementStream chan<- *membersys.MembershipAgreementWithKey,
	errorStream chan<- error) {
	var query string
	var stmt *gocql.Query
	var iter *gocql.Iter
	var lowerCriterion string = strings.ToLower(criterion)
	var startKey []byte
	var err error

	defer close(agreementStream)
	defer close(errorStream)

	// Fetch the name, street, city and fee columns of the application column
	// family.
	if len(prev) > 0 {
		var uuid gocql.UUID
		if uuid, err = gocql.ParseUUID(prev); err != nil {
			errorStream <- err
			return
		}
		startKey = append([]byte(applicationPrefix), []byte(uuid.Bytes())...)
	} else {
		startKey = []byte(applicationPrefix)
	}

	query = "SELECT key, pb_data FROM application WHERE key > ?"

	if num > 0 {
		query += " LIMIT " + strconv.Itoa(int(num)) + " ALLOW FILTERING"
		stmt = m.sess.Query(query, startKey)
	} else {
		query += " AND key < ? ALLOW FILTERING"
		stmt = m.sess.Query(query, startKey, applicationEnd)
	}

	stmt = stmt.WithContext(ctx).Consistency(gocql.One)
	defer stmt.Release()

	iter = stmt.Iter()

	for {
		var member *membersys.MembershipAgreementWithKey = new(membersys.MembershipAgreementWithKey)
		var agreement *membersys.MembershipAgreement = new(membersys.MembershipAgreement)
		var row map[string]interface{} = make(map[string]interface{})
		var key []byte
		var encodedProto []byte
		var uuid gocql.UUID

		if !iter.MapScan(row) {
			break
		}

		key = castBytes(row, "key")
		encodedProto = castBytes(row, "pb_data")

		if len(key) < len(applicationPrefix) {
			errorStream <- grpc.Errorf(codes.Internal,
				"Row with short key: %v", key)
			continue
		}

		uuid, err = gocql.UUIDFromBytes(key[len(applicationPrefix):])
		if err != nil {
			// FIXME: We should bump some form of counter here.
			errorStream <- err
			continue
		} else {
			member.Key = uuid.String()
		}

		err = proto.Unmarshal(encodedProto, agreement)
		if err != nil {
			// FIXME: We should bump some form of counter here.
			errorStream <- err
			continue
		}
		proto.Merge(&member.MembershipAgreement, agreement)

		if criterion == "" || strings.HasPrefix(
			strings.ToLower(member.MemberData.GetName()), lowerCriterion) {
			agreementStream <- member
		}
	}

	err = iter.Close()
	if err != nil {
		errorStream <- grpc.Errorf(codes.Internal,
			"Error fetching applicant overview: %s", err.Error())
	}
}

// Get a list of all membership applications currently in the database.
// Returns a set of "num" entries beginning after "prev". If "criterion" is
// given, it will be compared against the name of the member.
func (m *CassandraDB) EnumerateMembershipRequests(
	ctx context.Context, criterion, prev string, num int32) (
	[]*membersys.MembershipAgreementWithKey, error) {
	// TODO: invoke StreamingEnumerateMembershipRequests().
	var query string
	var stmt *gocql.Query
	var iter *gocql.Iter
	var rv []*membersys.MembershipAgreementWithKey
	var lowerCriterion string = strings.ToLower(criterion)
	var startKey []byte
	var err error

	// Fetch the name, street, city and fee columns of the application column
	// family.
	if len(prev) > 0 {
		var uuid gocql.UUID
		if uuid, err = gocql.ParseUUID(prev); err != nil {
			return rv, err
		}
		startKey = append([]byte(applicationPrefix), []byte(uuid.Bytes())...)
	} else {
		startKey = []byte(applicationPrefix)
	}

	query = "SELECT key, pb_data FROM application WHERE key > ?"

	if num > 0 {
		query += " LIMIT " + strconv.Itoa(int(num)) + " ALLOW FILTERING"
		stmt = m.sess.Query(query, startKey)
	} else {
		query += " AND key < ? ALLOW FILTERING"
		stmt = m.sess.Query(query, startKey, applicationEnd)
	}

	stmt = stmt.WithContext(ctx).Consistency(gocql.One)
	defer stmt.Release()

	iter = stmt.Iter()

	for {
		var member *membersys.MembershipAgreementWithKey = new(membersys.MembershipAgreementWithKey)
		var agreement *membersys.MembershipAgreement = new(membersys.MembershipAgreement)
		var row map[string]interface{} = make(map[string]interface{})
		var key []byte
		var encodedProto []byte
		var uuid gocql.UUID

		if !iter.MapScan(row) {
			break
		}

		key = castBytes(row, "key")
		encodedProto = castBytes(row, "pb_data")

		if len(key) < len(applicationPrefix) {
			// FIXME: We should bump some form of counter here.
			continue
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

		if criterion == "" || strings.HasPrefix(
			strings.ToLower(member.MemberData.GetName()), lowerCriterion) {
			rv = append(rv, member)
		}
	}

	err = iter.Close()
	if err != nil {
		return rv, grpc.Errorf(codes.Internal,
			"Error fetching applicant overview: %s", err.Error())
	}

	return rv, nil
}

// Get a list of all future members which are currently in the queue.
func (m *CassandraDB) EnumerateQueuedMembers(
	ctx context.Context, prev string, num int32) ([]*membersys.MemberWithKey,
	error) {
	return m.enumerateQueuedMembersIn(
		ctx, "membership_queue", queuePrefix, prev, num)
}

// Get a list of all former members which are currently in the departing
// members queue.
func (m *CassandraDB) EnumerateDeQueuedMembers(
	ctx context.Context, prev string, num int32) ([]*membersys.MemberWithKey,
	error) {
	return m.enumerateQueuedMembersIn(
		ctx, "membership_dequeue", dequeuePrefix, prev, num)
}

func (m *CassandraDB) enumerateQueuedMembersIn(
	ctx context.Context, cf, prefix, prev string, num int32) (
	[]*membersys.MemberWithKey, error) {
	var query string
	var stmt *gocql.Query
	var iter *gocql.Iter
	var rv []*membersys.MemberWithKey
	var startKey []byte
	var err error

	// Fetch the name, street, city and fee columns of the application column
	// family.
	if len(prev) > 0 {
		var uuid gocql.UUID
		if uuid, err = gocql.ParseUUID(prev); err != nil {
			return rv, err
		}
		startKey = append([]byte(prefix), []byte(uuid.Bytes())...)
	} else {
		startKey = []byte(prefix)
	}

	query = "SELECT key, pb_data FROM " + cf + " WHERE key > ?"

	if num > 0 {
		query += " LIMIT " + strconv.Itoa(int(num))
	}
	query += " ALLOW FILTERING"

	stmt = m.sess.Query(query, startKey).
		WithContext(ctx).Consistency(gocql.One)
	defer stmt.Release()

	iter = stmt.Iter()

	for {
		var member *membersys.MemberWithKey = new(membersys.MemberWithKey)
		var agreement *membersys.MembershipAgreement = new(membersys.MembershipAgreement)
		var row map[string]interface{} = make(map[string]interface{})
		var key []byte
		var encodedProto []byte
		var uuid gocql.UUID

		if !iter.MapScan(row) {
			break
		}

		key = castBytes(row, "key")
		encodedProto = castBytes(row, "pb_data")

		if len(key) < len(prefix) {
			return nil, grpc.Errorf(codes.Internal,
				"Row with short key: %v", key)
			continue
		}

		uuid, err = gocql.UUIDFromBytes(key[len(prefix):])
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
		return rv, grpc.Errorf(codes.Internal,
			"Error fetching applicant overview: %s", err.Error())
	}

	return rv, nil
}

// Get a list of all members which are currently in the trash.
func (m *CassandraDB) EnumerateTrashedMembers(
	ctx context.Context, prev string, num int32) ([]*membersys.MemberWithKey,
	error) {
	return m.enumerateQueuedMembersIn(ctx, "membership_archive",
		archivePrefix, prev, num)
}

func (m *CassandraDB) streamingEnumerateQueuedMembersIn(
	ctx context.Context, cf, prefix, prev string, num int32,
	queued chan<- *membersys.MemberWithKey, errors chan<- error) {
	var query string
	var stmt *gocql.Query
	var iter *gocql.Iter
	var startKey []byte
	var err error

	defer close(queued)
	defer close(errors)

	// Fetch the name, street, city and fee columns of the application column
	// family.
	if len(prev) > 0 {
		var uuid gocql.UUID
		if uuid, err = gocql.ParseUUID(prev); err != nil {
			errors <- err
			return
		}
		startKey = append([]byte(prefix), []byte(uuid.Bytes())...)
	} else {
		startKey = []byte(prefix)
	}

	query = "SELECT key, pb_data FROM " + cf + " WHERE key > ?"

	if num > 0 {
		query += " LIMIT " + strconv.Itoa(int(num))
	}
	query += " ALLOW FILTERING"

	stmt = m.sess.Query(query, startKey).
		WithContext(ctx).Consistency(gocql.One)
	defer stmt.Release()

	iter = stmt.Iter()

	for {
		var member *membersys.MemberWithKey = new(membersys.MemberWithKey)
		var agreement *membersys.MembershipAgreement = new(membersys.MembershipAgreement)
		var row map[string]interface{} = make(map[string]interface{})
		var key []byte
		var encodedProto []byte
		var uuid gocql.UUID

		if !iter.MapScan(row) {
			break
		}

		key = castBytes(row, "key")
		encodedProto = castBytes(row, "pb_data")

		if len(key) < len(prefix) {
			errors <- grpc.Errorf(codes.Internal,
				"Row with short key: %v", key)
			continue
		}

		uuid, err = gocql.UUIDFromBytes(key[len(prefix):])
		if err != nil {
			errors <- err
			continue
		} else {
			member.Key = uuid.String()
		}

		err = proto.Unmarshal(encodedProto, agreement)
		if err != nil {
			errors <- err
			continue
		}
		proto.Merge(&member.Member, agreement.GetMemberData())

		queued <- member
	}

	err = iter.Close()
	if err != nil {
		errors <- grpc.Errorf(codes.Internal,
			"Error fetching applicant overview: %s", err.Error())
	}
}

// Get a list of all future members which are currently in the queue.
func (m *CassandraDB) StreamingEnumerateQueuedMembers(
	ctx context.Context, prev string, num int32,
	queued chan<- *membersys.MemberWithKey, errors chan<- error) {
	m.streamingEnumerateQueuedMembersIn(
		ctx, "membership_queue", queuePrefix, prev, num, queued, errors)
}

// Get a list of all former members which are currently in the departing
// members queue.
func (m *CassandraDB) StreamingEnumerateDeQueuedMembers(
	ctx context.Context, prev string, num int32,
	queued chan<- *membersys.MemberWithKey, errors chan<- error) {
	m.streamingEnumerateQueuedMembersIn(
		ctx, "membership_dequeue", dequeuePrefix, prev, num, queued, errors)
}

// Get a list of all members which are currently in the trash.
func (m *CassandraDB) StreamingEnumerateTrashedMembers(
	ctx context.Context, prev string, num int32,
	queued chan<- *membersys.MemberWithKey, errors chan<- error) {
	m.streamingEnumerateQueuedMembersIn(ctx, "membership_archive",
		archivePrefix, prev, num, queued, errors)
}

// Move a member record to the queue for getting their user account removed
// (e.g. when they leave us).
func (m *CassandraDB) MoveMemberToTrash(
	ctx context.Context, id, initiator, reason string) error {
	var now time.Time = time.Now()
	var now_long uint64 = uint64(now.Unix())
	var uuid gocql.UUID
	var member *membersys.MembershipAgreement = new(membersys.MembershipAgreement)
	var qstmt *gocql.Query
	var batch *gocql.Batch
	var encodedProto []byte

	var err error

	qstmt = m.sess.Query(
		"SELECT pb_data FROM members WHERE key = ?",
		append([]byte(memberPrefix), []byte(id)...)).WithContext(ctx).
		Consistency(gocql.Quorum)
	defer qstmt.Release()

	err = qstmt.Scan(&encodedProto)
	if err == gocql.ErrNotFound {
		return grpc.Errorf(codes.NotFound, "No such member \"%s\" in records",
			id)
	}
	if err != nil {
		return grpc.Errorf(codes.Internal, "Error looking up \"%s\" in member records: %s",
			id, err.Error())
	}

	uuid = gocql.UUIDFromTime(now)

	err = proto.Unmarshal(encodedProto, member)
	if err != nil {
		return grpc.Errorf(codes.DataLoss, "Error parsing member data: %s",
			err.Error())
	}

	member.Metadata.GoodbyeInitiator = &initiator
	member.Metadata.GoodbyeTimestamp = &now_long
	member.Metadata.GoodbyeReason = &reason

	encodedProto, err = proto.Marshal(member)
	if err != nil {
		return grpc.Errorf(codes.Internal,
			"Error encoding member data for deletion: %s", err.Error())
	}

	batch = gocql.NewBatch(gocql.LoggedBatch).WithContext(ctx)
	batch.SetConsistency(gocql.Quorum)
	batch.Query("INSERT INTO membership_dequeue (key, pb_data) VALUES (?, ?)",
		uuid, encodedProto)
	batch.Query("DELETE FROM members WHERE key = ?",
		append([]byte(memberPrefix), []byte(id)...))

	err = m.sess.ExecuteBatch(batch)
	if err != nil {
		return grpc.Errorf(codes.Internal,
			"Error moving membership record to trash in Cassandra database: %s",
			err.Error())
	}

	return nil
}

// Move the record of the given queued member from the queue of new users to
// the list of active users. This method is to be used by the account creation
// software.
func (m *CassandraDB) MoveNewMemberToFullMember(
	ctx context.Context, member *membersys.MemberWithKey) error {
	var encodedProto []byte
	var batch *gocql.Batch
	var err error

	encodedProto, err = proto.Marshal(member)
	if err != nil {
		return grpc.Errorf(codes.Internal,
			"Error encoding member data for creation: %s", err.Error())
	}

	batch = gocql.NewBatch(gocql.LoggedBatch).WithContext(ctx)
	batch.SetConsistency(gocql.Quorum)
	// TODO: fill in other fields.
	batch.Query("INSERT INTO members (key, pb_data) VALUES (?, ?)",
		append([]byte(memberPrefix), []byte(member.GetEmail())...),
		encodedProto)
	batch.Query("INSERT INTO member_agreements (key, pb_data) VALUES (?, ?)",
		append([]byte(memberPrefix), []byte(member.GetEmail())...),
		encodedProto)
	batch.Query("DELETE FROM membership_queue WHERE key = ?",
		append([]byte(queuePrefix), []byte(member.Key)...))

	err = m.sess.ExecuteBatch(batch)
	if err != nil {
		return grpc.Errorf(codes.Internal,
			"Error moving membership application to full membership in Cassandra database: %s",
			err.Error())
	}

	return nil
}

// Move the record of the given dequeued member from the queue of deleted
// users to the list of archived members. Set the retention to 2 years instead
// of just 6 months, since they have been a member. This method is to be used
// by the account deletion software.
func (m *CassandraDB) MoveDeletedMemberToArchive(
	ctx context.Context, member *membersys.MemberWithKey) error {
	var encodedProto []byte
	var batch *gocql.Batch
	var err error

	encodedProto, err = proto.Marshal(member)
	if err != nil {
		return grpc.Errorf(codes.Internal,
			"Error encoding member data for creation: %s", err.Error())
	}

	batch = gocql.NewBatch(gocql.LoggedBatch).WithContext(ctx)
	batch.SetConsistency(gocql.Quorum)
	// TODO: set TTL.
	batch.Query("INSERT INTO membership_archive (key, pb_data) VALUES (?, ?)",
		append([]byte(archivePrefix),
			[]byte(member.Key[len(dequeuePrefix)+1:])...), encodedProto)
	batch.Query("DELETE FROM membership_dequeue WHERE key = ?",
		append([]byte(dequeuePrefix), []byte(member.Key)...))

	err = m.sess.ExecuteBatch(batch)
	if err != nil {
		return grpc.Errorf(codes.Internal,
			"Error moving membership record to archive in Cassandra database: %s",
			err.Error())
	}

	return nil
}

// Move the record of the given applicant to the queue of new users to be
// processed. The approver will be set to "initiator".
func (m *CassandraDB) MoveApplicantToNewMember(
	ctx context.Context, id, initiator string) error {
	return m.moveRecordToTable(ctx, id, initiator, "application",
		applicationPrefix, "membership_queue", queuePrefix, 0)
}

// Move the record of the given applicant to a temporary archive of deleted
// applications. The deleter will be set to "initiator".
func (m *CassandraDB) MoveApplicantToTrash(
	ctx context.Context, id, initiator string) error {
	return m.moveRecordToTable(ctx, id, initiator, "application",
		applicationPrefix, "membership_archive", archivePrefix,
		int32(6*30*24*60*60))
}

// Move a member from the queue to the trash (e.g. if they can't be processed).
func (m *CassandraDB) MoveQueuedRecordToTrash(
	ctx context.Context, id, initiator string) error {
	return m.moveRecordToTable(ctx, id, initiator, "membership_queue",
		queuePrefix, "membership_archive", archivePrefix,
		int32(6*30*24*60*60))
}

// Move the record of the given applicant to a different column family.
func (m *CassandraDB) moveRecordToTable(
	ctx context.Context,
	id, initiator, src_table, src_prefix, dst_table, dst_prefix string,
	ttl int32) error {
	var member *membersys.MembershipAgreement = new(membersys.MembershipAgreement)
	var qstmt *gocql.Query
	var batch *gocql.Batch
	var encodedProto []byte

	var err error

	qstmt = m.sess.Query(
		"SELECT pb_data FROM "+src_table+" WHERE key = ?",
		append([]byte(src_prefix), []byte(id)...)).WithContext(ctx).
		Consistency(gocql.Quorum)
	defer qstmt.Release()

	err = qstmt.Scan(&encodedProto)
	if err == gocql.ErrNotFound {
		return grpc.Errorf(codes.NotFound, "No such %s \"%s\" in records",
			src_table, id)
	}
	if err != nil {
		return grpc.Errorf(codes.Internal, "Error looking up key \"%s\" in %s: %s",
			id, src_table, err.Error())
	}

	err = proto.Unmarshal(encodedProto, member)
	if err != nil {
		return grpc.Errorf(codes.DataLoss, "Error parsing member data: %s",
			err.Error())
	}

	if dst_table == "membership_queue" && len(member.AgreementPdf) == 0 {
		return grpc.Errorf(codes.Internal,
			"No membership agreement scan has been uploaded")
	}

	// Fill in details concerning the approval.
	member.Metadata.ApproverUid = proto.String(initiator)
	member.Metadata.ApprovalTimestamp = proto.Uint64(uint64(time.Now().Unix()))

	encodedProto, err = proto.Marshal(member)
	if err != nil {
		return grpc.Errorf(codes.Internal,
			"Error encoding member data for deletion: %s", err.Error())
	}

	batch = gocql.NewBatch(gocql.LoggedBatch).WithContext(ctx)
	batch.SetConsistency(gocql.Quorum)
	batch.Query("INSERT INTO "+dst_table+" (key, pb_data) VALUES (?, ?)",
		append([]byte(dst_prefix), []byte(id)...), encodedProto)
	batch.Query("DELETE FROM "+src_table+" WHERE key = ?",
		append([]byte(src_prefix), []byte(id)...))

	err = m.sess.ExecuteBatch(batch)
	if err != nil {
		return grpc.Errorf(codes.Internal,
			"Error moving membership record to %s in Cassandra database: %s",
			dst_table, err.Error())
	}

	return nil
}

// Add the membership agreement form scan to the given membership request
// record.
func (m *CassandraDB) StoreMembershipAgreement(
	ctx context.Context, id string, agreement_data []byte) error {
	var agreement *membersys.MembershipAgreement
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

	agreement, err = m.GetMembershipRequest(ctx, id)
	if err != nil {
		return err
	}

	agreement.AgreementPdf = agreement_data
	value, err = proto.Marshal(agreement)
	if err != nil {
		return grpc.Errorf(codes.Internal,
			"Error encoding updated membership agreement: %s", err.Error())
	}

	batch = gocql.NewBatch(gocql.LoggedBatch).WithContext(ctx)
	batch.SetConsistency(gocql.Quorum)
	batch.Query(
		"UPDATE application SET pb_data = ?, application_pdf = ? WHERE key = ?",
		value, agreement_data, buuid)

	err = m.sess.ExecuteBatch(batch)
	if err != nil {
		return grpc.Errorf(codes.Internal,
			"Error moving membership record to trash in Cassandra database: %s",
			err.Error())
	}

	return nil
}
