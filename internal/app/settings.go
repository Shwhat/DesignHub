package app

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

func (a *App) loadSettings(ctx context.Context) error {
	rows, err := a.DB.Query(ctx, `SELECT key, value FROM settings`)
	if err != nil {
		return err
	}
	defer rows.Close()
	m := map[string]json.RawMessage{}
	for rows.Next() {
		var k string
		var v []byte
		if err := rows.Scan(&k, &v); err != nil {
			return err
		}
		m[k] = v
	}
	a.settingsMu.Lock()
	a.settings = m
	a.settingsMu.Unlock()
	return rows.Err()
}

// Setting decodes a setting into v. Missing settings leave v unchanged.
func (a *App) Setting(key string, v any) {
	a.settingsMu.RLock()
	raw, ok := a.settings[key]
	a.settingsMu.RUnlock()
	if ok {
		_ = json.Unmarshal(raw, v)
	}
}

// SetSetting stores a setting and records it in the audit log.
func (a *App) SetSetting(ctx context.Context, u *User, ip, key string, v any) error {
	return a.setSettingAudited(ctx, u, ip, key, v, v)
}

// setSettingAudited stores v and records auditValue, for settings that hold
// secrets.
func (a *App) setSettingAudited(ctx context.Context, u *User, ip, key string, v, auditValue any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if _, err := a.DB.Exec(ctx, `INSERT INTO settings (key, value, updated_by) VALUES ($1, $2, $3)
		ON CONFLICT (key) DO UPDATE SET value = $2, updated_by = $3, updated_at = now()`, key, b, u.ID); err != nil {
		return err
	}
	a.settingsMu.Lock()
	a.settings[key] = b
	a.settingsMu.Unlock()
	a.Audit(ctx, u, ip, "settings.change", "", map[string]any{"key": key, "value": auditValue})
	return nil
}

// AllowedHosts returns the allowed addresses, lower case.
func (a *App) AllowedHosts() []string {
	var hs []string
	a.Setting("allowed_hosts", &hs)
	for i := range hs {
		hs[i] = strings.ToLower(strings.TrimSpace(hs[i]))
	}
	return hs
}

// System settings visible in Admin, System (Super Admin) and Admin.

type systemSettings struct {
	AllowedHosts           []string       `json:"allowedHosts"`
	SignInMessage          string         `json:"signInMessage"`
	CORejectReturnsToDraft bool           `json:"coRejectReturnsToDraft"`
	BackupSchedule         backupSchedule `json:"backupSchedule"`
}

type backupSchedule struct {
	Hour int `json:"hour"`
	Keep int `json:"keep"`
}

func (a *App) currentSystemSettings() systemSettings {
	s := systemSettings{AllowedHosts: []string{}, BackupSchedule: backupSchedule{Hour: 2, Keep: 14}}
	a.Setting("allowed_hosts", &s.AllowedHosts)
	a.Setting("sign_in_message", &s.SignInMessage)
	a.Setting("co_reject_returns_to_draft", &s.CORejectReturnsToDraft)
	a.Setting("backup_schedule", &s.BackupSchedule)
	return s
}

func (a *App) apiGetSystemSettings(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	if !u.Can(PermSystem) && !u.Can(PermUsers) {
		return errForbidden
	}
	return ok(w, a.currentSystemSettings())
}

func (a *App) apiPutSystemSettings(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	var in struct {
		AllowedHosts           *[]string       `json:"allowedHosts"`
		SignInMessage          *string         `json:"signInMessage"`
		CORejectReturnsToDraft *bool           `json:"coRejectReturnsToDraft"`
		BackupSchedule         *backupSchedule `json:"backupSchedule"`
	}
	if err := decode(r, &in); err != nil {
		return err
	}
	ctx, ip := r.Context(), clientIP(r)
	// Admins manage the sign-in message; everything else is Super Admin only.
	if in.SignInMessage != nil {
		if !u.Can(PermUsers) {
			return errForbidden
		}
		if err := a.SetSetting(ctx, u, ip, "sign_in_message", strings.TrimSpace(*in.SignInMessage)); err != nil {
			return err
		}
	}
	if in.AllowedHosts != nil || in.CORejectReturnsToDraft != nil || in.BackupSchedule != nil {
		if !u.Can(PermSystem) {
			return errForbidden
		}
	}
	if in.AllowedHosts != nil {
		var hs []string
		for _, h := range *in.AllowedHosts {
			h = strings.ToLower(strings.TrimSpace(h))
			if h != "" {
				hs = append(hs, h)
			}
		}
		if hs == nil {
			hs = []string{}
		}
		// Refuse a list that would lock out the browser making the change.
		if len(hs) > 0 {
			host := strings.ToLower(r.Host)
			if i := strings.LastIndex(host, ":"); i > 0 && !strings.Contains(host[i:], "]") {
				host = host[:i]
			}
			found := false
			for _, h := range hs {
				if h == strings.Trim(host, "[]") {
					found = true
				}
			}
			if !found {
				return badRequest("The list must include the address you are using now (%s).", host)
			}
		}
		if err := a.SetSetting(ctx, u, ip, "allowed_hosts", hs); err != nil {
			return err
		}
	}
	if in.CORejectReturnsToDraft != nil {
		if err := a.SetSetting(ctx, u, ip, "co_reject_returns_to_draft", *in.CORejectReturnsToDraft); err != nil {
			return err
		}
	}
	if in.BackupSchedule != nil {
		b := *in.BackupSchedule
		if b.Hour < 0 || b.Hour > 23 || b.Keep < 1 || b.Keep > 365 {
			return badRequest("Backup hour must be 0 to 23 and copies kept 1 to 365.")
		}
		if err := a.SetSetting(ctx, u, ip, "backup_schedule", b); err != nil {
			return err
		}
	}
	return ok(w, a.currentSystemSettings())
}
