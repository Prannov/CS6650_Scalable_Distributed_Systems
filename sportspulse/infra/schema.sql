-- Players
CREATE TABLE IF NOT EXISTS players (
    player_id   TEXT PRIMARY KEY,
    player_name TEXT NOT NULL,
    team_id     TEXT NOT NULL,
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

-- Aggregated stats per player (upserted by stats-worker)
CREATE TABLE IF NOT EXISTS player_stats (
    player_id    TEXT PRIMARY KEY REFERENCES players(player_id),
    points       NUMERIC DEFAULT 0,
    assists      NUMERIC DEFAULT 0,
    rebounds     NUMERIC DEFAULT 0,
    games_played INT     DEFAULT 0,
    updated_at   TIMESTAMPTZ DEFAULT NOW()
);

-- Raw event log (append-only, used for correctness verification)
CREATE TABLE IF NOT EXISTS game_events (
    event_id    TEXT PRIMARY KEY,
    player_id   TEXT NOT NULL,
    team_id     TEXT NOT NULL,
    event_type  TEXT NOT NULL,  -- 'shot', 'assist', 'rebound'
    value       NUMERIC NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    ingested_at TIMESTAMPTZ DEFAULT NOW()
);

-- Seed a few players for testing
INSERT INTO players (player_id, player_name, team_id) VALUES
    ('p1', 'LeBron James',   'lakers'),
    ('p2', 'Stephen Curry',  'warriors'),
    ('p3', 'Kevin Durant',   'suns'),
    ('p4', 'Giannis A.',     'bucks'),
    ('p5', 'Nikola Jokic',   'nuggets')
ON CONFLICT DO NOTHING;

INSERT INTO player_stats (player_id) VALUES
    ('p1'), ('p2'), ('p3'), ('p4'), ('p5')
ON CONFLICT DO NOTHING;