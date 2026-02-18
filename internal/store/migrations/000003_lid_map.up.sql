CREATE TABLE IF NOT EXISTS lid_map (
    lid TEXT PRIMARY KEY,
    pn TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_lid_map_pn ON lid_map(pn);
