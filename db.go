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

package main

import (
	"database/cassandra"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"time"

	"code.google.com/p/goprotobuf/proto"
)

type MembershipDB struct {
	conn *cassandra.RetryCassandraClient
}

type MemberWithKey struct {
	Key string `json:"key"`
	Member
}

// List of all relevant columns; used for a few copies here.
var allColumns [][]byte = [][]byte{
	[]byte("name"), []byte("street"), []byte("city"), []byte("zipcode"),
	[]byte("country"), []byte("email"), []byte("email_verified"),
	[]byte("phone"), []byte("fee"), []byte("username"), []byte("pwhash"),
	[]byte("fee_yearly"), []byte("sourceip"), []byte("useragent"),
	[]byte("metadata"), []byte("pb_data"), []byte("application_pdf"),
	[]byte("approval_ts"),
}

// Create a new connection to the membership database on the given "host".
// Will set the keyspace to "dbname".
func NewMembershipDB(host, dbname string, timeout time.Duration) (*MembershipDB, error) {
	var conn *cassandra.RetryCassandraClient
	var ire *cassandra.InvalidRequestException
	var err error
	conn, err = cassandra.NewRetryCassandraClientTimeout(host, timeout)
	if err != nil {
		return nil, err
	}
	ire, err = conn.SetKeyspace(dbname)
	if ire != nil {
		return nil, errors.New(ire.Why)
	}
	if err != nil {
		return nil, err
	}
	return &MembershipDB{
		conn: conn,
	}, nil
}

// Create a new mutation with the given name, value and time stamp.
func newCassandraMutationBytes(name string, value []byte, now *time.Time, ttl int32) *cassandra.Mutation {
	var ret = cassandra.NewMutation()
	var col = cassandra.NewColumn()

	col.Timestamp = now.UnixNano()
	col.Name = []byte(name)
	col.Value = value

	if ttl > 0 {
		col.Ttl = ttl
	}

	ret.ColumnOrSupercolumn = cassandra.NewColumnOrSuperColumn()
	ret.ColumnOrSupercolumn.Column = col

	return ret
}

// Create a new mutation with the given name, value and time stamp.
func newCassandraMutationString(name, value string, now *time.Time) *cassandra.Mutation {
	return newCassandraMutationBytes(name, []byte(value), now, 0)
}

// Funciton for lazy people to add a column to a membership request mutation map.
func addMembershipRequestInfoBytes(mmap map[string][]*cassandra.Mutation, name string, value []byte, now *time.Time) {
	mmap["application"] = append(mmap["application"],
		newCassandraMutationBytes(name, value, now, 0))
}

// Funciton for lazy people to add a column to a membership request mutation map.
func addMembershipRequestInfoString(mmap map[string][]*cassandra.Mutation, name string, value *string, now *time.Time) {
	if value != nil && len(*value) > 0 {
		mmap["application"] = append(mmap["application"],
			newCassandraMutationString(name, *value, now))
	}
}

