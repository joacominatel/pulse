-- disable row level security and drop all policies

-- drop policies on activity_events
DROP POLICY IF EXISTS "users can view own activity events" ON pulse.activity_events;
DROP POLICY IF EXISTS "users can insert own activity events" ON pulse.activity_events;
DROP POLICY IF EXISTS "service role full access to activity_events" ON pulse.activity_events;
ALTER TABLE pulse.activity_events DISABLE ROW LEVEL SECURITY;

-- drop policies on communities
DROP POLICY IF EXISTS "authenticated users can view active communities" ON pulse.communities;
DROP POLICY IF EXISTS "creators can update own communities" ON pulse.communities;
DROP POLICY IF EXISTS "authenticated users can create communities" ON pulse.communities;
DROP POLICY IF EXISTS "service role full access to communities" ON pulse.communities;
ALTER TABLE pulse.communities DISABLE ROW LEVEL SECURITY;

-- drop policies on users_profile
DROP POLICY IF EXISTS "users can view own profile" ON pulse.users_profile;
DROP POLICY IF EXISTS "users can update own profile" ON pulse.users_profile;
DROP POLICY IF EXISTS "users can insert own profile" ON pulse.users_profile;
DROP POLICY IF EXISTS "service role full access to users_profile" ON pulse.users_profile;
ALTER TABLE pulse.users_profile DISABLE ROW LEVEL SECURITY;
