-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied.

CREATE TABLE users
(
    id      BIGSERIAL PRIMARY KEY,
    name    TEXT      NOT NULL,
    created TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE cardgroups
(
    id      BIGSERIAL PRIMARY KEY,
    name    TEXT      NOT NULL,
    created TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE cards
(
    id            BIGSERIAL PRIMARY KEY,
    front         TEXT      NOT NULL,
    back          TEXT      NOT NULL,
    review_date   TIMESTAMP NOT NULL,
    interval_days INT       NOT NULL DEFAULT 1,
    created       TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated       TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    cardgroup_id  INTEGER   NOT NULL,
    FOREIGN KEY (cardgroup_id) REFERENCES cardgroups (id) ON DELETE CASCADE
);

CREATE TABLE cardgroup_users
(
    cardgroup_id BIGINT NOT NULL,
    user_id      BIGINT NOT NULL,
    PRIMARY KEY (cardgroup_id, user_id),
    FOREIGN KEY (cardgroup_id) REFERENCES cardgroups (id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
);

CREATE TABLE roles
(
    id   BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE
);

CREATE TABLE user_roles
(
    user_id BIGINT NOT NULL,
    role_id BIGINT NOT NULL,
    PRIMARY KEY (user_id, role_id),
    FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    FOREIGN KEY (role_id) REFERENCES roles (id) ON DELETE CASCADE
);

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back.

DROP TABLE IF EXISTS user_roles;
DROP TABLE IF EXISTS roles;
DROP TABLE IF EXISTS cardgroup_users;
DROP TABLE IF EXISTS cards;
DROP TABLE IF EXISTS cardgroups;
DROP TABLE IF EXISTS users;
