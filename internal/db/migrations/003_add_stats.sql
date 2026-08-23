CREATE TABLE IF NOT EXISTS stats (
    id             INTEGER PRIMARY KEY CHECK (id = 1),
    total_captured INTEGER NOT NULL DEFAULT 0
);

INSERT OR IGNORE INTO stats (id, total_captured) VALUES (1, (SELECT COUNT(*) FROM requests));
