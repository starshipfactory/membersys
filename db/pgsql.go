package db

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"strconv"

	"github.com/golang/protobuf/proto"
	_ "github.com/lib/pq"
	"github.com/starshipfactory/membersys"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

const allColumns = "id, name, street, city, zipcode, country, email, " +
	"email_verified, phone, fee, fee_yearly, username, pwhash, has_key, " +
	"payments_caught_up_to, request_timestamp, request_source_ip, " +
	"verification_email, approval_timestamp, approver_uid, request_comment, " +
	"user_agent, goodbye_timestamp, goodbye_initiator, goodbye_reason, " +
	"agreement_scan_id"

type scannable interface {
	Scan(...interface{}) error
}

// Implementation of a PostgreSQL client.
type PostgreSQLDB struct {
	db *sql.DB
}

// Establish new PostgreSQL database connection.
func NewPostgreSQLDB(host, dbname, user, password string, ssl bool) (
	*PostgreSQLDB, error) {
	var db *sql.DB
	var dsn string
	var err error

	if host != "" {
		var hostname string
		var port string

		host, port, err = net.SplitHostPort(host)
		if err != nil {
			return nil, err
		}

		dsn += fmt.Sprintf("host=%s port=%s ", hostname, port)
	}

	if user != "" {
		dsn += "user=" + user + " "
	}

	if password != "" {
		dsn += "password=" + password + " "
	}

	if ssl {
		dsn += "sslmode=verify-ca "
	} else {
		dsn += "sslmode=disable "
	}

	dsn += "dbname=" + dbname

	db, err = sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}

	return &PostgreSQLDB{
		db: db,
	}, nil
}

func stringOrNil(value string) interface{} {
	if value == "" {
		return nil
	}

	return value
}

func fullRowToMembershipAgreement(row scannable) (*membersys.MembershipAgreement, int64, error) {
	var agreementScanId int64
	var err error
	var member *membersys.MembershipAgreement = new(membersys.MembershipAgreement)
	member.MemberData = new(membersys.Member)
	member.Metadata = new(membersys.MembershipMetadata)

	err = row.Scan(&member.MemberData.Id, &member.MemberData.Name,
		&member.MemberData.Street, &member.MemberData.City,
		&member.MemberData.Zipcode, &member.MemberData.Country,
		&member.MemberData.Email, &member.MemberData.EmailVerified,
		&member.MemberData.Phone, &member.MemberData.Fee,
		&member.MemberData.FeeYearly, &member.MemberData.Username,
		&member.MemberData.Pwhash, &member.MemberData.HasKey,
		&member.MemberData.PaymentsCaughtUpTo,
		&member.Metadata.RequestTimestamp, &member.Metadata.RequestSourceIp,
		&member.Metadata.VerificationEmail,
		&member.Metadata.ApprovalTimestamp, &member.Metadata.ApproverUid,
		&member.Metadata.Comment, &member.Metadata.UserAgent,
		&member.Metadata.GoodbyeTimestamp, &member.Metadata.GoodbyeInitiator,
		&member.Metadata.GoodbyeReason, &agreementScanId)
	return member, agreementScanId, err
}

func (p *PostgreSQLDB) fetchMembershipAgreementPDF(
	ctx context.Context, id int64) ([]byte, error) {
	var row *sql.Row
	var data []byte
	var err error

	row = p.db.QueryRowContext(ctx,
		"SELECT data FROM membership_agreement_scans WHERE id = ?", id)
	err = row.Scan(&data)
	if err == sql.ErrNoRows {
		return nil, grpc.Errorf(codes.NotFound,
			"No membership agreement PDF found with ID %d", id)
	}
	if err != nil {
		return nil, grpc.Errorf(codes.Internal,
			"Error fetching membership agreement PDF: %s", err.Error())
	}

	return data, nil
}

