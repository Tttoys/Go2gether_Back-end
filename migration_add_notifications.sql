-- Migration: Add notifications, availabilities, and available_periods tables
-- Run this if you already have the database and need to add these tables

-- ---------------------------------------------------------------------------
-- Notifications
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type VARCHAR(50) NOT NULL,
    title VARCHAR(255) NOT NULL,
    message TEXT,
    data JSONB,
    action_url TEXT,
    read BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_notifications_user_id ON notifications(user_id);
CREATE INDEX IF NOT EXISTS idx_notifications_read ON notifications(read);
CREATE INDEX IF NOT EXISTS idx_notifications_created_at ON notifications(created_at);
CREATE INDEX IF NOT EXISTS idx_notifications_type ON notifications(type);

-- ---------------------------------------------------------------------------
-- Availabilities
-- ---------------------------------------------------------------------------
-- Check if enum type exists before creating
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'availability_status') THEN
        CREATE TYPE availability_status AS ENUM ('free', 'flexible', 'busy');
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS availabilities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    trip_id UUID NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    date DATE NOT NULL,
    status availability_status NOT NULL DEFAULT 'free',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE(trip_id, user_id, date)
);

CREATE INDEX IF NOT EXISTS idx_availabilities_trip_id ON availabilities(trip_id);
CREATE INDEX IF NOT EXISTS idx_availabilities_user_id ON availabilities(user_id);
CREATE INDEX IF NOT EXISTS idx_availabilities_date ON availabilities(date);
CREATE INDEX IF NOT EXISTS idx_availabilities_status ON availabilities(status);

-- ---------------------------------------------------------------------------
-- Available Periods
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS available_periods (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    trip_id UUID NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    period_number INTEGER NOT NULL,
    start_date DATE NOT NULL,
    end_date DATE NOT NULL,
    duration_days INTEGER,
    free_count INTEGER DEFAULT 0,
    flexible_count INTEGER DEFAULT 0,
    total_members INTEGER,
    availability_percentage DOUBLE PRECISION,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE(trip_id, period_number)
);

CREATE INDEX IF NOT EXISTS idx_available_periods_trip_id ON available_periods(trip_id);
CREATE INDEX IF NOT EXISTS idx_available_periods_period_number ON available_periods(period_number);

