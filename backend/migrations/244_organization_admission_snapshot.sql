-- Persist admission snapshots, including explicitly unaffiliated requests.
CREATE OR REPLACE FUNCTION stamp_organization_usage() RETURNS trigger LANGUAGE plpgsql AS $$
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
 ELSIF NEW.organization_membership_id IS NOT NULL THEN
   -- Zero is a request-local sentinel for explicitly unaffiliated usage. It is
   -- normalized before FK validation and never stored as a membership ID.
   IF NEW.organization_membership_id=0 THEN
     NEW.organization_team_id:=NULL;
     NEW.organization_membership_id:=NULL;
   ELSE
     SELECT team_id INTO NEW.organization_team_id FROM team_memberships
       WHERE id=NEW.organization_membership_id AND user_id=NEW.user_id;
     IF NOT FOUND THEN RAISE EXCEPTION 'Invalid organization usage attribution'; END IF;
   END IF;
 ELSE
   SELECT team_id,id INTO NEW.organization_team_id,NEW.organization_membership_id
   FROM team_memberships WHERE user_id=NEW.user_id AND joined_at<=attribution_time
   AND (left_at IS NULL OR attribution_time<left_at) ORDER BY joined_at DESC,id DESC LIMIT 1;
 END IF;
 RETURN NEW;
END $$;