// Store the given membership request in the database.
func (p *PostgreSQLDB) StoreMembershipRequest(
	ctx context.Context, req *membersys.FormInputData) (string, error) {
	var result sql.Result
	var id int64
	var err error

	result, err = p.db.ExecContext(ctx, "INSERT INTO members (name, street, city, "+
		"zipcode, country, email, phone, fee, username, pwhash, fee_yearly, "+
		"request_timestamp, request_source_ip, request_comment, user_agent, "+
		"membership_status) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, "+
		"'now'::timestamptz, ?, ?, ?, 'APPLICATION')", req.MemberData.Name,
		req.MemberData.Street, req.MemberData.City, req.MemberData.Zipcode,
		req.MemberData.Country, req.MemberData.Email, req.MemberData.Phone,
		req.MemberData.GetFee(), req.MemberData.Username,
		req.MemberData.Pwhash, req.MemberData.GetFeeYearly(),
		req.Metadata.RequestSourceIp, req.Metadata.Comment,
		req.Metadata.UserAgent)
	if err != nil {
		return "", err
	}

	id, err = result.LastInsertId()
	if err != nil {
		return "", err
	}

	return strconv.FormatInt(id, 10), nil
}

// Retrieve a specific members defailed membership data, but fetch it by the
// user name of the member.
func (p *PostgreSQLDB) GetMemberDetailByUsername(
	ctx context.Context, username string) (
	*membersys.MembershipAgreement, error) {
	var member *membersys.MembershipAgreement
	var row *sql.Row
	var agreementId int64
	var err error

	row = p.db.QueryRowContext(ctx, "SELECT "+allColumns+" FROM members "+
		"WHERE username = ?", username)
	member, agreementId, err = fullRowToMembershipAgreement(row)

	if err == sql.ErrNoRows {
		return nil, grpc.Errorf(codes.NotFound,
			"No member found with user name \"%s\"", username)
	}
	if err != nil {
		return nil, grpc.Errorf(codes.Internal,
			"Error fetching member by user name: %s", err.Error())
	}

	if agreementId != 0 {
		member.AgreementPdf, err = p.fetchMembershipAgreementPDF(
			ctx, agreementId)
		if err != nil {
			return nil, err
		}
	}

	return member, nil
}

// Retrieve a specific members defailed membership data, but fetch it by the
// user name of the member.
func (p *PostgreSQLDB) GetMemberDetail(
	ctx context.Context, id string) (
	*membersys.MembershipAgreement, error) {
	var intId int64
	var member *membersys.MembershipAgreement
	var row *sql.Row
	var agreementId int64
	var err error

	intId, err = strconv.ParseInt(id, 10, 64)
	if err != nil {
		return nil, grpc.Errorf(codes.Internal,
			"Cannot parse \"%s\" as a number", id)
	}

	row = p.db.QueryRowContext(ctx, "SELECT "+allColumns+" FROM members "+
		"WHERE id = ?", intId)
	member, agreementId, err = fullRowToMembershipAgreement(row)

	if err == sql.ErrNoRows {
		return nil, grpc.Errorf(codes.NotFound,
			"No member found with member ID \"%d\"", intId)
	}
	if err != nil {
		return nil, grpc.Errorf(codes.Internal,
			"Error fetching member by member ID: %s", err.Error())
	}

	if agreementId != 0 {
		member.AgreementPdf, err = p.fetchMembershipAgreementPDF(
			ctx, agreementId)
		if err != nil {
			return nil, err
		}
	}

	return member, nil
}

// Update the membership fee for the given member.
func (p *PostgreSQLDB) SetMemberFee(
	ctx context.Context, id string, fee uint64, yearly bool) error {
	var intId int64
	var err error

	intId, err = strconv.ParseInt(id, 10, 64)
	if err != nil {
		return grpc.Errorf(codes.Internal,
			"Cannot parse \"%s\" as a number", id)
	}

	_, err = p.db.ExecContext(ctx, "UPDATE members SET fee = ?, "+
		"fee_yearly = ? WHERE id = ?", fee, yearly, intId)
	if err != nil {
		return grpc.Errorf(codes.Internal,
			"Error updating membership fee: %s", err.Error())
	}

	return nil
}

// Update the specified long field for the given member.
func (p *PostgreSQLDB) SetLongValue(
	ctx context.Context, id string, field string, value uint64) error {
	var intId int64
	var err error

	intId, err = strconv.ParseInt(id, 10, 64)
	if err != nil {
		return grpc.Errorf(codes.Internal,
			"Cannot parse \"%s\" as a number", id)
	}

	if field != "payments_caught_up_to" {
		return grpc.Errorf(codes.NotFound, "Unknown field specified: %s",
			field)
	}

	_, err = p.db.ExecContext(ctx, "UPDATE members SET "+field+" = ? "+
		"WHERE id = ?", value, intId)
	if err != nil {
		return grpc.Errorf(codes.Internal,
			"Error updating membership fee: %s", err.Error())
	}

	return nil
}

