-- Performance indexes for hot-path queries
-- These indexes optimize the most frequently executed queries in the platform.

-- Optimize GetPendingRides: WHERE status = 'requested' ORDER BY requested_at ASC
-- Used by drivers checking for available rides
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_rides_status_requested_at
    ON rides (status, requested_at ASC)
    WHERE status = 'requested';

-- Optimize driver matching: rides within last 30 days for acceptance rate calculation
-- Used by GetDriverMatchStats during real-time matching
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_rides_driver_created_30d
    ON rides (driver_id, created_at DESC)
    WHERE created_at > NOW() - INTERVAL '30 days';

-- Optimize wallet lookups: frequently queried during payment flows
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_wallets_user_id
    ON wallets (user_id);

-- Optimize active ride lookups: drivers and riders checking current ride
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_rides_active_rider
    ON rides (rider_id)
    WHERE status IN ('requested', 'accepted', 'in_progress');

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_rides_active_driver
    ON rides (driver_id)
    WHERE status IN ('accepted', 'in_progress') AND driver_id IS NOT NULL;

-- Optimize payment stats: analytics aggregate queries
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_payments_status_created
    ON payments (status, created_at DESC);
