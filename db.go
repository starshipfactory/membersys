/*
 * (c) 2014, Tonnerre Lombard <tonnerre@ancient-solutions.com>,
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
)

type MembershipDB struct {
	conn *cassandra.RetryCassandraClient
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
func newCassandraMutationBytes(name string, value []byte, now *time.Time) *cassandra.Mutation {
	var ret = cassandra.NewMutation()
	var col = cassandra.NewColumn()

	col.Timestamp = now.UnixNano()
	col.Name = []byte(name)
	col.Value = value

	ret.ColumnOrSupercolumn = cassandra.NewColumnOrSuperColumn()
	ret.ColumnOrSupercolumn.Column = col

	return ret
}

// Create a new mutation with the given name, value and time stamp.
func newCassandraMutationString(name, value string, now *time.Time) *cassandra.Mutation {
	return newCassandraMutationBytes(name, []byte(value), now)
}

// Funciton for lazy people to add a column to a membership request mutation map.
func addMembershipRequestInfoBytes(mmap map[string][]*cassandra.Mutation, name string, value []byte, now *time.Time) {
	mmap["application"] = append(mmap["application"],
		newCassandraMutationBytes(name, value, now))
}

// Funciton for lazy people to add a column to a membership request mutation map.
func addMembershipRequestInfoString(mmap map[string][]*cassandra.Mutation, name string, value *string, now *time.Time) {
	if value != nil && len(*value) > 0 {
		mmap["application"] = append(mmap["application"],
			newCassandraMutationString(name, *value, now))
	}
}

// Store the given membership request in the database.
func (m *MembershipDB) StoreMembershipRequest(req *Member) (key string, err error) {
	var bmods map[string]map[string][]*cassandra.Mutation
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

	addMembershipRequestInfoString(bmods[c_key], "name", req.Name, &now)
	addMembershipRequestInfoString(bmods[c_key], "street", req.Street, &now)
	addMembershipRequestInfoString(bmods[c_key], "city", req.City, &now)
	addMembershipRequestInfoString(bmods[c_key], "zipcode", req.Zipcode, &now)
	addMembershipRequestInfoString(bmods[c_key], "country", req.Country, &now)
	addMembershipRequestInfoString(bmods[c_key], "email", req.Email, &now)
	addMembershipRequestInfoBytes(bmods[c_key], "email_verified", []byte{0}, &now)
	addMembershipRequestInfoString(bmods[c_key], "phone", req.Phone, &now)
	bdata = make([]byte, 8)
	binary.BigEndian.PutUint64(bdata, req.GetFee())
	addMembershipRequestInfoBytes(bmods[c_key], "fee", bdata, &now)
	addMembershipRequestInfoString(bmods[c_key], "username", req.Username, &now)
	addMembershipRequestInfoString(bmods[c_key], "pwhash", req.Pwhash, &now)
	if req.GetFeeYearly() {
		addMembershipRequestInfoBytes(bmods[c_key], "fee_yearly", []byte{1}, &now)
	} else {
		addMembershipRequestInfoBytes(bmods[c_key], "fee_yearly", []byte{0}, &now)
	}

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
