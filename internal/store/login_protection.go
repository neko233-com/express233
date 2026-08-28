package store

import (
	"database/sql"
	"encoding/json"
	"strings"
	"time"
)

// LoginIPPolicy controls persistent IP-based credential-stuffing protection.
// Values are supplied by the API layer so deployments can tune them safely.
type LoginIPPolicy struct {
	FailureLimit int
	Window       time.Duration
	BaseBan      time.Duration
	MaxBan       time.Duration
}

type LoginIPBan struct {
	IP               string   `json:"ip"`
	Username         string   `json:"username"`
	Failures         int      `json:"failures"`
	AttemptCount     int      `json:"attempt_count"`
	BanCount         int      `json:"ban_count"`
	WindowStarted    string   `json:"window_started"`
	BannedUntil      string   `json:"banned_until"`
	LastFailure      string   `json:"last_failure"`
	LastAttemptTimes []string `json:"last_attempt_times"`
}

func containsDuplicateColumn(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "duplicate column name")
}

func (s *Store) migrateLoginProtection() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS login_ip_bans (
  ip TEXT PRIMARY KEY,
  failures INTEGER NOT NULL DEFAULT 0,
  ban_count INTEGER NOT NULL DEFAULT 0,
  window_started TEXT NOT NULL,
  banned_until TEXT NOT NULL DEFAULT '',
  last_failure TEXT NOT NULL,
  username TEXT NOT NULL DEFAULT '',
  attempt_count INTEGER NOT NULL DEFAULT 0,
  last_attempts TEXT NOT NULL DEFAULT '[]'
);
CREATE INDEX IF NOT EXISTS idx_login_ip_bans_until ON login_ip_bans(banned_until);
CREATE TABLE IF NOT EXISTS login_protection_settings (
  singleton INTEGER PRIMARY KEY CHECK(singleton = 1),
  enabled INTEGER NOT NULL DEFAULT 0
);
INSERT OR IGNORE INTO login_protection_settings(singleton, enabled) VALUES(1, 0);
`)
	if err != nil {
		return err
	}
	for _, migration := range []string{
		"ALTER TABLE login_ip_bans ADD COLUMN username TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE login_ip_bans ADD COLUMN attempt_count INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE login_ip_bans ADD COLUMN last_attempts TEXT NOT NULL DEFAULT '[]'",
	} {
		if _, err := s.db.Exec(migration); err != nil && !containsDuplicateColumn(err) {
			return err
		}
	}
	return nil
}

// LoginProtectionEnabled returns whether failed credentials may temporarily
// block a source IP. It defaults to off so an operator can observe attacks
// before enabling enforcement.
func (s *Store) LoginProtectionEnabled() (bool, error) {
	var enabled int
	if err := s.db.QueryRow(`SELECT enabled FROM login_protection_settings WHERE singleton = 1`).Scan(&enabled); err != nil {
		return false, err
	}
	return enabled != 0, nil
}

// SetLoginProtectionEnabled persists the administrator's enforcement switch.
func (s *Store) SetLoginProtectionEnabled(enabled bool) error {
	value := 0
	if enabled {
		value = 1
	}
	_, err := s.db.Exec(`INSERT INTO login_protection_settings(singleton, enabled) VALUES(1, ?) ON CONFLICT(singleton) DO UPDATE SET enabled = excluded.enabled`, value)
	return err
}

func (s *Store) LoginIPBlocked(ip string, now time.Time) (time.Duration, bool, error) {
	var until string
	err := s.db.QueryRow(`SELECT banned_until FROM login_ip_bans WHERE ip=?`, ip).Scan(&until)
	if err == sql.ErrNoRows || until == "" {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	t, err := time.Parse(time.RFC3339Nano, until)
	if err != nil || !t.After(now) {
		return 0, false, nil
	}
	return time.Until(t).Round(time.Second), true, nil
}

// RecordLoginIPFailure advances the per-IP counters atomically. A non-positive
// FailureLimit records attempts only; it never creates a ban.
func (s *Store) RecordLoginIPFailure(ip, username string, now time.Time, policy LoginIPPolicy) (LoginIPBan, error) {
	out := LoginIPBan{IP: ip}
	tx, err := s.db.Begin()
	if err != nil {
		return out, err
	}
	defer func() { _ = tx.Rollback() }()
	var windowStarted, bannedUntil, lastAttempts string
	err = tx.QueryRow(`SELECT username,failures,attempt_count,ban_count,window_started,banned_until,last_failure,last_attempts FROM login_ip_bans WHERE ip=?`, ip).Scan(&out.Username, &out.Failures, &out.AttemptCount, &out.BanCount, &windowStarted, &bannedUntil, &out.LastFailure, &lastAttempts)
	if err == sql.ErrNoRows {
		out.Username, out.Failures, out.AttemptCount, out.BanCount = username, 0, 0, 0
		windowStarted = now.UTC().Format(time.RFC3339Nano)
	} else if err != nil {
		return out, err
	}
	started, parseErr := time.Parse(time.RFC3339Nano, windowStarted)
	if parseErr != nil || now.Sub(started) >= policy.Window {
		out.Failures = 0
		windowStarted = now.UTC().Format(time.RFC3339Nano)
	}
	out.Failures++
	out.AttemptCount++
	out.Username = username
	_ = json.Unmarshal([]byte(lastAttempts), &out.LastAttemptTimes)
	out.LastAttemptTimes = append(out.LastAttemptTimes, now.UTC().Format(time.RFC3339Nano))
	if len(out.LastAttemptTimes) > 3 {
		out.LastAttemptTimes = out.LastAttemptTimes[len(out.LastAttemptTimes)-3:]
	}
	if policy.FailureLimit > 0 && out.Failures >= policy.FailureLimit {
		out.BanCount++
		ban := policy.BaseBan
		for i := 1; i < out.BanCount && ban < policy.MaxBan; i++ {
			ban *= 2
		}
		if ban > policy.MaxBan {
			ban = policy.MaxBan
		}
		bannedUntil = now.Add(ban).UTC().Format(time.RFC3339Nano)
		out.Failures = 0
		windowStarted = now.UTC().Format(time.RFC3339Nano)
	}
	out.WindowStarted, out.BannedUntil, out.LastFailure = windowStarted, bannedUntil, now.UTC().Format(time.RFC3339Nano)
	encodedAttempts, _ := json.Marshal(out.LastAttemptTimes)
	_, err = tx.Exec(`INSERT INTO login_ip_bans(ip,username,failures,attempt_count,ban_count,window_started,banned_until,last_failure,last_attempts) VALUES(?,?,?,?,?,?,?,?,?) ON CONFLICT(ip) DO UPDATE SET username=excluded.username,failures=excluded.failures,attempt_count=excluded.attempt_count,ban_count=excluded.ban_count,window_started=excluded.window_started,banned_until=excluded.banned_until,last_failure=excluded.last_failure,last_attempts=excluded.last_attempts`, out.IP, out.Username, out.Failures, out.AttemptCount, out.BanCount, out.WindowStarted, out.BannedUntil, out.LastFailure, string(encodedAttempts))
	if err != nil {
		return LoginIPBan{}, err
	}
	return out, tx.Commit()
}

func (s *Store) ClearLoginIPFailures(ip string) error {
	_, err := s.db.Exec(`UPDATE login_ip_bans SET failures=0,banned_until='' WHERE ip=?`, ip)
	return err
}

// DeleteLoginIPHistory removes an attacker record after administrator review.
func (s *Store) DeleteLoginIPHistory(ip string) error {
	_, err := s.db.Exec(`DELETE FROM login_ip_bans WHERE ip=?`, ip)
	return err
}

func (s *Store) ListLoginIPBans(now time.Time, limit int) ([]LoginIPBan, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.Query(`SELECT ip,username,failures,attempt_count,ban_count,window_started,banned_until,last_failure,last_attempts FROM login_ip_bans WHERE last_failure >= ? ORDER BY last_failure DESC LIMIT ?`, now.Add(-LogRetention).UTC().Format(time.RFC3339Nano), limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []LoginIPBan
	for rows.Next() {
		var b LoginIPBan
		var lastAttempts string
		if err := rows.Scan(&b.IP, &b.Username, &b.Failures, &b.AttemptCount, &b.BanCount, &b.WindowStarted, &b.BannedUntil, &b.LastFailure, &lastAttempts); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(lastAttempts), &b.LastAttemptTimes)
		out = append(out, b)
	}
	return out, rows.Err()
}
