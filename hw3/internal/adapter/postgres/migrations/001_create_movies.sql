CREATE TABLE movies (
    id BIGINT PRIMARY KEY CHECK (id > 0),
    title TEXT NOT NULL,
    year INTEGER NOT NULL CHECK (year > 0),
    director TEXT NOT NULL
);
