-- Google sign-in: the Google account a user is linked to, and the signing
-- PIN Google users enter for electronic signatures.
ALTER TABLE users ADD COLUMN google_sub text;
ALTER TABLE users ADD COLUMN signing_pin_hash text NOT NULL DEFAULT '';
CREATE UNIQUE INDEX users_google_sub ON users (google_sub) WHERE google_sub IS NOT NULL;
