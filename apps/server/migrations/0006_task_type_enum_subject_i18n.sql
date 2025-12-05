-- Migration: Convert task type to ENUM and add bilingual subject names
-- WARNING: This is a destructive migration that will:
-- 1. Map existing localized type strings to ENUM values
-- 2. Delete any tasks with unmappable type values
-- 3. Convert type column to ENUM
-- 4. Add name_en to subjects for English translations

-- Ensure the type column exists before mapping
-- Add type column if missing (runner ignores duplicate column errors)
ALTER TABLE tasks
  ADD COLUMN type VARCHAR(32) NOT NULL DEFAULT 'OTHER';

-- Step 1: Map common localized strings to ENUM values
-- English mappings
UPDATE tasks SET type = 'ONLINE' WHERE type IN ('Online', 'online', 'ONLINE');
UPDATE tasks SET type = 'FACE_TO_FACE' WHERE type IN ('Face-to-face', 'face-to-face', 'FACE_TO_FACE', 'Face to Face');
UPDATE tasks SET type = 'QUIZ' WHERE type IN ('Quiz', 'quiz', 'QUIZ', 'Test', 'test');
UPDATE tasks SET type = 'PRESENTATION' WHERE type IN ('Presentation', 'presentation', 'PRESENTATION');
UPDATE tasks SET type = 'SELF_STUDY' WHERE type IN ('Self-study', 'self-study', 'SELF_STUDY', 'Self-study/Preview', 'Preview');
UPDATE tasks SET type = 'OTHER' WHERE type IN ('Other', 'other', 'OTHER');

-- Japanese mappings
UPDATE tasks SET type = 'ONLINE' WHERE type IN ('オンライン', 'ｵﾝﾗｲﾝ');
UPDATE tasks SET type = 'FACE_TO_FACE' WHERE type IN ('対面', '対面授業', '授業');
UPDATE tasks SET type = 'QUIZ' WHERE type IN ('小テスト', '小ﾃｽﾄ', 'テスト', 'ﾃｽﾄ');
UPDATE tasks SET type = 'PRESENTATION' WHERE type IN ('プレゼン', 'ﾌﾟﾚｾﾞﾝ', 'プレゼンテーション', 'ﾌﾟﾚｾﾞﾝﾃｰｼｮﾝ', '発表');
UPDATE tasks SET type = 'SELF_STUDY' WHERE type IN ('予習', '自習', '自主学習', '予習・復習', '復習');
UPDATE tasks SET type = 'OTHER' WHERE type IN ('その他', 'ｿﾉ他');

-- Step 2: Delete any remaining tasks with invalid type values
DELETE FROM tasks 
WHERE type NOT IN ('ONLINE','FACE_TO_FACE','QUIZ','PRESENTATION','SELF_STUDY','OTHER');

-- Step 3: Alter tasks table to use ENUM for type
ALTER TABLE tasks 
  MODIFY COLUMN type ENUM('ONLINE','FACE_TO_FACE','QUIZ','PRESENTATION','SELF_STUDY','OTHER') NOT NULL DEFAULT 'OTHER';

-- Step 4: Add name_en column to subjects for English translations
-- Add subjects.name_en if missing (runner ignores duplicate column/index)
ALTER TABLE subjects 
  ADD COLUMN name_en VARCHAR(255) NULL AFTER title;
