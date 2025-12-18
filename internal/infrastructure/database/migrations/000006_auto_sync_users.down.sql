-- remove auto user sync trigger and function

-- drop trigger first
drop trigger if exists on_auth_user_created on auth.users;

-- drop function
drop function if exists pulse.handle_auth_user_created();

-- revoke permissions
revoke execute on function pulse.handle_auth_user_created() from supabase_auth_admin;
revoke insert on pulse.users_profile from supabase_auth_admin;
