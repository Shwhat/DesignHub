// Package secrets generates the values the stack needs on first start: the
// database password and a self-signed TLS certificate for the proxy. Existing
// files are never overwritten.
package secrets

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
)

// AppUID is the user the application container runs as.
const AppUID = 65532

// Init creates any missing secret files in dir and reports what it created.
func Init(dir string) ([]string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	var created []string
	pw := filepath.Join(dir, "db_password")
	if !exists(pw) {
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			return nil, err
		}
		if err := writeFile(pw, []byte(base64.RawURLEncoding.EncodeToString(b)+"\n")); err != nil {
			return nil, err
		}
		created = append(created, "database password")
	}
	crt, key := filepath.Join(dir, "tls.crt"), filepath.Join(dir, "tls.key")
	if !exists(crt) || !exists(key) {
		c, k, err := selfSigned()
		if err != nil {
			return nil, err
		}
		if err := writeFile(key, k); err != nil {
			return nil, err
		}
		if err := writeFile(crt, c); err != nil {
			return nil, err
		}
		created = append(created, "self-signed TLS certificate")
	}
	sk := filepath.Join(dir, "signing.asc")
	if !exists(sk) {
		b, err := signingKey()
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(sk, b, 0o600); err != nil {
			return nil, err
		}
		// Only the application reads the baseline signing key.
		if os.Geteuid() == 0 {
			os.Chown(sk, AppUID, AppUID)
		}
		created = append(created, "baseline signing key")
	}
	return created, nil
}

// signingKey creates an OpenPGP key used to sign baseline tags.
func signingKey() ([]byte, error) {
	e, err := openpgp.NewEntity("DesignHub Baselines", "", "baselines@designhub.local", &packet.Config{Algorithm: packet.PubKeyAlgoEdDSA})
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	w, err := armor.Encode(&buf, openpgp.PrivateKeyType, nil)
	if err != nil {
		return nil, err
	}
	if err := e.SerializePrivate(w, nil); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func selfSigned() (certPEM, keyPEM []byte, err error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 127))
	if err != nil {
		return nil, nil, err
	}
	now := time.Now()
	tpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "DesignHub", Organization: []string{"DesignHub"}},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"designhub.local", "localhost"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &priv.PublicKey, priv)
	if err != nil {
		return nil, nil, err
	}
	kb, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return nil, nil, err
	}
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: kb})
	return certPEM, keyPEM, nil
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// writeFile writes atomically. Files are world-readable inside the private
// secrets volume because the database, proxy and application containers run
// as different users.
func writeFile(p string, b []byte) error {
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", p, err)
	}
	return os.Rename(tmp, p)
}