// Store the given membership request in the database.
func (m *MembershipDB) StoreMembershipRequest(req *FormInputData) (key string, err error) {
	var bmods map[string]map[string][]*cassandra.Mutation
	var pb *MembershipAgreement = new(MembershipAgreement)
	var ire *cassandra.InvalidRequestException
	var ue *cassandra.UnavailableException
	var te *cassandra.TimedOutException
	var bdata []byte
	var now = time.Now()
	var uuid cassandra.UUID
	var c_key string

	// First, let's generate an UUID for the new record.
	uuid, err = cassandra.GenTimeUUID(&now)
	if err != nil {
		return "", err
	}

	c_key = string(uuid)
	key = hex.EncodeToString(uuid)

	bmods = make(map[string]map[string][]*cassandra.Mutation)
	bmods[c_key] = make(map[string][]*cassandra.Mutation)
	bmods[c_key]["application"] = make([]*cassandra.Mutation, 0)

	addMembershipRequestInfoString(bmods[c_key], "name", req.MemberData.Name, &now)
	addMembershipRequestInfoString(bmods[c_key], "street", req.MemberData.Street, &now)
	addMembershipRequestInfoString(bmods[c_key], "city", req.MemberData.City, &now)
	addMembershipRequestInfoString(bmods[c_key], "zipcode", req.MemberData.Zipcode, &now)
	addMembershipRequestInfoString(bmods[c_key], "country", req.MemberData.Country, &now)
	addMembershipRequestInfoString(bmods[c_key], "email", req.MemberData.Email, &now)
	addMembershipRequestInfoBytes(bmods[c_key], "email_verified", []byte{0}, &now)
	addMembershipRequestInfoString(bmods[c_key], "phone", req.MemberData.Phone, &now)
	bdata = make([]byte, 8)
	binary.BigEndian.PutUint64(bdata, req.MemberData.GetFee())
	addMembershipRequestInfoBytes(bmods[c_key], "fee", bdata, &now)
	addMembershipRequestInfoString(bmods[c_key], "username", req.MemberData.Username, &now)
	addMembershipRequestInfoString(bmods[c_key], "pwhash", req.MemberData.Pwhash, &now)
	if req.MemberData.GetFeeYearly() {
		addMembershipRequestInfoBytes(bmods[c_key], "fee_yearly", []byte{1}, &now)
	} else {
		addMembershipRequestInfoBytes(bmods[c_key], "fee_yearly", []byte{0}, &now)
	}

	// Set the IP explicitly.
	addMembershipRequestInfoString(bmods[c_key], "sourceip",
		req.Metadata.RequestSourceIp, &now)
	addMembershipRequestInfoString(bmods[c_key], "useragent",
		req.Metadata.UserAgent, &now)

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
	addMembershipRequestInfoBytes(bmods[c_key], "pb_data", bdata, &now)

	// Now execute the batch mutation.
	ire, ue, te, err = m.conn.BatchMutate(bmods, cassandra.ConsistencyLevel_QUORUM)
	if ire != nil {
		err = errors.New(ire.Why)
		return
	}
	if ue != nil {
		err = errors.New("Cassandra unavailable: " + ue.String())
		return
	}
	if te != nil {
		err = errors.New("Timed out: " + te.String())
		return
	}
	if err != nil {
		return
	}
	return
}

// Retrieve a specific members detailed membership data.
func (m *MembershipDB) GetMemberDetail(id string) (*MembershipAgreement, error) {
	var member *MembershipAgreement = new(MembershipAgreement)
	var cp *cassandra.ColumnPath = cassandra.NewColumnPath()
	var r *cassandra.ColumnOrSuperColumn
	var ire *cassandra.InvalidRequestException
	var nfe *cassandra.NotFoundException
	var ue *cassandra.UnavailableException
	var te *cassandra.TimedOutException
	var err error

	cp.ColumnFamily = "members"
	cp.Column = []byte("pb_data")

	// Retrieve the protobuf with all data from Cassandra.
	r, ire, nfe, ue, te, err = m.conn.Get(
		[]byte(id), cp, cassandra.ConsistencyLevel_ONE)
	if ire != nil {
		return nil, errors.New(ire.Why)
	}
	if nfe != nil {
		return nil, errors.New("Not found")
	}
	if ue != nil {
		return nil, errors.New("Unavailable")
	}
	if te != nil {
		return nil, errors.New("Timed out")
	}
	if err != nil {
		return nil, err
	}

	// Decode the protobuf which was written to the column.
	err = proto.Unmarshal(r.Column.Value, member)
	return member, err
}

// Retrieve an individual applicants data.
func (m *MembershipDB) GetMembershipRequest(id, table string) (*MembershipAgreement, int64, error) {
	var uuid cassandra.UUID
	var member *MembershipAgreement = new(MembershipAgreement)
	var cp *cassandra.ColumnPath = cassandra.NewColumnPath()
	var r *cassandra.ColumnOrSuperColumn
	var ire *cassandra.InvalidRequestException
	var nfe *cassandra.NotFoundException
	var ue *cassandra.UnavailableException
	var te *cassandra.TimedOutException
	var err error

	if uuid, err = cassandra.ParseUUID(id); err != nil {
		return nil, 0, err
	}

	cp.ColumnFamily = table
	cp.Column = []byte("pb_data")

	// Retrieve the protobuf with all data from Cassandra.
	r, ire, nfe, ue, te, err = m.conn.Get([]byte(uuid), cp, cassandra.ConsistencyLevel_ONE)
	if ire != nil {
		return nil, 0, errors.New(ire.Why)
	}
	if nfe != nil {
		return nil, 0, errors.New("Not found")
	}
	if ue != nil {
		return nil, 0, errors.New("Unavailable")
	}
	if te != nil {
		return nil, 0, errors.New("Timed out")
	}
	if err != nil {
		return nil, 0, err
	}

	// Decode the protobuf which was written to the column.
	err = proto.Unmarshal(r.Column.Value, member)
	return member, r.Column.Timestamp, err
}

