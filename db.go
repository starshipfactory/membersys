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
	Key string
	Member
}

// List of all relevant columns; used for a few copies here.
var allColumns [][]byte = [][]byte{
	[]byte("name"), []byte("street"), []byte("city"), []byte("zipcode"),
	[]byte("country"), []byte("email"), []byte("email_verified"),
	[]byte("phone"), []byte("fee"), []byte("username"), []byte("pwhash"),
	[]byte("fee_yearly"), []byte("sourceip"), []byte("useragent"),
	[]byte("metadata"), []byte("pb_data"), []byte("application_pdf"),
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

// Retrieve an individual applicants data.
func (m *MembershipDB) GetMembershipRequest(id string) (*MembershipAgreement, int64, error) {
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

	cp.ColumnFamily = "application"
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

// Move the record of the given applicant to the queue of new users to be
// processed. The approver will be set to "initiator".
func (m *MembershipDB) MoveApplicantToNewMember(id, initiator string) error {
	return m.moveApplicantToTable(id, initiator, "membership_queue", 0)
}

// Move the record of the given applicant to a temporary archive of deleted
// applications. The deleter will be set to "initiator".
func (m *MembershipDB) MoveApplicantToTrash(id, initiator string) error {
	return m.moveApplicantToTable(id, initiator, "membership_archive",
		int32(6*30*24*60*60))
}

// Move the record of the given applicant to a different column family.
func (m *MembershipDB) moveApplicantToTable(id, initiator, table string, ttl int32) error {
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
	if member, timestamp, err = m.GetMembershipRequest(id); err != nil {
		return err
	}

	if len(member.AgreementPdf) == 0 {
		return errors.New("No membership agreement scan has been uploaded")
	}

	// Fill in details concerning the approval.
	member.Metadata.ApproverUid = proto.String(initiator)
	member.Metadata.ApprovalTimestamp = proto.Uint64(uint64(now.Unix()))

	bmods = make(map[string]map[string][]*cassandra.Mutation)
	bmods[string(uuid)] = make(map[string][]*cassandra.Mutation)
	bmods[string(uuid)][table] = make([]*cassandra.Mutation, 0)
	bmods[string(uuid)]["application"] = make([]*cassandra.Mutation, 0)

	value, err = proto.Marshal(member)
	if err != nil {
		return err
	}

	// Add the application protobuf to the membership data.
	bmods[string(uuid)][table] = append(bmods[string(uuid)][table],
		newCassandraMutationBytes("pb_data", value, &now, ttl))

	// Delete the application data.
	mutation.Deletion = cassandra.NewDeletion()
	mutation.Deletion.Predicate = cassandra.NewSlicePredicate()
	mutation.Deletion.Predicate.ColumnNames = allColumns
	mutation.Deletion.Timestamp = timestamp
	bmods[string(uuid)]["application"] = append(
		bmods[string(uuid)]["application"], mutation)

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

	agreement, _, err = m.GetMembershipRequest(id)
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
