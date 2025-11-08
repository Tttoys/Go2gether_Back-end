-- Create users table
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create indexes for better performance
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);

-- Create a function to automatically update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Create trigger to automatically update updated_at
CREATE TRIGGER update_users_updated_at 
    BEFORE UPDATE ON users 
    FOR EACH ROW 
    EXECUTE FUNCTION update_updated_at_column();

-- Create auth_verifications table for OTP/verification codes
CREATE TABLE IF NOT EXISTS auth_verifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    email VARCHAR(255) NOT NULL,
    code VARCHAR(10) NOT NULL,
    purpose VARCHAR(50) NOT NULL DEFAULT 'password_reset',
    used BOOLEAN DEFAULT FALSE,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create indexes for auth_verifications
CREATE INDEX IF NOT EXISTS idx_auth_verifications_user_id ON auth_verifications(user_id);
CREATE INDEX IF NOT EXISTS idx_auth_verifications_email ON auth_verifications(email);
CREATE INDEX IF NOT EXISTS idx_auth_verifications_code ON auth_verifications(code);
CREATE INDEX IF NOT EXISTS idx_auth_verifications_expires_at ON auth_verifications(expires_at);

-- ---------------------------------------------------------------------------
-- Profiles
-- ---------------------------------------------------------------------------
-- Create profiles table
CREATE TABLE IF NOT EXISTS profiles (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    username VARCHAR(255) UNIQUE NOT NULL,
    first_name VARCHAR(255),
    last_name VARCHAR(255),
    display_name VARCHAR(255),
    avatar_url TEXT,
    phone VARCHAR(50),
    bio TEXT,
    birth_date DATE,
    food_preferences TEXT,
    chronic_disease TEXT,
    allergic_food TEXT,
    allergic_drugs TEXT,
    emergency_contact TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create indexes for profiles
CREATE INDEX IF NOT EXISTS idx_profiles_username ON profiles(username);
CREATE INDEX IF NOT EXISTS idx_profiles_user_id ON profiles(user_id);

-- Trigger to auto-update updated_at on profiles
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_trigger WHERE tgname = 'update_profiles_updated_at'
    ) THEN
        CREATE TRIGGER update_profiles_updated_at
            BEFORE UPDATE ON profiles
            FOR EACH ROW
            EXECUTE FUNCTION update_updated_at_column();
    END IF;
END $$;

-- ---------------------------------------------------------------------------
-- Trips
-- ---------------------------------------------------------------------------
-- Create trips table (if not exists)
CREATE TABLE IF NOT EXISTS trips (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    destination TEXT NOT NULL,
    start_date TIMESTAMP WITH TIME ZONE NOT NULL,
    end_date TIMESTAMP WITH TIME ZONE NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'draft', -- draft | published | cancelled
    total_budget DOUBLE PRECISION NOT NULL DEFAULT 0,
    currency TEXT NOT NULL DEFAULT 'THB',
    creator_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Trigger to auto-update updated_at on trips
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_trigger WHERE tgname = 'update_trips_updated_at'
    ) THEN
        CREATE TRIGGER update_trips_updated_at
            BEFORE UPDATE ON trips
            FOR EACH ROW
            EXECUTE FUNCTION update_updated_at_column();
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_trips_status ON trips(status);
CREATE INDEX IF NOT EXISTS idx_trips_creator_id ON trips(creator_id);
CREATE INDEX IF NOT EXISTS idx_trips_created_at ON trips(created_at);

-- ---------------------------------------------------------------------------
-- Trip Members
-- ---------------------------------------------------------------------------
-- Each user can belong to a trip with a role and status
CREATE TABLE IF NOT EXISTS trip_members (
    trip_id UUID NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role TEXT NOT NULL DEFAULT 'member',              -- creator | member
    status TEXT NOT NULL DEFAULT 'pending',           -- pending | accepted | declined
    availability_submitted BOOLEAN NOT NULL DEFAULT FALSE,
    invited_at TIMESTAMP WITH TIME ZONE NULL,
    joined_at TIMESTAMP WITH TIME ZONE NULL,
    PRIMARY KEY (trip_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_trip_members_trip_id ON trip_members(trip_id);
CREATE INDEX IF NOT EXISTS idx_trip_members_user_id ON trip_members(user_id);
CREATE INDEX IF NOT EXISTS idx_trip_members_status ON trip_members(status);
