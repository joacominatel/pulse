-- automatic user profile sync from auth.users
-- creates a profile in pulse.users_profile when a new user signs up via supabase auth

-- function to handle new user creation
-- runs with elevated permissions to write to pulse schema
create or replace function pulse.handle_auth_user_created()
returns trigger
language plpgsql
security definer
set search_path = ''
as $$
declare
    derived_username text;
    username_base text;
    username_suffix int := 0;
    final_username text;
begin
    -- derive username from email (part before @)
    -- handles edge cases: empty email, no @ symbol
    if new.email is not null and new.email like '%@%' then
        username_base := lower(split_part(new.email, '@', 1));
    else
        -- fallback to user id prefix if email is weird
        username_base := 'user_' || left(new.id::text, 8);
    end if;
    
    -- sanitize: keep only alphanumeric and underscore, limit to 47 chars (leave room for suffix)
    username_base := regexp_replace(username_base, '[^a-z0-9_]', '_', 'g');
    username_base := left(username_base, 47);
    
    -- ensure minimum length of 3 chars
    if length(username_base) < 3 then
        username_base := username_base || '_user';
    end if;
    
    final_username := username_base;
    
    -- handle conflicts by appending numeric suffix
    -- try up to 100 times before giving up
    loop
        begin
            insert into pulse.users_profile (
                id,
                external_id,
                username,
                display_name,
                created_at,
                updated_at
            ) values (
                gen_random_uuid(),
                new.id::text,
                final_username,
                coalesce(new.raw_user_meta_data->>'full_name', new.raw_user_meta_data->>'name', final_username),
                now(),
                now()
            );
            -- success, exit loop
            exit;
        exception when unique_violation then
            -- username taken, try with suffix
            username_suffix := username_suffix + 1;
            if username_suffix > 100 then
                -- too many attempts, use uuid suffix as last resort
                final_username := left(username_base, 40) || '_' || left(new.id::text, 8);
                
                insert into pulse.users_profile (
                    id,
                    external_id,
                    username,
                    display_name,
                    created_at,
                    updated_at
                ) values (
                    gen_random_uuid(),
                    new.id::text,
                    final_username,
                    coalesce(new.raw_user_meta_data->>'full_name', new.raw_user_meta_data->>'name', final_username),
                    now(),
                    now()
                );
                exit;
            end if;
            final_username := username_base || '_' || username_suffix::text;
        end;
    end loop;
    
    return new;
end;
$$;

-- trigger that fires when new user signs up
create trigger on_auth_user_created
    after insert on auth.users
    for each row
    execute function pulse.handle_auth_user_created();

-- grant permissions to supabase_auth_admin to execute the function
grant execute on function pulse.handle_auth_user_created() to supabase_auth_admin;

-- grant insert permission on users_profile to supabase_auth_admin
grant insert on pulse.users_profile to supabase_auth_admin;
