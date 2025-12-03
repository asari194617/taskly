-- Add index on business_calendar(date) for lookups

CREATE INDEX idx_business_calendar_date ON business_calendar(date);
