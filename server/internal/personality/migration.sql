-- Create personalities table
CREATE TABLE IF NOT EXISTS personalities (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    system_prompt TEXT NOT NULL,
    is_active BOOLEAN DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Create personality_traits table
CREATE TABLE IF NOT EXISTS personality_traits (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    personality_id TEXT NOT NULL,
    key TEXT NOT NULL,
    value TEXT NOT NULL,
    weight REAL DEFAULT 0.5,
    FOREIGN KEY(personality_id) REFERENCES personalities(id) ON DELETE CASCADE,
    UNIQUE(personality_id, key)
);

-- Create indexes
CREATE INDEX IF NOT EXISTS idx_personalities_name ON personalities(name);
CREATE INDEX IF NOT EXISTS idx_personalities_active ON personalities(is_active);
CREATE INDEX IF NOT EXISTS idx_traits_personality_id ON personality_traits(personality_id);

