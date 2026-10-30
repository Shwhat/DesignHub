package app

import (
	"bytes"
	"context"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
)

// RunScheduler runs scheduled backups and due-date notifications until ctx
// is cancelled.
func (a *App) RunScheduler(ctx context.Context) {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	lastBackupDay := ""
	lastDue := time.Time{}
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-t.C:
			var s backupSchedule
			s.Hour = 2
			a.Setting("backup_schedule", &s)
			day := now.Format("2006-01-02")
			if now.Hour() == s.Hour && lastBackupDay != day {
				lastBackupDay = day
				name, err := a.Backup(ctx)
				a.Audit(ctx, nil, "", "backup.scheduled", "", map[string]any{"name": name, "error": errString(err)})
				if err != nil {
					log.Printf("scheduled backup failed: %v", err)
				}
			}
			if now.Sub(lastDue) > time.Hour {
				lastDue = now
				a.dueDateNotifications(ctx)
				a.DB.Exec(ctx, `DELETE FROM sessions WHERE expires_at < now() OR (NOT keep AND last_seen < now() - interval '1 day')`)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Baseline signing key

var signingEntity *openpgp.Entity

// LoadSigningKey reads the baseline signing key from the secrets folder,
// creating it on first use. Without a writable secrets folder, baselines
// are tagged without a signature.
func (a *App) LoadSigningKey() {
	p := filepath.Join(a.Cfg.SecretsDir, "signing.asc")
	if b, err := os.ReadFile(p); err == nil {
		el, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(b))
		if err == nil && len(el) > 0 {
			signingEntity = el[0]
			return
		}
		log.Printf("signing key unreadable: %v", err)
		return
	}
	e, err := openpgp.NewEntity("DesignHub Baselines", "", "baselines@designhub.local", &packet.Config{Algorithm: packet.PubKeyAlgoEdDSA})
	if err != nil {
		log.Printf("signing key: %v", err)
		return
	}
	var buf bytes.Buffer
	w, err := armor.Encode(&buf, openpgp.PrivateKeyType, nil)
	if err != nil {
		return
	}
	if err := e.SerializePrivate(w, nil); err != nil {
		return
	}
	w.Close()
	if err := os.WriteFile(p, buf.Bytes(), 0o600); err != nil {
		log.Printf("baselines will not be signed: %v", err)
		return
	}
	signingEntity = e
}

func (a *App) signingKey() *openpgp.Entity { return signingEntity }

// apiSigningKey returns the public key that verifies baseline tags.
func (a *App) apiSigningKey(w http.ResponseWriter, r *http.Request) error {
	if signingEntity == nil {
		return errNotFound
	}
	var buf bytes.Buffer
	aw, err := armor.Encode(&buf, openpgp.PublicKeyType, nil)
	if err != nil {
		return err
	}
	if err := signingEntity.Serialize(aw); err != nil {
		return err
	}
	aw.Close()
	w.Header().Set("Content-Type", "application/pgp-keys")
	w.Header().Set("Content-Disposition", `attachment; filename="designhub-baselines.asc"`)
	_, err = w.Write(buf.Bytes())
	return err
}
