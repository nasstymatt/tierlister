CREATE TABLE IF NOT EXISTS tierlists (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS tiers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    color TEXT NOT NULL DEFAULT '#CCCCCC',
    tierlist_id INTEGER NOT NULL,
    position    REAL NOT NULL,
    FOREIGN KEY (tierlist_id) REFERENCES tierlists(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS tier_images (
    tierlist_id INTEGER NOT NULL,
    tier_id   INTEGER NOT NULL,
    image_id  INTEGER NOT NULL,
    position  REAL NOT NULL,
    PRIMARY KEY (image_id, tierlist_id),
    FOREIGN KEY (tier_id) REFERENCES tiers(id) ON DELETE CASCADE,
    FOREIGN KEY (tierlist_id) REFERENCES tierlists(id) ON DELETE CASCADE,
    FOREIGN KEY (image_id) REFERENCES images(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS images (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    image_path TEXT UNIQUE NOT NULL,
    original_filename TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT (datetime('now'))
);


--Indices----------------------------------------------------------------------
CREATE INDEX IF NOT EXISTS idx_tiers_position       ON tiers(tierlist_id, position);
CREATE INDEX IF NOT EXISTS idx_tier_images_position ON tier_images(tierlist_id, tier_id, position);


--Triggers---------------------------------------------------------------------
CREATE TRIGGER IF NOT EXISTS create_tierlist_defaults
AFTER INSERT ON tierlists
FOR EACH ROW
BEGIN
    INSERT INTO tiers (title, color, tierlist_id, position) VALUES ('S', '#FF7F7F', NEW.id, 1000.0);
    INSERT INTO tiers (title, color, tierlist_id, position) VALUES ('A', '#FFBF7F', NEW.id, 2000.0);
    INSERT INTO tiers (title, color, tierlist_id, position) VALUES ('B', '#FFDF7F', NEW.id, 3000.0);
    INSERT INTO tiers (title, color, tierlist_id, position) VALUES ('C', '#FFFF7F', NEW.id, 4000.0);
    INSERT INTO tiers (title, color, tierlist_id, position) VALUES ('D', '#7FBF7F', NEW.id, 5000.0);
END;