// Get a list of all members currently in the database. Returns a set of
// "num" entries beginning after "prev".
// Returns a filled-out member structure and the timestamp when the
// membership was approved.
func (m *MembershipDB) EnumerateMembers(prev string, num int32) (
	[]*Member, error) {
	var cp *cassandra.ColumnParent = cassandra.NewColumnParent()
	var pred *cassandra.SlicePredicate = cassandra.NewSlicePredicate()
	var r *cassandra.KeyRange = cassandra.NewKeyRange()
	var kss []*cassandra.KeySlice
	var ks *cassandra.KeySlice
	var rv []*Member
	var ire *cassandra.InvalidRequestException
	var ue *cassandra.UnavailableException
	var te *cassandra.TimedOutException
	var err error

	// Fetch all relevant non-protobuf columns of the members column family.
	cp.ColumnFamily = "members"
	pred.ColumnNames = [][]byte{
		[]byte("name"), []byte("street"), []byte("city"), []byte("country"),
		[]byte("email"), []byte("phone"), []byte("username"), []byte("fee"),
		[]byte("fee_yearly"),
	}
	if len(prev) > 0 {
		r.StartKey = []byte(prev)
	} else {
		r.StartKey = make([]byte, 0)
	}
	r.EndKey = make([]byte, 0)
	r.Count = num

	kss, ire, ue, te, err = m.conn.GetRangeSlices(cp, pred, r, cassandra.ConsistencyLevel_ONE)
	if ire != nil {
		err = errors.New(ire.Why)
		return rv, err
	}
	if ue != nil {
		err = errors.New("Cassandra unavailable: " + ue.String())
		return rv, err
	}
	if te != nil {
		err = errors.New("Timed out: " + te.String())
		return rv, err
	}
	if err != nil {
		return rv, err
	}

	for _, ks = range kss {
		var member *Member = new(Member)
		var scol *cassandra.ColumnOrSuperColumn

		if len(ks.Columns) == 0 {
			continue
		}

		member.Email = proto.String(string(ks.Key))

		for _, scol = range ks.Columns {
			var col *cassandra.Column = scol.Column
			var colname string = string(col.Name)

			if colname == "name" {
				member.Name = proto.String(string(col.Value))
			} else if colname == "street" {
				member.Street = proto.String(string(col.Value))
			} else if colname == "city" {
				member.City = proto.String(string(col.Value))
			} else if colname == "country" {
				member.Country = proto.String(string(col.Value))
			} else if colname == "email" {
				member.Email = proto.String(string(col.Value))
			} else if colname == "phone" {
				member.Phone = proto.String(string(col.Value))
			} else if colname == "username" {
				member.Username = proto.String(string(col.Value))
			} else if colname == "fee" {
				member.Fee = proto.Uint64(binary.BigEndian.Uint64(col.Value))
			} else if colname == "fee_yearly" {
				member.FeeYearly = proto.Bool(col.Value[0] == 1)
			}
		}

		rv = append(rv, member)
	}

	return rv, nil
}

