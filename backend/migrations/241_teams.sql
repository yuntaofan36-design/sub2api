CREATE TABLE teams (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL CHECK (length(trim(name)) > 0),
    status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active','disabled')),
    balance NUMERIC(20,8) NOT NULL DEFAULT 0 CHECK (balance >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE team_memberships (
    id BIGSERIAL PRIMARY KEY,
    team_id BIGINT NOT NULL REFERENCES teams(id),
    user_id BIGINT NOT NULL REFERENCES users(id),
    status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active','suspended','left')),
    joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    left_at TIMESTAMPTZ,
    CHECK ((status = 'left') = (left_at IS NOT NULL)),
    UNIQUE (id, team_id, user_id)
);
CREATE UNIQUE INDEX team_memberships_current_user ON team_memberships(user_id) WHERE left_at IS NULL;
CREATE INDEX team_memberships_team ON team_memberships(team_id, status);
CREATE TABLE team_allowed_groups (
    team_id BIGINT NOT NULL REFERENCES teams(id),
    group_id BIGINT NOT NULL REFERENCES groups(id),
    PRIMARY KEY(team_id, group_id)
);
CREATE TABLE team_invitations (
    id BIGSERIAL PRIMARY KEY,
    team_id BIGINT NOT NULL REFERENCES teams(id),
    email VARCHAR(255) NOT NULL,
    token_hash VARCHAR(64) NOT NULL UNIQUE,
    created_by BIGINT NOT NULL REFERENCES users(id),
    expires_at TIMESTAMPTZ NOT NULL,
    accepted_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE team_subscription_grants (
    id BIGSERIAL PRIMARY KEY,
    team_id BIGINT NOT NULL REFERENCES teams(id),
    group_id BIGINT NOT NULL REFERENCES groups(id),
    validity_days INTEGER NOT NULL CHECK (validity_days BETWEEN 1 AND 36500),
    quantity INTEGER NOT NULL CHECK (quantity > 0),
    remaining INTEGER NOT NULL CHECK (remaining >= 0 AND remaining <= quantity),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE team_events (
    id BIGSERIAL PRIMARY KEY,
    team_id BIGINT NOT NULL REFERENCES teams(id),
    actor_id BIGINT NOT NULL REFERENCES users(id),
    member_id BIGINT REFERENCES team_memberships(id),
    kind VARCHAR(50) NOT NULL,
    amount NUMERIC(20,8) NOT NULL DEFAULT 0,
    balance_before NUMERIC(20,8),
    balance_after NUMERIC(20,8),
    details JSONB NOT NULL DEFAULT '{}',
    operation_id VARCHAR(100),
    fingerprint VARCHAR(64),
    result JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(team_id, operation_id)
);
CREATE INDEX team_events_history ON team_events(team_id, created_at DESC, id DESC);

ALTER TABLE api_keys ADD COLUMN team_id BIGINT REFERENCES teams(id),
    ADD COLUMN team_membership_id BIGINT,
    ADD CONSTRAINT api_keys_team_pair CHECK ((team_id IS NULL) = (team_membership_id IS NULL)),
    ADD CONSTRAINT api_keys_team_member FOREIGN KEY (team_membership_id, team_id, user_id)
        REFERENCES team_memberships(id, team_id, user_id);
CREATE INDEX api_keys_team ON api_keys(team_id, team_membership_id) WHERE team_id IS NOT NULL;
ALTER TABLE usage_logs ADD COLUMN team_id BIGINT REFERENCES teams(id),
    ADD COLUMN team_membership_id BIGINT REFERENCES team_memberships(id);
CREATE INDEX usage_logs_team_time ON usage_logs(team_id, created_at DESC, id DESC) WHERE team_id IS NOT NULL;
ALTER TABLE batch_image_jobs ADD COLUMN team_id BIGINT REFERENCES teams(id),
    ADD COLUMN team_membership_id BIGINT REFERENCES team_memberships(id);

-- Key ownership never changes. Looking it up at insert also covers existing
-- async billing writers, retries and batch settlement after a member leaves.
CREATE FUNCTION stamp_team_usage() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    SELECT team_id, team_membership_id INTO NEW.team_id, NEW.team_membership_id
    FROM api_keys WHERE id = NEW.api_key_id;
    RETURN NEW;
END $$;
CREATE TRIGGER usage_logs_team_snapshot BEFORE INSERT ON usage_logs FOR EACH ROW EXECUTE FUNCTION stamp_team_usage();
CREATE TRIGGER batch_jobs_team_snapshot BEFORE INSERT ON batch_image_jobs FOR EACH ROW EXECUTE FUNCTION stamp_team_usage();
CREATE FUNCTION preserve_team_key_owner() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.team_id IS DISTINCT FROM NEW.team_id OR OLD.team_membership_id IS DISTINCT FROM NEW.team_membership_id
       OR (OLD.team_id IS NOT NULL AND OLD.user_id IS DISTINCT FROM NEW.user_id) THEN
        RAISE EXCEPTION 'Team key ownership is immutable' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER api_keys_preserve_team BEFORE UPDATE ON api_keys FOR EACH ROW EXECUTE FUNCTION preserve_team_key_owner();

-- Deferred checks allow appointing a manager and creating membership atomically.
-- They also protect legacy user disable/delete/role-change entry points.
CREATE FUNCTION check_team_integrity() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF EXISTS (SELECT 1 FROM users u WHERE u.deleted_at IS NULL AND u.role = 'team_admin'
        AND NOT EXISTS (SELECT 1 FROM team_memberships m WHERE m.user_id=u.id AND m.left_at IS NULL AND m.status='active')) THEN
        RAISE EXCEPTION 'Team administrators must have an active team membership' USING ERRCODE = '23514';
    END IF;
    IF EXISTS (SELECT 1 FROM team_memberships m JOIN users u ON u.id=m.user_id
        WHERE m.left_at IS NULL AND u.role='admin') THEN
        RAISE EXCEPTION 'Platform administrators cannot be team members' USING ERRCODE = '23514';
    END IF;
    IF EXISTS (SELECT 1 FROM teams t WHERE t.status='active' AND NOT EXISTS (
        SELECT 1 FROM team_memberships m JOIN users u ON u.id=m.user_id
        WHERE m.team_id=t.id AND m.status='active' AND m.left_at IS NULL
          AND u.role='team_admin' AND u.status='active' AND u.deleted_at IS NULL)) THEN
        RAISE EXCEPTION 'An active team must retain an active administrator' USING ERRCODE = '23514';
    END IF;
    RETURN NULL;
END $$;
CREATE FUNCTION lock_team_integrity() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    PERFORM pg_advisory_xact_lock(824001);
    RETURN NEW;
END $$;
CREATE TRIGGER users_team_lock BEFORE UPDATE OF role,status,deleted_at ON users FOR EACH ROW EXECUTE FUNCTION lock_team_integrity();
CREATE CONSTRAINT TRIGGER users_team_integrity AFTER UPDATE OF role,status,deleted_at ON users
    DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_team_integrity();
CREATE CONSTRAINT TRIGGER memberships_team_integrity AFTER INSERT OR UPDATE OR DELETE ON team_memberships
    DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_team_integrity();
CREATE CONSTRAINT TRIGGER teams_integrity AFTER INSERT OR UPDATE ON teams
    DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_team_integrity();
