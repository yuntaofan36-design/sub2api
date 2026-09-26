-- Organization reporting is separate from API key ownership and billing.
ALTER TABLE enterprises ADD COLUMN allow_member_usage_view boolean NOT NULL DEFAULT false;
ALTER TABLE usage_logs
    ADD COLUMN request_submitted_at timestamptz,
    ADD COLUMN organization_team_id bigint REFERENCES teams(id),
    ADD COLUMN organization_membership_id bigint REFERENCES team_memberships(id);
ALTER TABLE batch_image_jobs
    ADD COLUMN request_submitted_at timestamptz,
    ADD COLUMN organization_team_id bigint REFERENCES teams(id),
    ADD COLUMN organization_membership_id bigint REFERENCES team_memberships(id);

-- Historical personal usage has only its accounting timestamp. Never infer a
-- membership where none was recorded, and never move a team key's attribution.
UPDATE usage_logs SET organization_team_id=team_id, organization_membership_id=team_membership_id
 WHERE team_id IS NOT NULL;
UPDATE usage_logs l SET organization_team_id=m.team_id, organization_membership_id=m.id
 FROM team_memberships m WHERE l.team_id IS NULL AND l.user_id=m.user_id
 AND l.created_at>=m.joined_at AND (m.left_at IS NULL OR l.created_at<m.left_at);
UPDATE batch_image_jobs SET request_submitted_at=created_at,
 organization_team_id=team_id, organization_membership_id=team_membership_id;
UPDATE batch_image_jobs j SET organization_team_id=m.team_id, organization_membership_id=m.id
 FROM team_memberships m WHERE j.team_id IS NULL AND j.user_id=m.user_id
 AND j.created_at>=m.joined_at AND (m.left_at IS NULL OR j.created_at<m.left_at);

-- Admission time is carried through async billing. The historical membership
-- interval also handles requests finishing after a departure or a later rejoin.
CREATE FUNCTION stamp_organization_usage() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE owner_team bigint; owner_membership bigint; attribution_time timestamptz;
BEGIN
 SELECT team_id,team_membership_id INTO owner_team,owner_membership FROM api_keys WHERE id=NEW.api_key_id;
 attribution_time := COALESCE(NEW.request_submitted_at,NEW.created_at);
 IF TG_TABLE_NAME='batch_image_jobs' THEN
   NEW.request_submitted_at := attribution_time;
 END IF;
 IF owner_team IS NOT NULL THEN
   NEW.organization_team_id := owner_team;
   NEW.organization_membership_id := owner_membership;
 ELSE
   SELECT team_id,id INTO NEW.organization_team_id,NEW.organization_membership_id
   FROM team_memberships WHERE user_id=NEW.user_id AND joined_at<=attribution_time
   AND (left_at IS NULL OR attribution_time<left_at) ORDER BY joined_at DESC,id DESC LIMIT 1;
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER stamp_organization_usage_before_insert BEFORE INSERT ON usage_logs
 FOR EACH ROW EXECUTE FUNCTION stamp_organization_usage();
CREATE TRIGGER stamp_organization_batch_before_insert BEFORE INSERT ON batch_image_jobs
 FOR EACH ROW EXECUTE FUNCTION stamp_organization_usage();
CREATE INDEX usage_logs_organization_time_idx ON usage_logs(organization_team_id,created_at DESC,user_id)
 WHERE organization_team_id IS NOT NULL;
CREATE INDEX team_memberships_user_interval_idx ON team_memberships(user_id,joined_at,left_at);