// Get a list of all membership applications currently in the database.
// Returns a set of "num" entries beginning after "prev". If "criterion" is
// given, it will be compared against the name of the member.
func (m *MembershipDB) EnumerateMembershipRequests(criterion, prev string, num int32) (
	[]*MemberWithKey, error) {
	var cp *cassandra.ColumnParent = cassandra.NewColumnParent()
	var pred *cassandra.SlicePredicate = cassandra.NewSlicePredicate()
	var r *cassandra.KeyRange = cassandra.NewKeyRange()
	var kss []*cassandra.KeySlice
	var ks *cassandra.KeySlice
	var rv []*MemberWithKey
	var ire *cassandra.InvalidRequestException
	var ue *cassandra.UnavailableException
	var te *cassandra.TimedOutException
	var err error

	// Fetch the name, street, city and fee columns of the application column family.
	cp.ColumnFamily = "application"
	pred.ColumnNames = [][]byte{
		[]byte("name"), []byte("street"), []byte("city"), []byte("fee"),
		[]byte("fee_yearly"),
	}
	if len(prev) > 0 {
		var uuid cassandra.UUID
		if uuid, err = cassandra.ParseUUID(prev); err != nil {
			return rv, err
		}
		r.StartKey = []byte(uuid)
	} else {
		r.StartKey = make([]byte, 0)
	}
	r.EndKey = make([]byte, 0)
	r.Count = num

	// Only count those rows which contain data in this column family.
	r.RowFilter = []*cassandra.IndexExpression{
		&cassandra.IndexExpression{
			ColumnName: []byte("name"),
			Op:         cassandra.IndexOperator_GT,
			Value:      make([]byte, 0),
		},
	}

	kss, ire, ue, te, err = m.conn.GetRangeSlices(cp, pred, r, cassandra.ConsistencyLevel_ONE)
	if ire != nil {
		err = errors.New(ire.Why)
		return rv, err
	}
	if ue != nil {
		err = errors.New("Cassandra unavailable: " + ue.String())
		return rv, err
	}
	if te != nil {
		err = errors.New("Timed out: " + te.String())
		return rv, err
	}
	if err != nil {
		return rv, err
	}

	for _, ks = range kss {
		var member *MemberWithKey = new(MemberWithKey)
		var scol *cassandra.ColumnOrSuperColumn
		var uuid cassandra.UUID = cassandra.UUIDFromBytes(ks.Key)

		member.Key = uuid.String()

		if len(ks.Columns) == 0 {
			continue
		}

		for _, scol = range ks.Columns {
			var col *cassandra.Column = scol.Column

			if string(col.Name) == "name" {
				member.Name = proto.String(string(col.Value))
			} else if string(col.Name) == "street" {
				member.Street = proto.String(string(col.Value))
			} else if string(col.Name) == "city" {
				member.City = proto.String(string(col.Value))
			} else if string(col.Name) == "fee" {
				member.Fee = proto.Uint64(binary.BigEndian.Uint64(col.Value))
			} else if string(col.Name) == "fee_yearly" {
				member.FeeYearly = proto.Bool(col.Value[0] == 1)
			}
		}

		rv = append(rv, member)
	}

	return rv, nil
}

// Get a list of all future members which are currently in the queue.
func (m *MembershipDB) EnumerateQueuedMembers(prev string, num int32) ([]*MemberWithKey, error) {
	return m.enumerateQueuedMembersIn("membership_queue", prev, num)
}

// Get a list of all future members which are currently in the departing queue.
func (m *MembershipDB) EnumerateDeQueuedMembers(prev string, num int32) ([]*MemberWithKey, error) {
	return m.enumerateQueuedMembersIn("membership_dequeue", prev, num)
}

