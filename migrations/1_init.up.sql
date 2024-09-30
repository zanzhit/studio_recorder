CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash BYTEA NOT NULL
);

CREATE TABLE IF NOT EXISTS admins (
    user_id INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS cameras (
    camera_id TEXT PRIMARY KEY,
    camera_ip TEXT UNIQUE NOT NULL,
    location TEXT NOT NULL,
    has_audio BOOLEAN NOT NULL
);

CREATE TABLE IF NOT EXISTS recordings (
    record_id UUID PRIMARY KEY,
    user_id INTEGER NOT NULL,
    camera_id TEXT NOT NULL,
    start_time TIMESTAMP NOT NULL,
    stop_time TIMESTAMP,
    file_path TEXT,
    is_moved BOOLEAN NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (camera_id) REFERENCES cameras(camera_id) ON DELETE SET NULL
);
