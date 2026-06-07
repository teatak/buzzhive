package buzzhive

import (
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func (s *Store) AuthTokens() (map[string]AuthToken, error) {
	rows, err := s.query(`
		SELECT k.id, k.user_id, u.username, k.name, k.token, k.valid
		FROM user_api_keys k
		JOIN users u ON u.id = k.user_id
		WHERE k.valid = 1 AND u.valid = 1
		ORDER BY k.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]AuthToken)
	for rows.Next() {
		var user AuthToken
		var valid int
		if err := rows.Scan(&user.ID, &user.UserID, &user.UserName, &user.Name, &user.Token, &valid); err != nil {
			return nil, err
		}
		user.Valid = valid != 0
		out[user.Token] = user
	}
	return out, rows.Err()
}

func (s *Store) Users() ([]AppUser, error) {
	rows, err := s.query(`SELECT id, username, role, valid FROM users ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []AppUser
	for rows.Next() {
		var user AppUser
		var valid int
		if err := rows.Scan(&user.ID, &user.Username, &user.Role, &valid); err != nil {
			return nil, err
		}
		user.Valid = valid != 0
		out = append(out, user)
	}
	return out, rows.Err()
}

func (s *Store) UserAPIKeys() ([]AuthToken, error) {
	rows, err := s.query(`SELECT id, user_id, name, token, valid FROM user_api_keys ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []AuthToken
	for rows.Next() {
		var key AuthToken
		var valid int
		if err := rows.Scan(&key.ID, &key.UserID, &key.Name, &key.Token, &valid); err != nil {
			return nil, err
		}
		key.Valid = valid != 0
		out = append(out, key)
	}
	return out, rows.Err()
}

func (s *Store) UserAPIKey(id, userID int64) (AuthToken, error) {
	var key AuthToken
	var valid int
	err := s.queryRow(`SELECT id, user_id, name, token, valid FROM user_api_keys WHERE id = ? AND user_id = ?`, id, userID).
		Scan(&key.ID, &key.UserID, &key.Name, &key.Token, &valid)
	if err != nil {
		return AuthToken{}, err
	}
	key.Valid = valid != 0
	return key, nil
}

func (s *Store) CreateAppUser(username, password, role string) (AppUser, error) {
	if username == "" || password == "" {
		return AppUser{}, errors.New("username and password are required")
	}
	if role == "" {
		role = "user"
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return AppUser{}, err
	}
	now := storeNow()
	id, err := s.insertReturningID(`INSERT INTO users (username, password_hash, role, valid, created_at, updated_at) VALUES (?, ?, ?, 1, ?, ?)`, username, string(hash), role, now, now)
	if err != nil {
		return AppUser{}, err
	}
	return AppUser{ID: id, Username: username, Role: role, Valid: true}, nil
}

func (s *Store) CreateInitialAdmin(username, password string) (AppUser, error) {
	if required, err := s.SetupRequired(); err != nil {
		return AppUser{}, err
	} else if !required {
		return AppUser{}, errors.New("setup already completed")
	}
	return s.CreateAppUser(username, password, "admin")
}

func (s *Store) CreateUserAPIKey(key AuthToken) (AuthToken, error) {
	if key.UserID == 0 || key.Token == "" {
		return AuthToken{}, errors.New("user_id and token are required")
	}
	if key.Name == "" {
		key.Name = "user-key-" + randomToken(5)
	}
	now := storeNow()
	id, err := s.insertReturningID(`INSERT INTO user_api_keys (user_id, name, token, valid, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`, key.UserID, key.Name, key.Token, boolInt(key.Valid), now, now)
	if err != nil {
		return AuthToken{}, err
	}
	key.ID = id
	return key, nil
}

func (s *Store) SetUserAPIKeyValid(id, userID int64, valid bool) error {
	if id == 0 || userID == 0 {
		return errors.New("id and user_id are required")
	}
	_, err := s.exec(`UPDATE user_api_keys SET valid = ?, updated_at = ? WHERE id = ? AND user_id = ?`, boolInt(valid), storeNow(), id, userID)
	return err
}

func (s *Store) DeleteUserAPIKey(id, userID int64) error {
	if id == 0 || userID == 0 {
		return errors.New("id and user_id are required")
	}
	_, err := s.exec(`DELETE FROM user_api_keys WHERE id = ? AND user_id = ?`, id, userID)
	return err
}

func (s *Store) VerifyPassword(username, password string) (AppUser, error) {
	var user AppUser
	var hash string
	var valid int
	err := s.queryRow(`SELECT id, username, password_hash, role, valid FROM users WHERE username = ?`, username).Scan(&user.ID, &user.Username, &hash, &user.Role, &valid)
	if err != nil {
		return AppUser{}, err
	}
	if valid == 0 {
		return AppUser{}, errors.New("user disabled")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return AppUser{}, err
	}
	user.Valid = true
	return user, nil
}

func (s *Store) ChangePassword(userID int64, currentPassword, nextPassword string) error {
	if userID == 0 || currentPassword == "" || nextPassword == "" {
		return errors.New("current_password and new_password are required")
	}
	var hash string
	if err := s.queryRow(`SELECT password_hash FROM users WHERE id = ? AND valid = 1`, userID).Scan(&hash); err != nil {
		return err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(currentPassword)); err != nil {
		return errors.New("current password is incorrect")
	}
	nextHash, err := bcrypt.GenerateFromPassword([]byte(nextPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = s.exec(`UPDATE users SET password_hash = ?, updated_at = ? WHERE id = ?`, string(nextHash), storeNow(), userID)
	return err
}

func (s *Store) SetupRequired() (bool, error) {
	var count int
	if err := s.queryRow(`SELECT COUNT(1) FROM users`).Scan(&count); err != nil {
		return false, err
	}
	return count == 0, nil
}

func (s *Store) CreateSession(token string, userID int64, expiresAt time.Time) error {
	now := storeNow()
	_, err := s.exec(
		`INSERT INTO sessions (token_hash, user_id, expires_at, created_at) VALUES (?, ?, ?, ?)`,
		sessionHash(token), userID, expiresAt.UTC(), now,
	)
	return err
}

func (s *Store) UserBySession(token string) (SessionUser, error) {
	var user AppUser
	var valid int
	var expiresAt time.Time
	err := s.queryRow(`
		SELECT u.id, u.username, u.role, u.valid, s.expires_at
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = ? AND s.expires_at > ?`,
		sessionHash(token), storeNow(),
	).Scan(&user.ID, &user.Username, &user.Role, &valid, &expiresAt)
	if err != nil {
		return SessionUser{}, err
	}
	if valid == 0 {
		return SessionUser{}, errors.New("user disabled")
	}
	user.Valid = true
	return SessionUser{User: user, ExpiresAt: expiresAt.UTC()}, nil
}

func (s *Store) DeleteSession(token string) error {
	_, err := s.exec(`DELETE FROM sessions WHERE token_hash = ?`, sessionHash(token))
	return err
}

func (s *Store) RenewSession(token string, expiresAt time.Time) error {
	_, err := s.exec(`UPDATE sessions SET expires_at = ? WHERE token_hash = ?`, expiresAt.UTC(), sessionHash(token))
	return err
}

func (s *Store) DeleteExpiredSessions() error {
	_, err := s.exec(`DELETE FROM sessions WHERE expires_at <= ?`, storeNow())
	return err
}