func (m *MembershipDB) enumerateQueuedMembersIn(cf, prev string, num int32) ([]*MemberWithKey, error) {
	var cp *cassandra.ColumnParent = cassandra.NewColumnParent()
	var pred *cassandra.SlicePredicate = cassandra.NewSlicePredicate()
	var r *cassandra.KeyRange = cassandra.NewKeyRange()
	var kss []*cassandra.KeySlice
	var ks *cassandra.KeySlice
	var rv []*MemberWithKey
	var ire *cassandra.InvalidRequestException
	var ue *cassandra.UnavailableException
	var te *cassandra.TimedOutException
	var err error

	// Fetch the protobuf column of the application column family.
	cp.ColumnFamily = cf
	pred.ColumnNames = [][]byte{
		[]byte("pb_data"),
	}
	if len(prev) > 0 {
		var uuid cassandra.UUID
		if uuid, err = cassandra.ParseUUID(prev); err != nil {
			return rv, err
		}
		r.StartKey = []byte(uuid)
	} else {
		r.StartKey = make([]byte, 0)
	}
	r.EndKey = make([]byte, 0)
	r.Count = num

	// Only count those rows which contain data in this column family.
	r.RowFilter = []*cassandra.IndexExpression{
		&cassandra.IndexExpression{
			ColumnName: []byte("pb_data"),
			Op:         cassandra.IndexOperator_GT,
			Value:      make([]byte, 0),
		},
	}

	kss, ire, ue, te, err = m.conn.GetRangeSlices(cp, pred, r, cassandra.ConsistencyLevel_ONE)
	if ire != nil {
		err = errors.New(ire.Why)
		return rv, err
	}
	if ue != nil {
		err = errors.New("Cassandra unavailable: " + ue.String())
		return rv, err
	}
	if te != nil {
		err = errors.New("Timed out: " + te.String())
		return rv, err
	}
	if err != nil {
		return rv, err
	}

	for _, ks = range kss {
		var member *MemberWithKey
		var scol *cassandra.ColumnOrSuperColumn
		var uuid cassandra.UUID = cassandra.UUIDFromBytes(ks.Key)

		if len(ks.Columns) == 0 {
			continue
		}

		for _, scol = range ks.Columns {
			var col *cassandra.Column = scol.Column

			if string(col.Name) == "pb_data" {
				var agreement = new(MembershipAgreement)
				member = new(MemberWithKey)
				err = proto.Unmarshal(col.Value, agreement)
				proto.Merge(&member.Member, agreement.GetMemberData())
				member.Key = uuid.String()
			}
		}

		if member != nil {
			rv = append(rv, member)
		}
	}

	return rv, nil
}

// Get a list of all members which are currently in the trash.
func (m *MembershipDB) EnumerateTrashedMembers(prev string, num int32) ([]*MemberWithKey, error) {
	var cp *cassandra.ColumnParent = cassandra.NewColumnParent()
	var pred *cassandra.SlicePredicate = cassandra.NewSlicePredicate()
	var r *cassandra.KeyRange = cassandra.NewKeyRange()
	var kss []*cassandra.KeySlice
	var ks *cassandra.KeySlice
	var rv []*MemberWithKey
	var ire *cassandra.InvalidRequestException
	var ue *cassandra.UnavailableException
	var te *cassandra.TimedOutException
	var err error

	// Fetch the protobuf column of the application column family.
	cp.ColumnFamily = "membership_archive"
	pred.ColumnNames = [][]byte{
		[]byte("pb_data"),
	}
	if len(prev) > 0 {
		var uuid cassandra.UUID
		if uuid, err = cassandra.ParseUUID(prev); err != nil {
			return rv, err
		}
		r.StartKey = []byte(uuid)
	} else {
		r.StartKey = make([]byte, 0)
	}
	r.EndKey = make([]byte, 0)
	r.Count = num

	kss, ire, ue, te, err = m.conn.GetRangeSlices(cp, pred, r, cassandra.ConsistencyLevel_ONE)
	if ire != nil {
		err = errors.New(ire.Why)
		return rv, err
	}
	if ue != nil {
		err = errors.New("Cassandra unavailable: " + ue.String())
		return rv, err
	}
	if te != nil {
		err = errors.New("Timed out: " + te.String())
		return rv, err
	}
	if err != nil {
		return rv, err
	}

	for _, ks = range kss {
		var member *MemberWithKey
		var scol *cassandra.ColumnOrSuperColumn
		var uuid cassandra.UUID = cassandra.UUIDFromBytes(ks.Key)

		if len(ks.Columns) == 0 {
			continue
		}

		for _, scol = range ks.Columns {
			var col *cassandra.Column = scol.Column

			if string(col.Name) == "pb_data" {
				var agreement = new(MembershipAgreement)
				member = new(MemberWithKey)
				err = proto.Unmarshal(col.Value, agreement)
				proto.Merge(&member.Member, agreement.GetMemberData())
				member.Key = uuid.String()
			}
		}

		if member != nil {
			rv = append(rv, member)
		}
	}

	return rv, nil
}