// Update the specified boolean field for the given member.
func (p *PostgreSQLDB) SetBoolValue(
	ctx context.Context, id string, field string, value bool) error {
	var intId int64
	var err error

	intId, err = strconv.ParseInt(id, 10, 64)
	if err != nil {
		return grpc.Errorf(codes.Internal,
			"Cannot parse \"%s\" as a number", id)
	}

	if field != "has_key" {
		return grpc.Errorf(codes.NotFound, "Unknown field specified: %s",
			field)
	}

	_, err = p.db.ExecContext(ctx, "UPDATE members SET "+field+" = ? "+
		"WHERE id = ?", value, intId)
	if err != nil {
		return grpc.Errorf(codes.Internal,
			"Error updating membership fee: %s", err.Error())
	}

	return nil
}

// Update the specified text field for the given member.
func (p *PostgreSQLDB) SetTextValue(
	ctx context.Context, id string, field, value string) error {
	var intId int64
	var err error

	intId, err = strconv.ParseInt(id, 10, 64)
	if err != nil {
		return grpc.Errorf(codes.Internal,
			"Cannot parse \"%s\" as a number", id)
	}

	if field != "name" && field != "street" && field != "city" &&
		field != "zipcode" && field != "country" && field != "phone" &&
		field != "username" {
		return grpc.Errorf(codes.NotFound, "Unknown field specified: %s",
			field)
	}

	_, err = p.db.ExecContext(ctx, "UPDATE members SET "+field+" = ? "+
		"WHERE id = ?", value, intId)
	if err != nil {
		return grpc.Errorf(codes.Internal,
			"Error updating membership fee: %s", err.Error())
	}

	return nil
}

// Retrieve an individual applicants data.
func (p *PostgreSQLDB) GetMembershipRequest(ctx context.Context, id string) (
	*membersys.MembershipAgreement, error) {
	// Members and applicants are encoded the same way in PostgreSQL.
	return p.GetMemberDetail(ctx, id)
}

func (p *PostgreSQLDB) enumerateMembersOfState(
	ctx context.Context, state, criterion, prev string, num int32,
	members chan<- *membersys.MembershipAgreement, errors chan<- error) {
	var prevId int64
	var rows *sql.Rows
	var err error

	defer close(members)
	defer close(errors)

	if prev != "" {
		prevId, err = strconv.ParseInt(prev, 10, 64)
		if err != nil {
			errors <- grpc.Errorf(codes.Internal,
				"Cannot parse \"%s\" as a number", prev)
			return
		}
	}

	if criterion != "" {
		rows, err = p.db.QueryContext(ctx, "SELECT "+allColumns+" FROM "+
			"members WHERE id > ? AND membership_status = ? AND "+
			"substr(name from 1 for ?) = ? LIMIT ?", prevId, state,
			len(criterion), criterion, num)
	} else {
		rows, err = p.db.QueryContext(ctx, "SELECT "+allColumns+" FROM "+
			"members WHERE id > ? AND membership_status = ? LIMIT ?", prevId,
			state, num)
	}
	if err != nil {
		errors <- grpc.Errorf(codes.Internal,
			"Error fetching member list: %s", err.Error())
		return
	}

	for rows.Next() {
		var member *membersys.MembershipAgreement
		var row *sql.Row
		member, _, err = fullRowToMembershipAgreement(row)
		if err != nil {
			errors <- grpc.Errorf(codes.Internal,
				"Error fetching member by user name: %s", err.Error())
		} else {
			members <- member
		}
	}
}

func (p *PostgreSQLDB) StreamingEnumerateMembers(
	ctx context.Context, prev string, num int32,
	members chan<- *membersys.Member, errors chan<- error) {
	var agreements chan *membersys.MembershipAgreement = make(chan *membersys.MembershipAgreement, cap(members))
	defer close(members)
	go func() {
		var agreement *membersys.MembershipAgreement

		for agreement = range agreements {
			members <- agreement.MemberData
		}
	}()
	p.enumerateMembersOfState(ctx, "ACTIVE", "", prev, num, agreements, errors)
}

