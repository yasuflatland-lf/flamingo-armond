-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied.

CREATE TABLE users
(
    id      TEXT PRIMARY KEY,
    name    TEXT NOT NULL,
    created TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE cardgroups
(
    id      SERIAL PRIMARY KEY,
    name    TEXT NOT NULL,
    created TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE cards
(
    id            SERIAL PRIMARY KEY,
    front         TEXT         NOT NULL,
    back          TEXT         NOT NULL,
    review_date   TIMESTAMP    NOT NULL,
    interval_days INT          NOT NULL DEFAULT 1,
    created       TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated       TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    cardgroup_id  INTEGER      NOT NULL,
    CONSTRAINT fk_cardgroup
        FOREIGN KEY (cardgroup_id)
            REFERENCES cardgroups (id)
            ON DELETE CASCADE
);

CREATE TABLE cardgroups_users
(
    cardgroup_id INTEGER      NOT NULL,
    user_id      TEXT NOT NULL,
    PRIMARY KEY (cardgroup_id, user_id),
    FOREIGN KEY (cardgroup_id) REFERENCES cardgroups (id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
);

CREATE TABLE roles
(
    id   SERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE
);

CREATE TABLE users_roles
(
    user_id TEXT NOT NULL,
    role_id INTEGER      NOT NULL,
    PRIMARY KEY (user_id, role_id),
    FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    FOREIGN KEY (role_id) REFERENCES roles (id) ON DELETE CASCADE
);

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back.

DROP TABLE IF EXISTS users_roles;
DROP TABLE IF EXISTS roles;
DROP TABLE IF EXISTS card_groups_users;
DROP TABLE IF EXISTS cards;
DROP TABLE IF EXISTS card_groups;
DROP TABLE IF EXISTS users;