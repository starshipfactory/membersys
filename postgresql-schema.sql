--
-- Name: member_status; Type: TYPE; Schema: public; Owner: caoimhe
--

CREATE TYPE member_status AS ENUM (
    'APPLICATION',
    'IN_CREATION',
    'ACTIVE',
    'IN_DELETION',
    'ARCHIVED'
);


--
-- Name: members; Type: TABLE; Schema: public; Owner: caoimhe
--

CREATE TABLE members (
    id bigserial NOT NULL UNIQUE PRIMARY KEY,
    name text NOT NULL,
    street text NOT NULL,
    city text NOT NULL,
    zipcode text NOT NULL,
    country text NOT NULL,
    email text NOT NULL UNIQUE,
    email_verified boolean DEFAULT false NOT NULL,
    verification_email text,
    phone text,
    fee bigint NOT NULL,
    username text UNIQUE,
    pwhash text,
    fee_yearly boolean NOT NULL,
    has_key boolean DEFAULT false NOT NULL,
    payments_caught_up_to timestamp with time zone,
    request_timestamp timestamp with time zone NOT NULL,
    request_source_ip inet NOT NULL,
    approval_timestamp timestamp with time zone,
    approver_uid text,
    request_comment text,
    user_agent text NOT NULL,
    goodbye_timestamp timestamp with time zone,
    goodbye_initiator text,
    goodbye_reason text,
    agreement_scan_id bigint,
    membership_status public.member_status DEFAULT 'APPLICATION'::public.member_status NOT NULL
);


--
-- Name: membership_agreement_scans; Type: TABLE; Schema: public; Owner: caoimhe
--

CREATE TABLE membership_agreement_scans (
    id bigserial NOT NULL UNIQUE PRIMARY KEY,
    data bytea NOT NULL
);


--
-- Name: members agreement_scan_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: caoimhe
--

ALTER TABLE ONLY members
    ADD CONSTRAINT agreement_scan_id_fkey FOREIGN KEY (agreement_scan_id) REFERENCES membership_agreement_scans(id) ON DELETE CASCADE;