// Get a list of all members currently in the database. Returns a set of
// "num" entries beginning after "prev".
// Returns a filled-out member structure.
func (p *PostgreSQLDB) EnumerateMembers(
	ctx context.Context, prev string, num int32) ([]*membersys.Member,
	error) {
	var members chan *membersys.Member = make(chan *membersys.Member)
	var errors chan error = make(chan error)
	var memberList []*membersys.Member
	var member *membersys.Member
	var err error

	go p.StreamingEnumerateMembers(ctx, prev, num, members, errors)

	for {
		select {
		case member = <-members:
			memberList = append(memberList, member)
		case err = <-errors:
		default:
			return memberList, err
		}
	}
}

func (p *PostgreSQLDB) StreamingEnumerateMembershipRequests(
	ctx context.Context, criterion, prev string, num int32,
	agreementsWithKey chan<- *membersys.MembershipAgreementWithKey,
	errors chan<- error) {
	var agreements chan *membersys.MembershipAgreement = make(chan *membersys.MembershipAgreement, cap(agreementsWithKey))
	defer close(agreementsWithKey)
	go func() {
		var agreement *membersys.MembershipAgreement
		var agreementWithKey *membersys.MembershipAgreementWithKey

		for agreement = range agreements {
			agreementWithKey = new(membersys.MembershipAgreementWithKey)
			proto.Merge(&agreementWithKey.MembershipAgreement, agreement)
			agreementWithKey.Key = strconv.FormatUint(
				agreement.MemberData.GetId(), 10)
			agreementsWithKey <- agreementWithKey
		}
	}()
	p.enumerateMembersOfState(ctx, "APPLICATION", criterion, prev, num,
		agreements, errors)
}

// Get a list of all membership applications currently in the database.
// Returns a set of "num" entries beginning after "prev". If "criterion" is
// given, it will be compared against the name of the member.
func (p *PostgreSQLDB) EnumerateMembershipRequests(
	ctx context.Context, criterion, prev string, num int32) (
	[]*membersys.MembershipAgreementWithKey, error) {
	var agreements chan *membersys.MembershipAgreementWithKey = make(chan *membersys.MembershipAgreementWithKey)
	var errors chan error = make(chan error)
	var agreementsWithKey []*membersys.MembershipAgreementWithKey
	var agreementWithKey *membersys.MembershipAgreementWithKey
	var err error

	go p.StreamingEnumerateMembershipRequests(
		ctx, criterion, prev, num, agreements, errors)

	for {
		select {
		case agreementWithKey = <-agreements:
			agreementsWithKey = append(agreementsWithKey, agreementWithKey)
		case err = <-errors:
		default:
			return agreementsWithKey, err
		}
	}
}

func (p *PostgreSQLDB) StreamingEnumerateQueuedMembers(
	ctx context.Context, prev string, num int32,
	queued chan<- *membersys.MemberWithKey, errors chan<- error) {
	var agreements chan *membersys.MembershipAgreement = make(chan *membersys.MembershipAgreement, cap(queued))
	defer close(queued)
	go func() {
		var current *membersys.MembershipAgreement
		var currentWithKey *membersys.MemberWithKey

		for current = range agreements {
			currentWithKey = new(membersys.MemberWithKey)
			proto.Merge(&currentWithKey.Member, current.MemberData)
			currentWithKey.Key = strconv.FormatUint(
				current.MemberData.GetId(), 10)
			queued <- currentWithKey
		}
	}()
	p.enumerateMembersOfState(ctx, "IN_CREATION", "", prev, num,
		agreements, errors)
}

// Get a list of all future members which are currently in the queue.
func (p *PostgreSQLDB) EnumerateQueuedMembers(
	ctx context.Context, prev string, num int32) ([]*membersys.MemberWithKey,
	error) {
	var memberStream chan *membersys.MemberWithKey = make(chan *membersys.MemberWithKey)
	var errors chan error = make(chan error)
	var membersWithKey []*membersys.MemberWithKey
	var memberWithKey *membersys.MemberWithKey
	var err error

	go p.StreamingEnumerateQueuedMembers(
		ctx, prev, num, memberStream, errors)

	for {
		select {
		case memberWithKey = <-memberStream:
			membersWithKey = append(membersWithKey, memberWithKey)
		case err = <-errors:
		default:
			return membersWithKey, err
		}
	}
}

