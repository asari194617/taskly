-- Add config key for current day shift cycle
INSERT INTO configs(`key`,`value`) SELECT 'day_shift_current_cycle','' WHERE NOT EXISTS (SELECT 1 FROM configs WHERE `key`='day_shift_current_cycle');
