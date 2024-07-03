-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied.

CREATE TABLE users
(
    id   VARCHAR(255) PRIMARY KEY,
    name VARCHAR(255) NOT NULL
);

CREATE TABLE todos
(
    id      VARCHAR(255) PRIMARY KEY,
    text    VARCHAR(255) NOT NULL,
    done    BOOLEAN      NOT NULL DEFAULT FALSE,
    user_id VARCHAR(255) NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users (id)
);

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back.

DROP TABLE IF EXISTS todos;
DROP TABLE IF EXISTS users;
