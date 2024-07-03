-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

CREATE TABLE users (
   id SERIAL PRIMARY KEY,
   name TEXT NOT NULL,
   email TEXT NOT NULL UNIQUE
);

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back

DROP TABLE users;
