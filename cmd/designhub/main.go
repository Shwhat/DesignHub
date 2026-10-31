// Command designhub runs the DesignHub server and its maintenance commands.
package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"

	"designhub/internal/app"
	"designhub/internal/config"
	"designhub/internal/db"
	"designhub/internal/secrets"
	"designhub/web"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

const usage = `DesignHub %s

Usage: designhub <command>

Commands:
  serve          Run the server (default)
  init-secrets   Create the database password and TLS certificate if missing
  setup-admin    Create a Super Admin account, or reset a Super Admin password
  recovery       Let a Super Admin sign in with a password for one hour,
                 even when password sign-in is turned off
  seed           Load the test data set (test installations only)
  backup         Write a backup now
  restore NAME   Restore a backup (stop the designhub service first)
  backups        List backups
  healthcheck    Exit 0 when the server answers
  version        Print the version
`

func main() {
	log.SetFlags(log.LstdFlags)
	cmd := "serve"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	var err error
	switch cmd {
	case "serve":
		err = serve()
	case "init-secrets":
		err = initSecrets()
	case "setup-admin":
		err = setupAdmin()
	case "recovery":
		err = recovery()
	case "seed":
		err = seed()
	case "backup":
		err = backup()
	case "restore":
		if len(os.Args) < 3 {
			err = errors.New("usage: designhub restore NAME (see designhub backups)")
		} else {
			err = restore(os.Args[2])
		}
	case "backups":
		err = listBackups()
	case "healthcheck":
		err = healthcheck()
	case "version":
		fmt.Println(version)
	case "help", "-h", "--help":
		fmt.Printf(usage, version)
	default:
		fmt.Fprintf(os.Stderr, usage, version)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func loadConfig() (*config.Config, error) {
	return config.Load(version)
}

// open connects to the database, applies migrations and starts the app.
func open(ctx context.Context) (*app.App, func(), error) {
	cfg, err := loadConfig()
	if err != nil {
		return nil, nil, err
	}
	pool, err := db.Connect(ctx, cfg.DSN(), 2*time.Minute)
	if err != nil {
		return nil, nil, err
	}
	done, err := db.Migrate(ctx, pool)
	for _, m := range done {
		log.Printf("applied migration %s", m)
	}
	if err != nil {
		pool.Close()
		return nil, nil, err
	}
	a, err := app.New(ctx, cfg, pool, web.Files())
	if err != nil {
		pool.Close()
		return nil, nil, err
	}
	a.LoadSigningKey()
	return a, pool.Close, nil
}

func serve() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	a, closeDB, err := open(ctx)
	if err != nil {
		return err
	}
	defer closeDB()
	go a.RunScheduler(ctx)
	srv := &http.Server{
		Addr:              a.Cfg.Addr,
		Handler:           a.Handler(),
		ReadHeaderTimeout: 15 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}
	errc := make(chan error, 1)
	go func() {
		log.Printf("DesignHub %s listening on %s", version, a.Cfg.Addr)
		errc <- srv.ListenAndServe()
	}()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	log.Print("shutting down")
	sctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return srv.Shutdown(sctx)
}

func initSecrets() error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	created, err := secrets.Init(cfg.SecretsDir)
	if err != nil {
		return err
	}
	for _, c := range created {
		log.Printf("created %s", c)
	}
	if len(created) == 0 {
		log.Print("secrets already present")
	}
	return nil
}

var stdin = bufio.NewReader(os.Stdin)

func prompt(label string) (string, error) {
	fmt.Print(label)
	s, err := stdin.ReadString('\n')
	if err != nil && s == "" {
		return "", err
	}
	return strings.TrimSpace(s), nil
}

// promptPassword reads a password twice without echo.
func promptPassword(label string) (string, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return "", errors.New("run this command in an interactive terminal (docker compose exec -it, or docker compose run --rm -it)")
	}
	for i := 0; i < 3; i++ {
		fmt.Print(label)
		a, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err != nil {
			return "", err
		}
		fmt.Print("Repeat password: ")
		b, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err != nil {
			return "", err
		}
		if string(a) != string(b) {
			fmt.Println("The passwords do not match. Try again.")
			continue
		}
		return string(a), nil
	}
	return "", errors.New("passwords did not match")
}

