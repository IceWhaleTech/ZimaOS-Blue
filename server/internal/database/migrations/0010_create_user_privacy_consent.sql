-- Migration: Create user_privacy_consent table
-- Version: 0.10.19
-- Description: Store user privacy consent for online TTS services

CREATE TABLE IF NOT EXISTS user_privacy_consent (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    service TEXT NOT NULL,
    consent_given BOOLEAN DEFAULT FALSE,
    consent_date TIMESTAMP,
    consent_version TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, service)
);

CREATE INDEX IF NOT EXISTS idx_user_privacy_consent_user_id
ON user_privacy_consent(user_id);

CREATE INDEX IF NOT EXISTS idx_user_privacy_consent_service
ON user_privacy_consent(service);

CREATE INDEX IF NOT EXISTS idx_user_privacy_consent_user_service
ON user_privacy_consent(user_id, service);
