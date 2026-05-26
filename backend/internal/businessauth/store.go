package businessauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"imagestudio/internal/config"
	"imagestudio/internal/database"

	"golang.org/x/crypto/bcrypt"
)

const (
	RoleAdmin = "admin"
	RoleUser  = "user"

	StatusActive   = "active"
	StatusDisabled = "disabled"
	StatusDeleted  = "deleted"

	DefaultAdminUserID = "dev_admin"
	DefaultTestUserID  = "dev_user"

	DefaultUIDStart = int64(100001)

	DeletedUserRetention = 7 * 24 * time.Hour

	VerificationPurposeRegistration  = "registration"
	VerificationPurposePasswordReset = "password_reset"
)

var ErrUserAlreadyExists = errors.New("user already exists")
var ErrUserDeleted = errors.New("user is deleted")
var ErrInvalidCurrentPassword = errors.New("current password is invalid")
var ErrLastAdminUser = errors.New("cannot delete the last admin user")
var ErrVerificationTooFrequent = errors.New("verification code requested too frequently")
var ErrVerificationInvalid = errors.New("verification code is invalid")
var ErrVerificationExpired = errors.New("verification code is expired")

type User struct {
	ID           string `json:"id"`
	UID          int64  `json:"uid"`
	Username     string `json:"username"`
	Email        string `json:"email"`
	PasswordHash string `json:"-"`
	Role         string `json:"role"`
	Status       string `json:"status"`
	DeletedAt    string `json:"deleted_at,omitempty"`
	AvatarURL    string `json:"avatarUrl,omitempty"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

type Session struct {
	ID         string `json:"id"`
	UserID     string `json:"user_id"`
	TokenHash  string `json:"-"`
	ExpiresAt  string `json:"expires_at"`
	RevokedAt  string `json:"revoked_at,omitempty"`
	CreatedAt  string `json:"created_at"`
	LastSeenAt string `json:"last_seen_at"`
	User       User   `json:"user"`
}

type ListUsersOptions struct {
	IncludeDeleted bool
}

type PurgeResult struct {
	Users    []User `json:"users"`
	Sessions int64  `json:"sessions"`
}

type BootstrapUser struct {
	ID       string
	Username string
	Email    string
	Password string
	Role     string
}

type CreateUserTxHook func(context.Context, *sql.Tx, User) error

type Store struct {
	db     *sql.DB
	driver string
	ownDB  bool
}

func NewStore(cfg *config.Config) (*Store, error) {
	if !database.IsPostgres(cfg.Database.Driver) {
		return nil, fmt.Errorf("unsupported database driver %q", strings.TrimSpace(cfg.Database.Driver))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := database.Open(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if err := database.Migrate(ctx, db, cfg.Database.Driver); err != nil {
		_ = db.Close()
		return nil, err
	}
	store := NewStoreWithDB(db, cfg.Database.Driver)
	store.ownDB = true
	if err := store.init(); err != nil {
		_ = store.Close()
		return nil, err
	}
	return store, nil
}

func NewStoreWithDB(db *sql.DB, driver string) *Store {
	driver = strings.ToLower(strings.TrimSpace(driver))
	if driver == "" {
		driver = "postgres"
	}
	return &Store{db: db, driver: driver}
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	if !s.ownDB {
		return nil
	}
	return s.db.Close()
}

func (s *Store) init() error {
	if s.isPostgres() {
		return nil
	}
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS business_users (
			id TEXT PRIMARY KEY,
			uid INTEGER NOT NULL DEFAULT 0,
			username TEXT NOT NULL UNIQUE,
			email TEXT NOT NULL DEFAULT '',
			password_hash TEXT NOT NULL,
			role TEXT NOT NULL,
			status TEXT NOT NULL,
			deleted_at TEXT NOT NULL DEFAULT '',
			avatar_url TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS user_sessions (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			token_hash TEXT NOT NULL UNIQUE,
			expires_at TEXT NOT NULL,
			revoked_at TEXT NOT NULL,
			created_at TEXT NOT NULL,
			last_seen_at TEXT NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_user_sessions_user_expires
			ON user_sessions(user_id, expires_at DESC);`,
		`CREATE TABLE IF NOT EXISTS email_verification_codes (
			id TEXT PRIMARY KEY,
			email TEXT NOT NULL,
			purpose TEXT NOT NULL,
			code_hash TEXT NOT NULL,
			attempts INTEGER NOT NULL DEFAULT 0,
			consumed_at TEXT NOT NULL DEFAULT '',
			expires_at TEXT NOT NULL,
			created_at TEXT NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_email_verification_lookup
			ON email_verification_codes(email, purpose, created_at DESC);`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return err
		}
	}
	if err := s.migrateBusinessUsersEmail(); err != nil {
		return err
	}
	if err := s.migrateBusinessUsersDeletedAt(); err != nil {
		return err
	}
	if err := s.migrateBusinessUsersUID(); err != nil {
		return err
	}
	if err := s.migrateBusinessUsersAvatarURL(); err != nil {
		return err
	}
	return nil
}