func setupAdmin() error {
	ctx := context.Background()
	a, closeDB, err := open(ctx)
	if err != nil {
		return err
	}
	defer closeDB()
	fmt.Println("DesignHub Super Admin setup")
	username, err := prompt("Username: ")
	if err != nil {
		return err
	}
	exists, role, err := a.UserRole(ctx, username)
	if err != nil {
		return err
	}
	if exists {
		if role != app.RoleSuperAdmin {
			return fmt.Errorf("user %s exists and is not a Super Admin", username)
		}
		fmt.Printf("%s already exists. Enter a new password to reset it.\n", username)
		pw, err := promptPassword("New password: ")
		if err != nil {
			return err
		}
		if err := a.ResetPassword(ctx, username, pw); err != nil {
			return err
		}
		fmt.Println("Password reset. Sign in with the new password. If password sign-in is turned off, run designhub recovery instead.")
		return nil
	}
	display, err := prompt("Display name: ")
	if err != nil {
		return err
	}
	email, err := prompt("Email (optional): ")
	if err != nil {
		return err
	}
	pw, err := promptPassword("Password (at least 10 characters): ")
	if err != nil {
		return err
	}
	if err := a.SetupSuperAdmin(ctx, username, display, email, pw); err != nil {
		return err
	}
	fmt.Printf("Super Admin %s created. Open DesignHub in a browser and sign in.\n", username)
	return nil
}

func seed() error {
	ctx := context.Background()
	a, closeDB, err := open(ctx)
	if err != nil {
		return err
	}
	defer closeDB()
	fmt.Println("This loads the DesignHub test data set (TEST-PLAN.md Appendix A).")
	fmt.Println("Use it only on a test installation.")
	ans, err := prompt("Type LOAD to continue: ")
	if err != nil {
		return err
	}
	if ans != "LOAD" {
		return errors.New("cancelled")
	}
	pw, err := promptPassword("Password for all test accounts (at least 10 characters): ")
	if err != nil {
		return err
	}
	if err := a.LoadSeed(ctx, pw); err != nil {
		return err
	}
	fmt.Println("Test data loaded.")
	return nil
}

func backup() error {
	ctx := context.Background()
	a, closeDB, err := open(ctx)
	if err != nil {
		return err
	}
	defer closeDB()
	name, err := a.Backup(ctx)
	if err != nil {
		return err
	}
	fmt.Println("Backup written:", name)
	return nil
}

func listBackups() error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(cfg.BackupsDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), "designhub-") {
			mark := ""
			if _, err := os.Stat(filepath.Join(cfg.BackupsDir, e.Name(), "manifest.json")); err != nil {
				mark = "  (incomplete)"
			}
			fmt.Println(e.Name() + mark)
		}
	}
	return nil
}

func restore(name string) error {
	ctx := context.Background()
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	dir := filepath.Join(cfg.BackupsDir, filepath.Base(name))
	if _, err := os.Stat(filepath.Join(dir, "manifest.json")); err != nil {
		return fmt.Errorf("%s is not a complete backup", name)
	}
	fmt.Printf("This replaces all DesignHub data with backup %s.\n", name)
	ans, err := prompt("Type RESTORE to continue: ")
	if err != nil {
		return err
	}
	if ans != "RESTORE" {
		return errors.New("cancelled")
	}
	pool, err := db.Connect(ctx, cfg.DSN(), time.Minute)
	if err != nil {
		return err
	}
	pool.Close()
	r := &app.App{Cfg: cfg}
	fmt.Println("Restoring database...")
	if err := r.RestoreDatabase(ctx, filepath.Join(dir, "database.dump")); err != nil {
		return err
	}
	fmt.Println("Restoring engineering repository...")
	if _, err := os.Stat(cfg.RepoDir); err == nil {
		aside := cfg.RepoDir + ".before-restore-" + time.Now().Format("20060102-150405")
		if err := os.Rename(cfg.RepoDir, aside); err != nil {
			return err
		}
		fmt.Println("Previous repository kept at", aside)
	}
	if err := os.MkdirAll(cfg.RepoDir, 0o755); err != nil {
		return err
	}
	if err := app.UntarDir(filepath.Join(dir, "repository.tar.gz"), cfg.RepoDir); err != nil {
		return err
	}
	fmt.Println("Restoring attachment files...")
	n, err := app.SyncFilesBack(cfg.BackupsDir, cfg.FilesDir)
	if err != nil {
		return err
	}
	fmt.Printf("Restored %d files. Start DesignHub; the search index is rebuilt on startup.\n", n)
	return nil
}

func healthcheck() error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	addr := cfg.Addr
	if strings.HasPrefix(addr, ":") {
		addr = "127.0.0.1" + addr
	}
	c := &http.Client{Timeout: 4 * time.Second}
	resp, err := c.Get("http://" + addr + "/healthz")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}

func recovery() error {
	ctx := context.Background()
	a, closeDB, err := open(ctx)
	if err != nil {
		return err
	}
	defer closeDB()
	fmt.Println("DesignHub recovery sign-in")
	fmt.Println("Sets a new password for a Super Admin and allows password sign-in for that account for one hour.")
	username, err := prompt("Super Admin username: ")
	if err != nil {
		return err
	}
	pw, err := promptPassword("New password (at least 10 characters): ")
	if err != nil {
		return err
	}
	until, err := a.Recover(ctx, username, pw)
	if err != nil {
		return err
	}
	fmt.Printf("Sign in as %s with the new password before %s (UTC). The window closes after the first sign-in.\n", username, until.Format("2006-01-02 15:04"))
	return nil
}