func (p *PostgreSQLDB) StreamingEnumerateDeQueuedMembers(
	ctx context.Context, prev string, num int32,
	queued chan<- *membersys.MemberWithKey, errors chan<- error) {
	var agreements chan *membersys.MembershipAgreement = make(chan *membersys.MembershipAgreement, cap(queued))
	defer close(queued)
	go func() {
		var current *membersys.MembershipAgreement
		var currentWithKey *membersys.MemberWithKey

		for current = range agreements {
			currentWithKey = new(membersys.MemberWithKey)
			proto.Merge(&currentWithKey.Member, current.MemberData)
			currentWithKey.Key = strconv.FormatUint(
				current.MemberData.GetId(), 10)
			queued <- currentWithKey
		}
	}()
	p.enumerateMembersOfState(ctx, "IN_DELETION", "", prev, num,
		agreements, errors)
}

// Get a list of all former members which are currently in the departing
// members queue.
func (p *PostgreSQLDB) EnumerateDeQueuedMembers(
	ctx context.Context, prev string, num int32) ([]*membersys.MemberWithKey,
	error) {
	var memberStream chan *membersys.MemberWithKey = make(chan *membersys.MemberWithKey)
	var errors chan error = make(chan error)
	var membersWithKey []*membersys.MemberWithKey
	var memberWithKey *membersys.MemberWithKey
	var err error

	go p.StreamingEnumerateDeQueuedMembers(
		ctx, prev, num, memberStream, errors)

	for {
		select {
		case memberWithKey = <-memberStream:
			membersWithKey = append(membersWithKey, memberWithKey)
		case err = <-errors:
		default:
			return membersWithKey, err
		}
	}
}

func (p *PostgreSQLDB) StreamingEnumerateTrashedMembers(
	ctx context.Context, prev string, num int32,
	queued chan<- *membersys.MemberWithKey, errors chan<- error) {
	var agreements chan *membersys.MembershipAgreement = make(chan *membersys.MembershipAgreement, cap(queued))
	defer close(queued)
	go func() {
		var current *membersys.MembershipAgreement
		var currentWithKey *membersys.MemberWithKey

		for current = range agreements {
			currentWithKey = new(membersys.MemberWithKey)
			proto.Merge(&currentWithKey.Member, current.MemberData)
			currentWithKey.Key = strconv.FormatUint(
				current.MemberData.GetId(), 10)
			queued <- currentWithKey
		}
	}()
	p.enumerateMembersOfState(ctx, "ARCHIVED", "", prev, num,
		agreements, errors)
}

// Get a list of all members which are currently in the trash.
func (p *PostgreSQLDB) EnumerateTrashedMembers(
	ctx context.Context, prev string, num int32) ([]*membersys.MemberWithKey,
	error) {
	var memberStream chan *membersys.MemberWithKey = make(chan *membersys.MemberWithKey)
	var errors chan error = make(chan error)
	var membersWithKey []*membersys.MemberWithKey
	var memberWithKey *membersys.MemberWithKey
	var err error

	go p.StreamingEnumerateTrashedMembers(
		ctx, prev, num, memberStream, errors)

	for {
		select {
		case memberWithKey = <-memberStream:
			membersWithKey = append(membersWithKey, memberWithKey)
		case err = <-errors:
		default:
			return membersWithKey, err
		}
	}
}

// Move a member record to the queue for getting their user account removed
// (e.g. when they leave us).
func (p *PostgreSQLDB) MoveMemberToTrash(
	ctx context.Context, id, initiator, reason string) error {
	var intId int64
	var err error

	intId, err = strconv.ParseInt(id, 10, 64)
	if err != nil {
		return grpc.Errorf(codes.Internal,
			"Cannot parse \"%s\" as a number", id)
	}

	_, err = p.db.ExecContext(ctx, "UPDATE members SET membership_status = "+
		"'IN_DELETION', goodbye_initiator = ?, goodbye_reason = ?, "+
		"goodbye_timestamp = 'now'::timestamptz WHERE id = ?", initiator,
		reason, intId)

	if err != nil {
		return grpc.Errorf(codes.Internal,
			"Error updating membership fee: %s", err.Error())
	}

	return nil
}

