CREATE TABLE enterprises (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL CHECK (length(trim(name)) > 0),
    status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active','disabled')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE enterprise_leaders (
    id BIGSERIAL PRIMARY KEY,
    enterprise_id BIGINT NOT NULL REFERENCES enterprises(id),
    user_id BIGINT NOT NULL REFERENCES users(id),
    joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    left_at TIMESTAMPTZ
);
CREATE UNIQUE INDEX enterprise_leaders_current_user ON enterprise_leaders(user_id) WHERE left_at IS NULL;
CREATE INDEX enterprise_leaders_enterprise ON enterprise_leaders(enterprise_id) WHERE left_at IS NULL;
ALTER TABLE teams ADD COLUMN enterprise_id BIGINT REFERENCES enterprises(id);
CREATE INDEX teams_enterprise ON teams(enterprise_id) WHERE enterprise_id IS NOT NULL;
CREATE TABLE enterprise_events (
    id BIGSERIAL PRIMARY KEY,
    enterprise_id BIGINT NOT NULL REFERENCES enterprises(id),
    actor_id BIGINT NOT NULL REFERENCES users(id),
    kind VARCHAR(50) NOT NULL,
    details JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX enterprise_events_history ON enterprise_events(enterprise_id,created_at DESC,id DESC);

CREATE FUNCTION check_enterprise_integrity() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF EXISTS (SELECT 1 FROM users u WHERE u.role='enterprise_admin' AND u.deleted_at IS NULL
        AND NOT EXISTS (SELECT 1 FROM enterprise_leaders l WHERE l.user_id=u.id AND l.left_at IS NULL)) THEN
        RAISE EXCEPTION '企业负责人必须关联一个企业' USING ERRCODE='23514';
    END IF;
    IF EXISTS (SELECT 1 FROM enterprise_leaders l JOIN users u ON u.id=l.user_id
        WHERE l.left_at IS NULL AND u.deleted_at IS NULL AND u.role<>'enterprise_admin') THEN
        RAISE EXCEPTION '请通过企业管理任免企业负责人' USING ERRCODE='23514';
    END IF;
    IF EXISTS (SELECT 1 FROM team_memberships m JOIN users u ON u.id=m.user_id
        WHERE m.left_at IS NULL AND u.role='enterprise_admin') THEN
        RAISE EXCEPTION '企业负责人不能兼任团队成员' USING ERRCODE='23514';
    END IF;
    IF EXISTS (SELECT 1 FROM enterprises e WHERE e.status='active' AND NOT EXISTS (
        SELECT 1 FROM enterprise_leaders l JOIN users u ON u.id=l.user_id
        WHERE l.enterprise_id=e.id AND l.left_at IS NULL AND u.role='enterprise_admin'
          AND u.status='active' AND u.deleted_at IS NULL)) THEN
        RAISE EXCEPTION '正常企业必须保留至少一名有效负责人' USING ERRCODE='23514';
    END IF;
    RETURN NULL;
END $$;
-- Share the existing governance lock with user and team role changes.
CREATE TRIGGER enterprises_lock BEFORE INSERT OR UPDATE OR DELETE ON enterprises FOR EACH ROW EXECUTE FUNCTION lock_team_integrity();
CREATE TRIGGER enterprise_leaders_lock BEFORE INSERT OR UPDATE OR DELETE ON enterprise_leaders FOR EACH ROW EXECUTE FUNCTION lock_team_integrity();
CREATE CONSTRAINT TRIGGER enterprises_integrity AFTER INSERT OR UPDATE OR DELETE ON enterprises DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_enterprise_integrity();
CREATE CONSTRAINT TRIGGER enterprise_leaders_integrity AFTER INSERT OR UPDATE OR DELETE ON enterprise_leaders DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_enterprise_integrity();
CREATE CONSTRAINT TRIGGER users_enterprise_integrity AFTER INSERT OR UPDATE OF role,status,deleted_at OR DELETE ON users DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_enterprise_integrity();
CREATE CONSTRAINT TRIGGER memberships_enterprise_integrity AFTER INSERT OR UPDATE OR DELETE ON team_memberships DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_enterprise_integrity();
CREATE FUNCTION preserve_team_enterprise() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.enterprise_id IS NOT NULL AND OLD.enterprise_id IS DISTINCT FROM NEW.enterprise_id THEN
        RAISE EXCEPTION '本期不支持移除或变更团队的企业归属' USING ERRCODE='23514';
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER teams_preserve_enterprise BEFORE UPDATE OF enterprise_id ON teams FOR EACH ROW EXECUTE FUNCTION preserve_team_enterprise();