func (s *Store) migrateBusinessUsersEmail() error {
	if _, err := s.db.Exec(`ALTER TABLE business_users ADD COLUMN email TEXT NOT NULL DEFAULT ''`); err != nil && !isDuplicateColumnError(err) {
		return err
	}
	if _, err := s.db.Exec(
		`UPDATE business_users
		    SET email = lower(CASE
		      WHEN instr(username, '@') > 1 THEN username
		      ELSE username || '@local.invalid'
		    END)
		  WHERE trim(email) = ''`,
	); err != nil {
		return err
	}
	_, err := s.db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_business_users_email ON business_users(email)`)
	return err
}

func (s *Store) migrateBusinessUsersDeletedAt() error {
	if _, err := s.db.Exec(`ALTER TABLE business_users ADD COLUMN deleted_at TEXT NOT NULL DEFAULT ''`); err != nil && !isDuplicateColumnError(err) {
		return err
	}
	_, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_business_users_status_deleted ON business_users(status, deleted_at)`)
	return err
}

func (s *Store) migrateBusinessUsersUID() error {
	if _, err := s.db.Exec(`ALTER TABLE business_users ADD COLUMN uid INTEGER NOT NULL DEFAULT 0`); err != nil && !isDuplicateColumnError(err) {
		return err
	}
	rows, err := s.db.Query(`SELECT id FROM business_users WHERE uid = 0 ORDER BY created_at ASC, username ASC, id ASC`)
	if err != nil {
		return err
	}
	defer rows.Close()

	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, id := range ids {
		uid, err := s.nextUIDInTx(context.Background(), tx)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(s.rebind(`UPDATE business_users SET uid = ? WHERE id = ? AND uid = 0`), uid, id); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if _, err := s.db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_business_users_uid ON business_users(uid)`); err != nil {
		return err
	}
	_, err = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_business_users_created_uid ON business_users(created_at, uid)`)
	return err
}

func (s *Store) migrateBusinessUsersAvatarURL() error {
	if _, err := s.db.Exec(`ALTER TABLE business_users ADD COLUMN avatar_url TEXT NOT NULL DEFAULT ''`); err != nil && !isDuplicateColumnError(err) {
		return err
	}
	return nil
}