// Move a member record to the queue for getting their user account removed
// (e.g. when they leave us). Set the retention to 2 years instead of just
// 6 months, since they have been a member.
func (m *MembershipDB) MoveMemberToTrash(id, initiator, reason string) error {
	var now time.Time = time.Now()
	var now_long uint64 = uint64(now.Unix())
	var uuid cassandra.UUID
	var mmap map[string]map[string][]*cassandra.Mutation
	var member *MembershipAgreement

	var cp *cassandra.ColumnPath = cassandra.NewColumnPath()
	var cos *cassandra.ColumnOrSuperColumn
	var del *cassandra.Deletion = cassandra.NewDeletion()
	var mu *cassandra.Mutation

	var ire *cassandra.InvalidRequestException
	var nfe *cassandra.NotFoundException
	var ue *cassandra.UnavailableException
	var te *cassandra.TimedOutException
	var err error

	cp.ColumnFamily = "members"
	cp.Column = []byte("pb_data")

	uuid, err = cassandra.GenTimeUUID(&now)
	if err != nil {
		return err
	}

	cos, ire, nfe, ue, te, err = m.conn.Get(
		[]byte(id), cp, cassandra.ConsistencyLevel_QUORUM)
	if ire != nil {
		return errors.New(ire.Why)
	}
	if nfe != nil {
		return errors.New("Not found")
	}
	if ue != nil {
		return errors.New("Unavailable")
	}
	if te != nil {
		return errors.New("Timed out")
	}
	if err != nil {
		return err
	}

	member = new(MembershipAgreement)
	err = proto.Unmarshal(cos.Column.Value, member)
	if err != nil {
		return err
	}

	del.Predicate = cassandra.NewSlicePredicate()
	del.Predicate.ColumnNames = allColumns
	del.Timestamp = cos.Column.Timestamp

	mu = cassandra.NewMutation()
	mu.Deletion = del

	mmap = make(map[string]map[string][]*cassandra.Mutation)
	mmap[id] = make(map[string][]*cassandra.Mutation)
	mmap[id]["members"] = []*cassandra.Mutation{mu}

	member.Metadata.GoodbyeInitiator = &initiator
	member.Metadata.GoodbyeTimestamp = &now_long
	member.Metadata.GoodbyeReason = &reason

	cos.Column = cassandra.NewColumn()
	cos.Column.Name = []byte("pb_data")
	cos.Column.Timestamp = now.UnixNano()
	cos.Column.Value, err = proto.Marshal(member)
	if err != nil {
		return err
	}

	mu = cassandra.NewMutation()
	mu.ColumnOrSupercolumn = cos

	mmap[string([]byte(uuid))] = make(map[string][]*cassandra.Mutation)
	mmap[string([]byte(uuid))]["membership_dequeue"] = []*cassandra.Mutation{mu}

	ire, ue, te, err = m.conn.AtomicBatchMutate(mmap, cassandra.ConsistencyLevel_QUORUM)
	if ire != nil {
		return errors.New(ire.Why)
	}
	if ue != nil {
		return errors.New("Unavailable")
	}
	if te != nil {
		return errors.New("Timed out")
	}
	return err
}

// Move the record of the given applicant to the queue of new users to be
// processed. The approver will be set to "initiator".
func (m *MembershipDB) MoveApplicantToNewMember(id, initiator string) error {
	return m.moveRecordToTable(id, initiator, "application",
		"membership_queue", 0)
}

// Move the record of the given applicant to a temporary archive of deleted
// applications. The deleter will be set to "initiator".
func (m *MembershipDB) MoveApplicantToTrash(id, initiator string) error {
	return m.moveRecordToTable(id, initiator, "application",
		"membership_archive", int32(6*30*24*60*60))
}

// Move a member from the queue to the trash (e.g. if they can't be processed).
func (m *MembershipDB) MoveQueuedRecordToTrash(id, initiator string) error {
	return m.moveRecordToTable(id, initiator, "membership_queue",
		"membership_archive", int32(6*30*24*60*60))
}

