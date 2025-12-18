-- enable row level security on all pulse tables
-- using (select auth.uid()) for better performance as per supabase docs

-- enable RLS on users_profile
ALTER TABLE pulse.users_profile ENABLE ROW LEVEL SECURITY;

-- users can read their own profile
CREATE POLICY "users can view own profile"
ON pulse.users_profile FOR SELECT
TO authenticated
USING ((select auth.uid())::text = external_id);

-- users can update their own profile
CREATE POLICY "users can update own profile"
ON pulse.users_profile FOR UPDATE
TO authenticated
USING ((select auth.uid())::text = external_id);

-- users can insert their own profile (first login/registration)
CREATE POLICY "users can insert own profile"
ON pulse.users_profile FOR INSERT
TO authenticated
WITH CHECK ((select auth.uid())::text = external_id);

-- enable RLS on communities
ALTER TABLE pulse.communities ENABLE ROW LEVEL SECURITY;

-- anyone authenticated can read active communities
CREATE POLICY "authenticated users can view active communities"
ON pulse.communities FOR SELECT
TO authenticated
USING (is_active = true);

-- creators can update their own communities
CREATE POLICY "creators can update own communities"
ON pulse.communities FOR UPDATE
TO authenticated
USING (
    creator_id IN (
        SELECT id FROM pulse.users_profile 
        WHERE external_id = (select auth.uid())::text
    )
);

-- authenticated users can create communities
CREATE POLICY "authenticated users can create communities"
ON pulse.communities FOR INSERT
TO authenticated
WITH CHECK (
    creator_id IN (
        SELECT id FROM pulse.users_profile 
        WHERE external_id = (select auth.uid())::text
    )
);

-- enable RLS on activity_events
ALTER TABLE pulse.activity_events ENABLE ROW LEVEL SECURITY;

-- users can view their own activity events
CREATE POLICY "users can view own activity events"
ON pulse.activity_events FOR SELECT
TO authenticated
USING (
    user_id IN (
        SELECT id FROM pulse.users_profile 
        WHERE external_id = (select auth.uid())::text
    )
);

-- users can insert their own activity events
CREATE POLICY "users can insert own activity events"
ON pulse.activity_events FOR INSERT
TO authenticated
WITH CHECK (
    user_id IN (
        SELECT id FROM pulse.users_profile 
        WHERE external_id = (select auth.uid())::text
    )
);

-- service role bypass: allow service role full access
-- this is for background jobs and admin operations

CREATE POLICY "service role full access to users_profile"
ON pulse.users_profile FOR ALL
TO service_role
USING (true)
WITH CHECK (true);

CREATE POLICY "service role full access to communities"
ON pulse.communities FOR ALL
TO service_role
USING (true)
WITH CHECK (true);

CREATE POLICY "service role full access to activity_events"
ON pulse.activity_events FOR ALL
TO service_role
USING (true)
WITH CHECK (true);
