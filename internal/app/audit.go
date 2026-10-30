package app

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"
)

// Audit appends an entry to the audit log. Failures are logged, never
// returned, so auditing cannot block the action itself.
func (a *App) Audit(ctx context.Context, u *User, ip, action, recordID string, detail any) {
	a.auditQ(ctx, a.DB, u, ip, action, recordID, detail)
}

func (a *App) auditQ(ctx context.Context, q querier, u *User, ip, action, recordID string, detail any) {
	if detail == nil {
		detail = map[string]any{}
	}
	b, _ := json.Marshal(detail)
	var uid any
	name := ""
	if u != nil {
		uid, name = u.ID, u.Username
	}
	if _, err := q.Exec(ctx, `INSERT INTO audit_log (user_id, username, action, record_id, detail, ip) VALUES ($1, $2, $3, $4, $5, $6)`,
		uid, name, action, recordID, b, ip); err != nil {
		log.Printf("audit %s: %v", action, err)
	}
}

type auditEntry struct {
	ID       int64           `json:"id"`
	At       time.Time       `json:"at"`
	Username string          `json:"username"`
	Action   string          `json:"action"`
	RecordID string          `json:"recordId"`
	Detail   json.RawMessage `json:"detail"`
	IP       string          `json:"ip"`
}

func (a *App) apiAuditLog(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	if !u.Can(PermAudit) {
		return errForbidden
	}
	q := r.URL.Query()
	before, _ := strconv.ParseInt(q.Get("before"), 10, 64)
	if before <= 0 {
		before = 1 << 62
	}
	like := "%" + q.Get("q") + "%"
	rows, err := a.DB.Query(r.Context(), `SELECT id, at, username, action, record_id, detail, ip FROM audit_log
		WHERE id < $1 AND ($2 = '%%' OR action ILIKE $2 OR username ILIKE $2 OR record_id ILIKE $2 OR detail::text ILIKE $2)
		ORDER BY id DESC LIMIT 200`, before, like)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []auditEntry{}
	for rows.Next() {
		var e auditEntry
		if err := rows.Scan(&e.ID, &e.At, &e.Username, &e.Action, &e.RecordID, &e.Detail, &e.IP); err != nil {
			return err
		}
		out = append(out, e)
	}
	return ok(w, out)
}