// Move the record of the given applicant to a different column family.
func (m *MembershipDB) moveRecordToTable(id, initiator, src_table, dst_table string, ttl int32) error {
	var uuid cassandra.UUID
	var bmods map[string]map[string][]*cassandra.Mutation
	var now time.Time = time.Now()
	var member *MembershipAgreement
	var mutation *cassandra.Mutation = cassandra.NewMutation()
	var value []byte
	var timestamp int64
	var ire *cassandra.InvalidRequestException
	var ue *cassandra.UnavailableException
	var te *cassandra.TimedOutException
	var err error

	uuid, err = cassandra.ParseUUID(id)
	if err != nil {
		return err
	}

	// First, retrieve the desired membership data.
	if member, timestamp, err = m.GetMembershipRequest(id, src_table); err != nil {
		return err
	}

	if dst_table == "membership_queue" && len(member.AgreementPdf) == 0 {
		return errors.New("No membership agreement scan has been uploaded")
	}

	// Fill in details concerning the approval.
	member.Metadata.ApproverUid = proto.String(initiator)
	member.Metadata.ApprovalTimestamp = proto.Uint64(uint64(now.Unix()))

	bmods = make(map[string]map[string][]*cassandra.Mutation)
	bmods[string(uuid)] = make(map[string][]*cassandra.Mutation)
	bmods[string(uuid)][dst_table] = make([]*cassandra.Mutation, 0)
	bmods[string(uuid)][src_table] = make([]*cassandra.Mutation, 0)

	value, err = proto.Marshal(member)
	if err != nil {
		return err
	}

	// Add the application protobuf to the membership data.
	bmods[string(uuid)][dst_table] = append(bmods[string(uuid)][dst_table],
		newCassandraMutationBytes("pb_data", value, &now, ttl))

	// Delete the application data.
	mutation.Deletion = cassandra.NewDeletion()
	mutation.Deletion.Predicate = cassandra.NewSlicePredicate()
	mutation.Deletion.Predicate.ColumnNames = allColumns
	mutation.Deletion.Timestamp = timestamp
	bmods[string(uuid)][src_table] = append(
		bmods[string(uuid)][src_table], mutation)

	ire, ue, te, err = m.conn.AtomicBatchMutate(bmods, cassandra.ConsistencyLevel_QUORUM)
	if ire != nil {
		return errors.New(ire.Why)
	}
	if ue != nil {
		return errors.New("Unavailable")
	}
	if te != nil {
		return errors.New("Timed out")
	}

	return err
}

// Add the membership agreement form scan to the given membership request
// record.
func (m *MembershipDB) StoreMembershipAgreement(id string, agreement_data []byte) error {
	var agreement *MembershipAgreement
	var bmods map[string]map[string][]*cassandra.Mutation
	var ire *cassandra.InvalidRequestException
	var ue *cassandra.UnavailableException
	var te *cassandra.TimedOutException
	var now = time.Now()
	var uuid cassandra.UUID
	var buuid []byte
	var value []byte
	var err error

	uuid, err = cassandra.ParseUUID(id)
	if err != nil {
		return err
	}
	buuid = []byte(uuid)

	agreement, _, err = m.GetMembershipRequest(id, "application")
	if err != nil {
		return err
	}

	agreement.AgreementPdf = agreement_data

	bmods = make(map[string]map[string][]*cassandra.Mutation)
	bmods[string(buuid)] = make(map[string][]*cassandra.Mutation)
	bmods[string(buuid)]["application"] = make([]*cassandra.Mutation, 0)

	value, err = proto.Marshal(agreement)
	if err != nil {
		return err
	}

	addMembershipRequestInfoBytes(bmods[string(buuid)],
		"pb_data", value, &now)
	addMembershipRequestInfoBytes(bmods[string(buuid)],
		"application_pdf", agreement_data, &now)

	ire, ue, te, err = m.conn.AtomicBatchMutate(bmods, cassandra.ConsistencyLevel_QUORUM)
	if ire != nil {
		return errors.New(ire.Why)
	}
	if ue != nil {
		return errors.New("Unavailable")
	}
	if te != nil {
		return errors.New("Timed out")
	}

	return err
}
