-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied.

CREATE TABLE IF NOT EXISTS users
(
    id      BIGSERIAL PRIMARY KEY,
    name    TEXT      NOT NULL,
    created TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_users_id ON users(id);

CREATE TABLE IF NOT EXISTS cardgroups
(
    id      BIGSERIAL PRIMARY KEY,
    name    TEXT      NOT NULL,
    created TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_cardgroups_id ON cardgroups(id);

CREATE TABLE IF NOT EXISTS cards
(
    id            BIGSERIAL PRIMARY KEY,
    front         TEXT      NOT NULL,
    back          TEXT      NOT NULL,
    review_date   TIMESTAMP NOT NULL,
    interval_days INT       NOT NULL DEFAULT 1,
    created       TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated       TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    cardgroup_id  INTEGER   NOT NULL,
    FOREIGN KEY (cardgroup_id) REFERENCES cardgroups (id)
    );
CREATE INDEX idx_cards_id ON cards(id);

CREATE TABLE IF NOT EXISTS cardgroup_users
(
    cardgroup_id BIGINT NOT NULL,
    user_id      BIGINT NOT NULL,
    created TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (cardgroup_id, user_id),
    FOREIGN KEY (cardgroup_id) REFERENCES cardgroups (id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
    );
CREATE INDEX idx_cardgroup_users_cardgroup_id ON cardgroup_users(cardgroup_id);
CREATE INDEX idx_cardgroup_users_user_id ON cardgroup_users(user_id);

CREATE TABLE IF NOT EXISTS roles
(
    id   BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    created TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_roles_id ON roles(id);

CREATE TABLE IF NOT EXISTS user_roles
(
    user_id BIGINT NOT NULL,
    role_id BIGINT NOT NULL,
    created TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, role_id),
    FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    FOREIGN KEY (role_id) REFERENCES roles (id) ON DELETE CASCADE
    );
CREATE INDEX idx_user_roles_user_id ON user_roles(user_id);
CREATE INDEX idx_user_roles_role_id ON user_roles(role_id);

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back.

DROP TABLE IF EXISTS user_roles;
DROP TABLE IF EXISTS roles;
DROP TABLE IF EXISTS cardgroup_users;
DROP TABLE IF EXISTS cards;
DROP TABLE IF EXISTS cardgroups;
DROP TABLE IF EXISTS users;