func (s *Store) EnsureBootstrapUsers(ctx context.Context, users []BootstrapUser) error {
	for _, user := range users {
		if err := s.EnsureBootstrapUser(ctx, user); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) EnsureBootstrapUser(ctx context.Context, user BootstrapUser) error {
	user.ID = cleanID(user.ID)
	user.Username = normalizeUsername(user.Username)
	user.Email = normalizeEmail(firstNonEmpty(user.Email, emailFromLegacyUsername(user.Username)))
	if user.Username == "" {
		user.Username = usernameFromEmail(user.Email)
	}
	user.Role = normalizeRole(user.Role)
	user.Password = strings.TrimSpace(user.Password)
	if user.ID == "" {
		return fmt.Errorf("user id is required")
	}
	if user.Username == "" {
		return fmt.Errorf("username is required")
	}
	if user.Email == "" {
		return fmt.Errorf("email is required")
	}
	if user.Password == "" {
		return fmt.Errorf("password is required")
	}
	if user.Role == "" {
		return fmt.Errorf("role is required")
	}

	if _, ok, err := s.GetUserByEmail(ctx, user.Email); err != nil {
		return err
	} else if ok {
		return nil
	}

	existing, ok, err := s.GetUserByUsername(ctx, user.Username)
	if err != nil {
		return err
	}
	if ok {
		if existing.Email == "" || strings.HasSuffix(existing.Email, "@local.invalid") {
			_, err := s.db.ExecContext(
				ctx,
				s.rebind(`UPDATE business_users SET email = ?, updated_at = ? WHERE id = ?`),
				user.Email,
				s.dbTime(time.Now().UTC()),
				existing.ID,
			)
			return err
		}
		return nil
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	existing, ok, err = s.GetUserByID(ctx, user.ID)
	if err != nil {
		return err
	}
	if ok {
		_, err := s.db.ExecContext(
			ctx,
			s.rebind(`UPDATE business_users SET username = ?, email = ?, updated_at = ? WHERE id = ?`),
			user.Username,
			user.Email,
			s.dbTimeText(now),
			user.ID,
		)
		return err
	}

	passwordHash, err := HashPassword(user.Password)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	uid, err := s.nextUIDInTx(ctx, tx)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(
		ctx,
		s.rebind(`INSERT INTO business_users(id, uid, username, email, password_hash, role, status, deleted_at, created_at, updated_at)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`),
		user.ID,
		uid,
		user.Username,
		user.Email,
		passwordHash,
		user.Role,
		StatusActive,
		s.emptyTime(),
		s.dbTimeText(now),
		s.dbTimeText(now),
	)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) Authenticate(ctx context.Context, credential, password string) (User, bool, error) {
	credential = strings.TrimSpace(credential)
	password = strings.TrimSpace(password)
	if credential == "" || password == "" {
		return User{}, false, nil
	}
	user, ok, err := s.GetUserByEmail(ctx, credential)
	if err != nil {
		return User{}, false, err
	}
	if !ok {
		user, ok, err = s.GetUserByUsername(ctx, credential)
	}
	if err != nil || !ok {
		return User{}, false, err
	}
	if user.Status != StatusActive {
		return User{}, false, nil
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return User{}, false, nil
	}
	return user, true, nil
}

func (s *Store) GetUserByEmail(ctx context.Context, email string) (User, bool, error) {
	email = normalizeEmail(email)
	if email == "" {
		return User{}, false, nil
	}
	var user User
	err := s.db.QueryRowContext(
		ctx,
		s.rebind(`SELECT id, uid, username, email, password_hash, role, status, deleted_at, avatar_url, created_at, updated_at
		 FROM business_users
		 WHERE email = ?`),
		email,
	).Scan(
		s.userScanDest(&user)...,
	)
	if err == sql.ErrNoRows {
		return User{}, false, nil
	}
	if err != nil {
		return User{}, false, err
	}
	user.Role = normalizeRole(user.Role)
	user.Status = normalizeStatus(user.Status)
	return user, true, nil
}

func (s *Store) GetUserByUsername(ctx context.Context, username string) (User, bool, error) {
	username = normalizeUsername(username)
	if username == "" {
		return User{}, false, nil
	}
	var user User
	err := s.db.QueryRowContext(
		ctx,
		s.rebind(`SELECT id, uid, username, email, password_hash, role, status, deleted_at, avatar_url, created_at, updated_at
		 FROM business_users
		 WHERE username = ?`),
		username,
	).Scan(
		s.userScanDest(&user)...,
	)
	if err == sql.ErrNoRows {
		return User{}, false, nil
	}
	if err != nil {
		return User{}, false, err
	}
	user.Role = normalizeRole(user.Role)
	user.Status = normalizeStatus(user.Status)
	return user, true, nil
}

func (s *Store) ListUsers(ctx context.Context, options ...ListUsersOptions) ([]User, error) {
	includeDeleted := false
	if len(options) > 0 {
		includeDeleted = options[0].IncludeDeleted
	}
	query := `SELECT id, uid, username, email, password_hash, role, status, deleted_at, avatar_url, created_at, updated_at
		 FROM business_users`
	args := []any{}
	if !includeDeleted {
		query += ` WHERE status <> ?`
		args = append(args, StatusDeleted)
	}
	query += ` ORDER BY created_at ASC, username ASC`
	query = s.rebind(query)

	rows, err := s.db.QueryContext(
		ctx,
		query,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := make([]User, 0)
	for rows.Next() {
		var user User
		if err := rows.Scan(
			s.userScanDest(&user)...,
		); err != nil {
			return nil, err
		}
		user.Role = normalizeRole(user.Role)
		user.Status = normalizeStatus(user.Status)
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return users, nil
}

func (s *Store) CreateUser(ctx context.Context, email, password, role string) (User, error) {
	return s.CreateUserWithUsername(ctx, email, "", password, role)
}

func (s *Store) CreateEmailVerificationCode(ctx context.Context, email, purpose, code string, expiresAt time.Time, cooldown time.Duration) error {
	email = normalizeEmail(email)
	purpose = normalizeVerificationPurpose(purpose)
	code = strings.TrimSpace(code)
	if email == "" {
		return fmt.Errorf("email is required")
	}
	if purpose == "" {
		return fmt.Errorf("verification purpose is required")
	}
	if code == "" {
		return fmt.Errorf("verification code is required")
	}
	now := time.Now().UTC()
	if !expiresAt.After(now) {
		return fmt.Errorf("verification code expiry must be in the future")
	}
	if cooldown > 0 {
		var createdAtRaw string
		err := s.db.QueryRowContext(
			ctx,
			s.rebind(`SELECT created_at
			   FROM email_verification_codes
			  WHERE email = ? AND purpose = ? AND consumed_at IS NULL
			  ORDER BY created_at DESC
			  LIMIT 1`),
			email,
			purpose,
		).Scan(s.timeScanDest(&createdAtRaw))
		if err != nil && err != sql.ErrNoRows {
			return err
		}
		if createdAtRaw != "" {
			createdAt, err := time.Parse(time.RFC3339Nano, createdAtRaw)
			if err == nil && now.Sub(createdAt) < cooldown {
				return ErrVerificationTooFrequent
			}
		}
	}
	_, err := s.db.ExecContext(
		ctx,
		s.rebind(`INSERT INTO email_verification_codes(id, email, purpose, code_hash, attempts, consumed_at, expires_at, created_at)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?)`),
		newVerificationID(),
		email,
		purpose,
		emailVerificationCodeHash(email, purpose, code),
		0,
		s.emptyTime(),
		s.dbTime(expiresAt.UTC()),
		s.dbTime(now),
	)
	return err
}

func (s *Store) ConsumeEmailVerificationCode(ctx context.Context, email, purpose, code string, maxAttempts int) error {
	email = normalizeEmail(email)
	purpose = normalizeVerificationPurpose(purpose)
	code = strings.TrimSpace(code)
	if email == "" || purpose == "" || code == "" {
		return ErrVerificationInvalid
	}
	if maxAttempts <= 0 {
		maxAttempts = 5
	}

	var id string
	var codeHash string
	var attempts int
	var expiresAtRaw string
	err := s.db.QueryRowContext(
		ctx,
		s.rebind(`SELECT id, code_hash, attempts, expires_at
		   FROM email_verification_codes
		  WHERE email = ? AND purpose = ? AND consumed_at IS NULL
		  ORDER BY created_at DESC
		  LIMIT 1`),
		email,
		purpose,
	).Scan(&id, &codeHash, &attempts, s.timeScanDest(&expiresAtRaw))
	if err == sql.ErrNoRows {
		return ErrVerificationInvalid
	}
	if err != nil {
		return err
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, expiresAtRaw)
	if err != nil || !time.Now().UTC().Before(expiresAt) {
		_, _ = s.db.ExecContext(ctx, s.rebind(`UPDATE email_verification_codes SET consumed_at = ? WHERE id = ?`), s.dbTime(time.Now().UTC()), id)
		return ErrVerificationExpired
	}
	if attempts >= maxAttempts {
		return ErrVerificationInvalid
	}
	expected := emailVerificationCodeHash(email, purpose, code)
	if subtle.ConstantTimeCompare([]byte(expected), []byte(codeHash)) != 1 {
		_, _ = s.db.ExecContext(ctx, s.rebind(`UPDATE email_verification_codes SET attempts = attempts + 1 WHERE id = ?`), id)
		return ErrVerificationInvalid
	}
	_, err = s.db.ExecContext(
		ctx,
		s.rebind(`UPDATE email_verification_codes
		    SET consumed_at = ?
		  WHERE id = ? AND consumed_at IS NULL`),
		s.dbTime(time.Now().UTC()),
		id,
	)
	return err
}

func (s *Store) CreateUserWithUsername(ctx context.Context, email, username, password, role string) (User, error) {
	return s.CreateUserWithUsernameTx(ctx, email, username, password, role, nil)
}

func (s *Store) CreateUserWithUsernameTx(ctx context.Context, email, username, password, role string, hook CreateUserTxHook) (User, error) {
	rawEmail := strings.TrimSpace(email)
	email = normalizeEmail(rawEmail)
	if email == "" {
		email = emailFromLegacyUsername(rawEmail)
	}
	username = normalizeUsername(username)
	password = strings.TrimSpace(password)
	role = normalizeRole(role)
	if role == "" {
		role = RoleUser
	}
	if email == "" {
		return User{}, fmt.Errorf("email is required")
	}
	if password == "" {
		return User{}, fmt.Errorf("password is required")
	}

	if existing, ok, err := s.GetUserByEmail(ctx, email); err != nil {
		return User{}, err
	} else if ok {
		if existing.Status == StatusDeleted {
			return User{}, ErrUserDeleted
		}
		return User{}, ErrUserAlreadyExists
	}
	if username != "" {
		if existing, ok, err := s.GetUserByUsername(ctx, username); err != nil {
			return User{}, err
		} else if ok {
			if existing.Status == StatusDeleted {
				return User{}, ErrUserDeleted
			}
			return User{}, ErrUserAlreadyExists
		}
	} else {
		generatedUsername, err := s.uniqueUsernameForEmail(ctx, email)
		if err != nil {
			return User{}, err
		}
		username = generatedUsername
	}

	passwordHash, err := HashPassword(password)
	if err != nil {
		return User{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback()
	uid, err := s.nextUIDInTx(ctx, tx)
	if err != nil {
		return User{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	user := User{
		ID:           newUserID(),
		UID:          uid,
		Username:     username,
		Email:        email,
		PasswordHash: passwordHash,
		Role:         role,
		Status:       StatusActive,
		DeletedAt:    "",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	_, err = tx.ExecContext(
		ctx,
		s.rebind(`INSERT INTO business_users(id, uid, username, email, password_hash, role, status, deleted_at, created_at, updated_at)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`),
		user.ID,
		user.UID,
		user.Username,
		user.Email,
		user.PasswordHash,
		user.Role,
		user.Status,
		s.emptyTime(),
		s.dbTimeText(user.CreatedAt),
		s.dbTimeText(user.UpdatedAt),
	)
	if err != nil {
		return User{}, err
	}
	if hook != nil {
		if err := hook(ctx, tx, user); err != nil {
			return User{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return User{}, err
	}
	return user, nil
}

func (s *Store) SetUserStatus(ctx context.Context, id, status string) (User, bool, error) {
	id = cleanID(id)
	status = strings.ToLower(strings.TrimSpace(status))
	if id == "" {
		return User{}, false, nil
	}
	if status != StatusActive && status != StatusDisabled {
		return User{}, false, fmt.Errorf("status must be active or disabled")
	}
	user, ok, err := s.GetUserByID(ctx, id)
	if err != nil || !ok {
		return User{}, ok, err
	}
	if user.Status == StatusDeleted {
		return user, true, ErrUserDeleted
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := s.db.ExecContext(
		ctx,
		s.rebind(`UPDATE business_users
		 SET status = ?, updated_at = ?
		 WHERE id = ?`),
		status,
		s.dbTimeText(now),
		id,
	)
	if err != nil {
		return User{}, false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return User{}, false, err
	}
	if affected == 0 {
		return User{}, false, nil
	}
	if status == StatusDisabled {
		if err := s.RevokeSessionsByUserID(ctx, id); err != nil {
			return User{}, false, err
		}
	}
	user, ok, err = s.GetUserByID(ctx, id)
	return user, ok, err
}

func (s *Store) ResetUserPassword(ctx context.Context, id, password string) (User, bool, error) {
	id = cleanID(id)
	password = strings.TrimSpace(password)
	if id == "" {
		return User{}, false, nil
	}
	if password == "" {
		return User{}, false, fmt.Errorf("password is required")
	}
	user, ok, err := s.GetUserByID(ctx, id)
	if err != nil || !ok {
		return User{}, ok, err
	}
	if user.Status == StatusDeleted {
		return user, true, ErrUserDeleted
	}
	passwordHash, err := HashPassword(password)
	if err != nil {
		return User{}, false, err
	}
	result, err := s.db.ExecContext(
		ctx,
		s.rebind(`UPDATE business_users
		 SET password_hash = ?, updated_at = ?
		 WHERE id = ?`),
		passwordHash,
		s.dbTime(time.Now().UTC()),
		id,
	)
	if err != nil {
		return User{}, false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return User{}, false, err
	}
	if affected == 0 {
		return User{}, false, nil
	}
	if err := s.RevokeSessionsByUserID(ctx, id); err != nil {
		return User{}, false, err
	}
	user, ok, err = s.GetUserByID(ctx, id)
	return user, ok, err
}

func (s *Store) UpdateUser(ctx context.Context, id, username, password string) (User, bool, error) {
	id = cleanID(id)
	username = normalizeUsername(username)
	password = strings.TrimSpace(password)
	if id == "" {
		return User{}, false, nil
	}
	if username == "" {
		return User{}, false, fmt.Errorf("username is required")
	}
	user, ok, err := s.GetUserByID(ctx, id)
	if err != nil || !ok {
		return User{}, ok, err
	}
	if user.Status == StatusDeleted {
		return user, true, ErrUserDeleted
	}
	var existingID string
	err = s.db.QueryRowContext(
		ctx,
		s.rebind(`SELECT id FROM business_users WHERE username = ? AND id <> ? LIMIT 1`),
		username,
		id,
	).Scan(&existingID)
	if err == nil {
		return User{}, false, ErrUserAlreadyExists
	}
	if err != sql.ErrNoRows {
		return User{}, false, err
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	if password == "" {
		result, err := s.db.ExecContext(
			ctx,
			s.rebind(`UPDATE business_users
			 SET username = ?, updated_at = ?
			 WHERE id = ?`),
			username,
			s.dbTimeText(now),
			id,
		)
		if err != nil {
			return User{}, false, err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return User{}, false, err
		}
		if affected == 0 {
			return User{}, false, nil
		}
		user, ok, err = s.GetUserByID(ctx, id)
		return user, ok, err
	}

	passwordHash, err := HashPassword(password)
	if err != nil {
		return User{}, false, err
	}
	result, err := s.db.ExecContext(
		ctx,
		s.rebind(`UPDATE business_users
		 SET username = ?, password_hash = ?, updated_at = ?
		 WHERE id = ?`),
		username,
		passwordHash,
		s.dbTimeText(now),
		id,
	)
	if err != nil {
		return User{}, false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return User{}, false, err
	}
	if affected == 0 {
		return User{}, false, nil
	}
	if err := s.RevokeSessionsByUserID(ctx, id); err != nil {
		return User{}, false, err
	}
	user, ok, err = s.GetUserByID(ctx, id)
	return user, ok, err
}

func (s *Store) ChangeUserPassword(ctx context.Context, id, currentPassword, newPassword string) (User, bool, error) {
	id = cleanID(id)
	currentPassword = strings.TrimSpace(currentPassword)
	newPassword = strings.TrimSpace(newPassword)
	if id == "" {
		return User{}, false, nil
	}
	if currentPassword == "" {
		return User{}, false, ErrInvalidCurrentPassword
	}
	if newPassword == "" {
		return User{}, false, fmt.Errorf("password is required")
	}
	user, ok, err := s.GetUserByID(ctx, id)
	if err != nil || !ok {
		return User{}, ok, err
	}
	if user.Status == StatusDeleted {
		return user, true, ErrUserDeleted
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(currentPassword)); err != nil {
		return User{}, true, ErrInvalidCurrentPassword
	}
	passwordHash, err := HashPassword(newPassword)
	if err != nil {
		return User{}, false, err
	}
	_, err = s.db.ExecContext(
		ctx,
		s.rebind(`UPDATE business_users
		 SET password_hash = ?, updated_at = ?
		 WHERE id = ?`),
		passwordHash,
		s.dbTime(time.Now().UTC()),
		id,
	)
	if err != nil {
		return User{}, false, err
	}
	user, ok, err = s.GetUserByID(ctx, id)
	return user, ok, err
}

func (s *Store) GetUserByID(ctx context.Context, id string) (User, bool, error) {
	id = cleanID(id)
	if id == "" {
		return User{}, false, nil
	}
	var user User
	err := s.db.QueryRowContext(
		ctx,
		s.rebind(`SELECT id, uid, username, email, password_hash, role, status, deleted_at, avatar_url, created_at, updated_at
		 FROM business_users
		 WHERE id = ?`),
		id,
	).Scan(
		s.userScanDest(&user)...,
	)
	if err == sql.ErrNoRows {
		return User{}, false, nil
	}
	if err != nil {
		return User{}, false, err
	}
	user.Role = normalizeRole(user.Role)
	user.Status = normalizeStatus(user.Status)
	return user, true, nil
}

func (s *Store) UpdateUserAvatar(ctx context.Context, id, avatarURL string) (User, bool, error) {
	id = cleanID(id)
	avatarURL = strings.TrimSpace(avatarURL)
	if id == "" {
		return User{}, false, nil
	}
	user, ok, err := s.GetUserByID(ctx, id)
	if err != nil || !ok {
		return User{}, ok, err
	}
	if user.Status == StatusDeleted {
		return user, true, ErrUserDeleted
	}
	result, err := s.db.ExecContext(
		ctx,
		s.rebind(`UPDATE business_users
		 SET avatar_url = ?, updated_at = ?
		 WHERE id = ?`),
		avatarURL,
		s.dbTime(time.Now().UTC()),
		id,
	)
	if err != nil {
		return User{}, false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return User{}, false, err
	}
	if affected == 0 {
		return User{}, false, nil
	}
	user, ok, err = s.GetUserByID(ctx, id)
	return user, ok, err
}

func (s *Store) DeleteUser(ctx context.Context, id string) (User, bool, error) {
	id = cleanID(id)
	if id == "" {
		return User{}, false, nil
	}
	user, ok, err := s.GetUserByID(ctx, id)
	if err != nil || !ok {
		return User{}, ok, err
	}
	if user.Status == StatusDeleted {
		return user, true, nil
	}
	if user.Role == RoleAdmin {
		var adminCount int
		if err := s.db.QueryRowContext(
			ctx,
			s.rebind(`SELECT COUNT(*) FROM business_users WHERE role = ? AND status <> ?`),
			RoleAdmin,
			StatusDeleted,
		).Scan(&adminCount); err != nil {
			return User{}, false, err
		}
		if adminCount <= 1 {
			return User{}, false, ErrLastAdminUser
		}
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := s.db.ExecContext(
		ctx,
		s.rebind(`UPDATE business_users
		 SET status = ?, deleted_at = ?, updated_at = ?
		 WHERE id = ?`),
		StatusDeleted,
		s.dbTimeText(now),
		s.dbTimeText(now),
		id,
	)
	if err != nil {
		return User{}, false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return User{}, false, err
	}
	if affected == 0 {
		return User{}, false, nil
	}
	if err := s.RevokeSessionsByUserID(ctx, id); err != nil {
		return User{}, false, err
	}
	user, ok, err = s.GetUserByID(ctx, id)
	return user, ok, err
}

func (s *Store) RestoreUser(ctx context.Context, id string) (User, bool, error) {
	id = cleanID(id)
	if id == "" {
		return User{}, false, nil
	}
	user, ok, err := s.GetUserByID(ctx, id)
	if err != nil || !ok {
		return User{}, ok, err
	}
	if user.Status != StatusDeleted {
		return user, true, nil
	}
	result, err := s.db.ExecContext(
		ctx,
		s.rebind(`UPDATE business_users
		 SET status = ?, deleted_at = ?, updated_at = ?
		 WHERE id = ?`),
		StatusActive,
		s.emptyTime(),
		s.dbTime(time.Now().UTC()),
		id,
	)
	if err != nil {
		return User{}, false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return User{}, false, err
	}
	if affected == 0 {
		return User{}, false, nil
	}
	user, ok, err = s.GetUserByID(ctx, id)
	return user, ok, err
}

func (s *Store) DeletedUsersBefore(ctx context.Context, cutoff time.Time) ([]User, error) {
	rows, err := s.db.QueryContext(
		ctx,
		s.rebind(`SELECT id, uid, username, email, password_hash, role, status, deleted_at, avatar_url, created_at, updated_at
		 FROM business_users
		 WHERE status = ? AND deleted_at IS NOT NULL AND deleted_at <= ?
		 ORDER BY deleted_at ASC`),
		StatusDeleted,
		s.dbTime(cutoff.UTC()),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := make([]User, 0)
	for rows.Next() {
		var user User
		if err := rows.Scan(
			s.userScanDest(&user)...,
		); err != nil {
			return nil, err
		}
		user.Role = normalizeRole(user.Role)
		user.Status = normalizeStatus(user.Status)
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return users, nil
}

func (s *Store) PurgeDeletedUser(ctx context.Context, id string) (User, bool, error) {
	id = cleanID(id)
	if id == "" {
		return User{}, false, nil
	}
	user, ok, err := s.GetUserByID(ctx, id)
	if err != nil || !ok {
		return User{}, ok, err
	}
	if user.Status != StatusDeleted {
		return user, false, fmt.Errorf("user is not deleted")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, false, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, s.rebind(`DELETE FROM user_sessions WHERE user_id = ?`), id); err != nil {
		return User{}, false, err
	}
	result, err := tx.ExecContext(ctx, s.rebind(`DELETE FROM business_users WHERE id = ? AND status = ?`), id, StatusDeleted)
	if err != nil {
		return User{}, false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return User{}, false, err
	}
	if affected == 0 {
		return User{}, false, nil
	}
	if err := tx.Commit(); err != nil {
		return User{}, false, err
	}
	return user, true, nil
}

func (s *Store) CreateSession(ctx context.Context, id, token string, user User, expiresAt time.Time) (Session, error) {
	id = cleanID(id)
	tokenHash := TokenHash(token)
	if id == "" {
		return Session{}, fmt.Errorf("session id is required")
	}
	if tokenHash == "" {
		return Session{}, fmt.Errorf("session token is required")
	}
	if strings.TrimSpace(user.ID) == "" {
		return Session{}, fmt.Errorf("session user id is required")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	session := Session{
		ID:         id,
		UserID:     user.ID,
		TokenHash:  tokenHash,
		ExpiresAt:  expiresAt.UTC().Format(time.RFC3339Nano),
		CreatedAt:  now,
		LastSeenAt: now,
		User:       user,
	}
	_, err := s.db.ExecContext(
		ctx,
		s.rebind(`INSERT INTO user_sessions(id, user_id, token_hash, expires_at, revoked_at, created_at, last_seen_at)
		 VALUES(?, ?, ?, ?, ?, ?, ?)`),
		session.ID,
		session.UserID,
		session.TokenHash,
		s.dbTimeText(session.ExpiresAt),
		s.emptyTime(),
		s.dbTimeText(session.CreatedAt),
		s.dbTimeText(session.LastSeenAt),
	)
	return session, err
}

func (s *Store) GetSessionByToken(ctx context.Context, token string) (Session, bool, error) {
	tokenHash := TokenHash(token)
	if tokenHash == "" {
		return Session{}, false, nil
	}
	var session Session
	var user User
	dest := []any{
		&session.ID,
		&session.UserID,
		&session.TokenHash,
		s.timeScanDest(&session.ExpiresAt),
		s.timeScanDest(&session.RevokedAt),
		s.timeScanDest(&session.CreatedAt),
		s.timeScanDest(&session.LastSeenAt),
	}
	dest = append(dest, s.userScanDest(&user)...)
	err := s.db.QueryRowContext(
		ctx,
		s.rebind(`SELECT
			s.id, s.user_id, s.token_hash, s.expires_at, s.revoked_at, s.created_at, s.last_seen_at,
			u.id, u.uid, u.username, u.email, u.password_hash, u.role, u.status, u.deleted_at, u.avatar_url, u.created_at, u.updated_at
		 FROM user_sessions s
		 JOIN business_users u ON u.id = s.user_id
		 WHERE s.token_hash = ?`),
		tokenHash,
	).Scan(dest...)
	if err == sql.ErrNoRows {
		return Session{}, false, nil
	}
	if err != nil {
		return Session{}, false, err
	}
	user.Role = normalizeRole(user.Role)
	user.Status = normalizeStatus(user.Status)
	session.User = user
	if session.RevokedAt != "" || user.Status != StatusActive {
		return Session{}, false, nil
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, session.ExpiresAt)
	if err != nil {
		return Session{}, false, err
	}
	if !time.Now().Before(expiresAt) {
		return Session{}, false, nil
	}
	_, _ = s.db.ExecContext(
		ctx,
		s.rebind(`UPDATE user_sessions SET last_seen_at = ? WHERE id = ?`),
		s.dbTime(time.Now().UTC()),
		session.ID,
	)
	return session, true, nil
}

func (s *Store) RevokeSessionByToken(ctx context.Context, token string) error {
	tokenHash := TokenHash(token)
	if tokenHash == "" {
		return nil
	}
	_, err := s.db.ExecContext(
		ctx,
		s.rebind(`UPDATE user_sessions
		 SET revoked_at = ?
		 WHERE token_hash = ? AND revoked_at IS NULL`),
		s.dbTime(time.Now().UTC()),
		tokenHash,
	)
	return err
}

func (s *Store) RevokeSessionsByUserID(ctx context.Context, userID string) error {
	userID = cleanID(userID)
	if userID == "" {
		return nil
	}
	_, err := s.db.ExecContext(
		ctx,
		s.rebind(`UPDATE user_sessions
		 SET revoked_at = ?
		 WHERE user_id = ? AND revoked_at IS NULL`),
		s.dbTime(time.Now().UTC()),
		userID,
	)
	return err
}

func HashPassword(password string) (string, error) {
	password = strings.TrimSpace(password)
	if password == "" {
		return "", fmt.Errorf("password is required")
	}
	raw, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func TokenHash(token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func emailVerificationCodeHash(email, purpose, code string) string {
	return TokenHash(normalizeEmail(email) + "|" + normalizeVerificationPurpose(purpose) + "|" + strings.TrimSpace(code))
}

func UserIDForRole(role string) string {
	if normalizeRole(role) == RoleAdmin {
		return DefaultAdminUserID
	}
	return DefaultTestUserID
}

func normalizeUsername(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}

func normalizeEmail(email string) string {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || strings.ContainsAny(email, " \t\r\n") || strings.Count(email, "@") != 1 {
		return ""
	}
	parts := strings.Split(email, "@")
	if parts[0] == "" || parts[1] == "" {
		return ""
	}
	return email
}

func emailFromLegacyUsername(username string) string {
	username = normalizeUsername(username)
	if username == "" {
		return ""
	}
	if email := normalizeEmail(username); email != "" {
		return email
	}
	return username + "@local.invalid"
}

func usernameFromEmail(email string) string {
	email = normalizeEmail(email)
	if email == "" {
		return ""
	}
	local := strings.Split(email, "@")[0]
	var builder strings.Builder
	for _, r := range local {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '.', r == '_', r == '-':
			builder.WriteRune(r)
		}
	}
	username := strings.Trim(builder.String(), "._-")
	if username == "" {
		return "user"
	}
	return username
}

func (s *Store) uniqueUsernameForEmail(ctx context.Context, email string) (string, error) {
	base := usernameFromEmail(email)
	if base == "" {
		return "", fmt.Errorf("email is required")
	}
	for index := 0; index < 1000; index++ {
		candidate := base
		if index > 0 {
			candidate = fmt.Sprintf("%s%d", base, index+1)
		}
		if _, ok, err := s.GetUserByUsername(ctx, candidate); err != nil {
			return "", err
		} else if !ok {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("username is unavailable")
}

func (s *Store) nextUID(ctx context.Context) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	uid, err := s.nextUIDInTx(ctx, tx)
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return uid, nil
}

func (s *Store) nextUIDInTx(ctx context.Context, tx *sql.Tx) (int64, error) {
	if s.isPostgres() {
		var uid int64
		if err := tx.QueryRowContext(ctx, `SELECT nextval('business_user_uid_seq')`).Scan(&uid); err != nil {
			return 0, err
		}
		return uid, nil
	}
	var current sql.NullInt64
	if err := tx.QueryRowContext(ctx, `SELECT MAX(uid) FROM business_users`).Scan(&current); err != nil {
		return 0, err
	}
	if !current.Valid || current.Int64 < DefaultUIDStart {
		return DefaultUIDStart, nil
	}
	return current.Int64 + 1, nil
}

func (s *Store) isPostgres() bool {
	return strings.EqualFold(strings.TrimSpace(s.driver), "postgres")
}

func (s *Store) rebind(query string) string {
	if !s.isPostgres() {
		query = strings.ReplaceAll(query, "consumed_at IS NULL", "consumed_at = ''")
		query = strings.ReplaceAll(query, "revoked_at IS NULL", "revoked_at = ''")
		query = strings.ReplaceAll(query, "deleted_at IS NOT NULL", "deleted_at <> ''")
		return query
	}
	var builder strings.Builder
	index := 1
	for _, r := range query {
		if r == '?' {
			builder.WriteString(fmt.Sprintf("$%d", index))
			index++
			continue
		}
		builder.WriteRune(r)
	}
	return builder.String()
}

func (s *Store) emptyTime() any {
	if s.isPostgres() {
		return nil
	}
	return ""
}

func (s *Store) dbTime(value time.Time) any {
	if s.isPostgres() {
		return value.UTC()
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func (s *Store) dbTimeText(value string) any {
	parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(value))
	if err != nil {
		return strings.TrimSpace(value)
	}
	return s.dbTime(parsed)
}

func (s *Store) timeScanDest(target *string) any {
	if s.isPostgres() {
		return &nullableTimeString{target: target}
	}
	return target
}

func (s *Store) userScanDest(user *User) []any {
	if s.isPostgres() {
		return []any{
			&user.ID,
			&user.UID,
			&user.Username,
			&user.Email,
			&user.PasswordHash,
			&user.Role,
			&user.Status,
			nullableStringDest(&user.DeletedAt),
			&user.AvatarURL,
			&timeString{target: &user.CreatedAt},
			&timeString{target: &user.UpdatedAt},
		}
	}
	return []any{
		&user.ID,
		&user.UID,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&user.Role,
		&user.Status,
		&user.DeletedAt,
		&user.AvatarURL,
		&user.CreatedAt,
		&user.UpdatedAt,
	}
}

func nullableStringDest(target *string) any {
	return &nullableTimeString{target: target}
}

type timeString struct {
	target *string
}

func (s *timeString) Scan(value any) error {
	text, err := scanTimeText(value)
	if err != nil {
		return err
	}
	*s.target = text
	return nil
}

type nullableTimeString struct {
	target *string
}

func (s *nullableTimeString) Scan(value any) error {
	if value == nil {
		*s.target = ""
		return nil
	}
	text, err := scanTimeText(value)
	if err != nil {
		return err
	}
	*s.target = text
	return nil
}

func scanTimeText(value any) (string, error) {
	switch typed := value.(type) {
	case time.Time:
		return typed.UTC().Format(time.RFC3339Nano), nil
	case string:
		return typed, nil
	case []byte:
		return string(typed), nil
	default:
		return "", fmt.Errorf("unsupported time scan type %T", value)
	}
}

func isDuplicateColumnError(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "duplicate column")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func normalizeRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case RoleAdmin:
		return RoleAdmin
	case RoleUser:
		return RoleUser
	default:
		return ""
	}
}

func normalizeVerificationPurpose(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case VerificationPurposeRegistration:
		return VerificationPurposeRegistration
	case VerificationPurposePasswordReset:
		return VerificationPurposePasswordReset
	default:
		return ""
	}
}

func normalizeStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case StatusDisabled:
		return StatusDisabled
	case StatusDeleted:
		return StatusDeleted
	default:
		return StatusActive
	}
}

func cleanID(id string) string {
	return strings.ReplaceAll(strings.TrimSpace(id), "/", "-")
}

func newUserID() string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "user_" + strings.ReplaceAll(time.Now().UTC().Format(time.RFC3339Nano), ":", "-")
	}
	return "user_" + base64.RawURLEncoding.EncodeToString(raw[:])
}

func newVerificationID() string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "verify_" + strings.ReplaceAll(time.Now().UTC().Format(time.RFC3339Nano), ":", "-")
	}
	return "verify_" + base64.RawURLEncoding.EncodeToString(raw[:])
}
