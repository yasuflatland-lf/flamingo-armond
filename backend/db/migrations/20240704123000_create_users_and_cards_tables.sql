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
CREATE INDEX idx_users_name ON users(name);

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
CREATE INDEX idx_cards_front ON cards(front);
CREATE INDEX idx_cards_back ON cards(back);
CREATE INDEX idx_cards_cardgroup_id ON cards(cardgroup_id);
CREATE INDEX idx_cards_interval_days ON cards(interval_days);

CREATE TABLE IF NOT EXISTS cardgroup_users
(
    cardgroup_id BIGINT NOT NULL,
    user_id      BIGINT NOT NULL,
    state   INT   DEFAULT 1,
    created TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (cardgroup_id, user_id),
    FOREIGN KEY (cardgroup_id) REFERENCES cardgroups (id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
    );
CREATE INDEX idx_cardgroup_users_cardgroup_id ON cardgroup_users(cardgroup_id);
CREATE INDEX idx_cardgroup_users_user_id ON cardgroup_users(user_id);
CREATE INDEX idx_cardgroup_users_state ON cardgroup_users(state);

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

CREATE TABLE IF NOT EXISTS swipe_records
(
    id   BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    card_id BIGINT NOT NULL,
    direction TEXT NOT NULL,
    created TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    FOREIGN KEY (card_id) REFERENCES cards (id) ON DELETE CASCADE
);
CREATE INDEX idx_swipe_records_id ON swipe_records(id);
CREATE INDEX idx_swipe_records_user_id ON swipe_records(user_id);
CREATE INDEX idx_swipe_records_card_id ON swipe_records(card_id);

-- +goose statementbegin
-- When the updated column is not changed, assign NULL
CREATE FUNCTION trg_update_timestamp_none() RETURNS trigger AS
    $$
BEGIN
  IF NEW.updated = OLD.updated THEN
    NEW.updated := NULL;
END IF;
RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- When the updated column is NULL, assign the timestamp before executing the UPDATE statement
CREATE FUNCTION trg_update_timestamp_same() RETURNS trigger AS
    $$
BEGIN
  IF NEW.updated IS NULL THEN
    NEW.updated := OLD.updated;
END IF;
RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- When the updated column is NULL, assign the transaction start timestamp
CREATE FUNCTION trg_update_timestamp_current() RETURNS trigger AS
    $$
BEGIN
  IF NEW.updated IS NULL THEN
    NEW.updated := current_timestamp;
END IF;
RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Execute the first function first for `cardgroup_users` table
CREATE TRIGGER update_cardgroup_usersupdated_step1
    BEFORE UPDATE ON cardgroup_users FOR EACH ROW
    EXECUTE PROCEDURE trg_update_timestamp_none();

-- Execute the second function when the `updated` column is updated for `cardgroup_users` table
CREATE TRIGGER update_cardgroup_usersupdated_step2
    BEFORE UPDATE OF updated ON cardgroup_users FOR EACH ROW
    EXECUTE PROCEDURE trg_update_timestamp_same();

-- Finally, execute the third function for `cardgroup_users` table
CREATE TRIGGER update_cardgroup_usersupdated_step3
    BEFORE UPDATE ON cardgroup_users FOR EACH ROW
    EXECUTE PROCEDURE trg_update_timestamp_current();

-- Execute the first function first for `user_roles` table
CREATE TRIGGER update_user_rolesupdated_step1
    BEFORE UPDATE ON user_roles FOR EACH ROW
    EXECUTE PROCEDURE trg_update_timestamp_none();

-- Execute the second function when the `updated` column is updated for `user_roles` table
CREATE TRIGGER update_user_rolesupdated_step2
    BEFORE UPDATE OF updated ON user_roles FOR EACH ROW
    EXECUTE PROCEDURE trg_update_timestamp_same();

-- Finally, execute the third function for `user_roles` table
CREATE TRIGGER update_user_rolesupdated_step3
    BEFORE UPDATE ON user_roles FOR EACH ROW
    EXECUTE PROCEDURE trg_update_timestamp_current();

-- +goose statementend

-- +goose Down

-- Drop triggers for `cardgroup_users` table
DROP TRIGGER IF EXISTS update_cardgroup_usersupdated_step1 ON cardgroup_users;
DROP TRIGGER IF EXISTS update_cardgroup_usersupdated_step2 ON cardgroup_users;
DROP TRIGGER IF EXISTS update_cardgroup_usersupdated_step3 ON cardgroup_users;

-- Drop triggers for `user_roles` table
DROP TRIGGER IF EXISTS update_user_rolesupdated_step1 ON user_roles;
DROP TRIGGER IF EXISTS update_user_rolesupdated_step2 ON user_roles;
DROP TRIGGER IF EXISTS update_user_rolesupdated_step3 ON user_roles;

DROP FUNCTION IF EXISTS trg_update_timestamp_none();
DROP FUNCTION IF EXISTS trg_update_timestamp_same();
DROP FUNCTION IF EXISTS trg_update_timestamp_current();

DROP TABLE IF EXISTS user_roles;
DROP TABLE IF EXISTS roles;
DROP TABLE IF EXISTS cardgroup_users;
DROP TABLE IF EXISTS cards;
DROP TABLE IF EXISTS cardgroups;
DROP TABLE IF EXISTS swipe_records;
DROP TABLE IF EXISTS users;