func (p *PostgreSQLDB) updateMemberStatus(
	ctx context.Context, member *membersys.MemberWithKey, status string) error {
	var intId int64
	var err error

	intId, err = strconv.ParseInt(member.Key, 10, 64)
	if err != nil {
		return grpc.Errorf(codes.Internal,
			"Cannot parse \"%s\" as a number", member.Key)
	}

	_, err = p.db.ExecContext(ctx, "UPDATE members SET membership_status = ?"+
		" WHERE id = ?", status, intId)

	if err != nil {
		return grpc.Errorf(codes.Internal,
			"Error updating membership fee: %s", err.Error())
	}

	return nil
}

// Move the record of the given queued member from the queue of new users to
// the list of active users. This method is to be used by the account creation
// software.
func (p *PostgreSQLDB) MoveNewMemberToFullMember(
	ctx context.Context, member *membersys.MemberWithKey) error {
	return p.updateMemberStatus(ctx, member, "ACTIVE")
}

// Move the record of the given dequeued member from the queue of deleted
// users to the list of archived members. Set the retention to 2 years instead
// of just 6 months, since they have been a member. This method is to be used
// by the account deletion software.
func (p *PostgreSQLDB) MoveDeletedMemberToArchive(
	ctx context.Context, member *membersys.MemberWithKey) error {
	return p.updateMemberStatus(ctx, member, "ARCHIVED")
}

func (p *PostgreSQLDB) updateMemberStatusWithInitiator(
	ctx context.Context, id, initiator, status string) error {
	var intId int64
	var err error

	intId, err = strconv.ParseInt(id, 10, 64)
	if err != nil {
		return grpc.Errorf(codes.Internal,
			"Cannot parse \"%s\" as a number", id)
	}

	_, err = p.db.ExecContext(ctx, "UPDATE members SET membership_status = "+
		"?, approver_uid = ?, approval_timestamp = 'now'::timestamptz "+
		"WHERE id = ?", status, initiator, intId)

	if err != nil {
		return grpc.Errorf(codes.Internal,
			"Error updating membership fee: %s", err.Error())
	}

	return nil
}

// Move the record of the given applicant to the queue of new users to be
// processed. The approver will be set to "initiator".
func (p *PostgreSQLDB) MoveApplicantToNewMember(
	ctx context.Context, id, initiator string) error {
	return p.updateMemberStatusWithInitiator(ctx, id, initiator, "IN_CREATION")
}

// Move the record of the given applicant to a temporary archive of deleted
// applications. The deleter will be set to "initiator".
func (p *PostgreSQLDB) MoveApplicantToTrash(
	ctx context.Context, id, initiator string) error {
	return p.updateMemberStatusWithInitiator(ctx, id, initiator, "ARCHIVED")
}

// Move a member from the queue to the trash (e.g. if they can't be processed).
func (p *PostgreSQLDB) MoveQueuedRecordToTrash(
	ctx context.Context, id, initiator string) error {
	return p.updateMemberStatusWithInitiator(ctx, id, initiator, "ARCHIVED")
}

// Add the membership agreement form scan to the given membership request
// record.
func (p *PostgreSQLDB) StoreMembershipAgreement(
	ctx context.Context, id string, agreement_data []byte) error {
	var result sql.Result
	var memberId int64
	var insertId int64
	var err error

	memberId, err = strconv.ParseInt(id, 10, 64)
	if err != nil {
		return grpc.Errorf(codes.Internal, "Invalid member ID: \"%s\"", id)
	}

	result, err = p.db.ExecContext(ctx,
		"INSERT INTO membership_agreement_scans (data) VALUES (?)",
		agreement_data)
	if err != nil {
		return grpc.Errorf(codes.Internal,
			"Error inserting membership agreement PDF: %s", err.Error())
	}

	insertId, err = result.LastInsertId()
	if err != nil {
		return grpc.Errorf(codes.Internal,
			"Error inserting membership agreement PDF: %s", err.Error())
	}

	_, err = p.db.ExecContext(ctx,
		"UPDATE members SET agreement_scan_id = ? WHERE id = ?",
		insertId, memberId)
	if err != nil {
		return grpc.Errorf(codes.Internal,
			"Error updating member record with agreement PDF: %s",
			err.Error())
	}

	return nil
}
