package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"nofx/auth"
	"nofx/config"
	"nofx/crypto"
	"nofx/logger"
	"nofx/store"

	"github.com/joho/godotenv"
	"golang.org/x/term"
	"gorm.io/gorm"
)

// minResetPasswordLen mirrors the minimum enforced on the authenticated
// password-change path (PUT /api/user/password).
const minResetPasswordLen = 8

// runCLISubcommand dispatches local admin subcommands.
//
// SECURITY: account recovery (reset-password, reset-account) is intentionally
// NOT exposed over HTTP. Performing it requires running this binary on the host,
// which in turn requires shell/file access to the server. A remote attacker on a
// public-facing deployment has only the network — they can reach the API but not
// a local process — so recovery cannot be triggered remotely. This is what makes
// the recovery path safe even when NOFX is deployed on the public internet.
//
// Returns true if a subcommand was recognized and handled (caller should exit).
// Unknown first args fall through to false to preserve the historical behavior
// where `nofx <dbpath>` overrides the SQLite path.
func runCLISubcommand(args []string) bool {
	if len(args) == 0 {
		return false
	}
	switch args[0] {
	case "reset-password":
		runResetPassword(args[1:])
		return true
	case "reset-account":
		runResetAccount(args[1:])
		return true
	default:
		return false
	}
}

// openStoreForCLI loads config + encryption and opens the same database the
// server uses, so subcommands operate on the live data.
func openStoreForCLI(dbPathOverride string) (*store.Store, error) {
	_ = godotenv.Load()
	logger.Init(nil)
	config.MustInit()
	cfg := config.Get()
	if strings.TrimSpace(dbPathOverride) != "" {
		cfg.DBPath = dbPathOverride
	}

	cryptoService, err := crypto.NewCryptoService()
	if err != nil {
		return nil, fmt.Errorf("initialize encryption service: %w", err)
	}
	crypto.SetGlobalCryptoService(cryptoService)

	if cfg.DBType == "sqlite" {
		if dir := filepath.Dir(cfg.DBPath); dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return nil, fmt.Errorf("create data directory: %w", err)
			}
		}
	}

	dbType := store.DBTypeSQLite
	if cfg.DBType == "postgres" {
		dbType = store.DBTypePostgres
	}
	return store.NewWithConfig(store.DBConfig{
		Type:     dbType,
		Path:     cfg.DBPath,
		Host:     cfg.DBHost,
		Port:     cfg.DBPort,
		User:     cfg.DBUser,
		Password: cfg.DBPassword,
		DBName:   cfg.DBName,
		SSLMode:  cfg.DBSSLMode,
	})
}

// runResetPassword resets the password for a single account from the command
// line. Usage: `nofx reset-password --email you@example.com`.
func runResetPassword(args []string) {
	fs := flag.NewFlagSet("reset-password", flag.ExitOnError)
	email := fs.String("email", "", "email of the account to reset (required)")
	password := fs.String("password", "", "new password (min 8 chars); omit to enter it interactively")
	dbPath := fs.String("db", "", "override SQLite DB path (defaults to config / DB_PATH)")
	_ = fs.Parse(args)

	if strings.TrimSpace(*email) == "" {
		fmt.Fprintln(os.Stderr, "error: --email is required")
		fmt.Fprintln(os.Stderr, "usage: nofx reset-password --email you@example.com")
		os.Exit(2)
	}

	st, err := openStoreForCLI(*dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer st.Close()

	user, err := st.User().GetByEmail(strings.TrimSpace(*email))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: no account found for %q\n", strings.TrimSpace(*email))
		os.Exit(1)
	}

	newPassword, err := resolveNewPassword(*password)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	hash, err := auth.HashPassword(newPassword)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to hash password: %v\n", err)
		os.Exit(1)
	}
	if err := st.User().UpdatePassword(user.ID, hash); err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to update password: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Password reset for %s. Log in with the new password.\n", user.Email)
}

// runResetAccount wipes the database back to an uninitialized state. This is the
// destructive "forgot everything" recovery, moved off the public API.
func runResetAccount(args []string) {
	fs := flag.NewFlagSet("reset-account", flag.ExitOnError)
	dbPath := fs.String("db", "", "override SQLite DB path (defaults to config / DB_PATH)")
	yes := fs.Bool("yes", false, "skip the interactive confirmation prompt")
	_ = fs.Parse(args)

	if !*yes {
		fmt.Print("This permanently deletes ALL users, traders, strategies, AI models and\n" +
			"exchanges — including wallet keys and exchange credentials.\n" +
			"Type 'wipe' to confirm: ")
		line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		if strings.TrimSpace(line) != "wipe" {
			fmt.Fprintln(os.Stderr, "aborted")
			os.Exit(1)
		}
	}

	st, err := openStoreForCLI(*dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer st.Close()

	err = st.Transaction(func(tx *gorm.DB) error {
		tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&store.Trader{})
		tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&store.Strategy{})
		tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&store.AIModel{})
		tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&store.Exchange{})
		if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&store.User{}).Error; err != nil {
			return fmt.Errorf("failed to delete users: %w", err)
		}
		return nil
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to reset account: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✓ System wiped. Register a fresh account and re-import everything.")
}

// resolveNewPassword returns the new password from the --password flag, or
// prompts for it (hidden) on a TTY, or reads a single line from piped stdin.
func resolveNewPassword(flagValue string) (string, error) {
	if flagValue != "" {
		if len(flagValue) < minResetPasswordLen {
			return "", fmt.Errorf("password must be at least %d characters", minResetPasswordLen)
		}
		return flagValue, nil
	}

	if term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Printf("New password (min %d chars): ", minResetPasswordLen)
		first, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err != nil {
			return "", fmt.Errorf("failed to read password: %w", err)
		}
		fmt.Print("Confirm new password: ")
		second, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err != nil {
			return "", fmt.Errorf("failed to read password: %w", err)
		}
		if string(first) != string(second) {
			return "", errors.New("passwords do not match")
		}
		if len(first) < minResetPasswordLen {
			return "", fmt.Errorf("password must be at least %d characters", minResetPasswordLen)
		}
		return string(first), nil
	}

	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	password := strings.TrimRight(line, "\r\n")
	if password == "" {
		return "", fmt.Errorf("no password provided on stdin: %w", err)
	}
	if len(password) < minResetPasswordLen {
		return "", fmt.Errorf("password must be at least %d characters", minResetPasswordLen)
	}
	return password, nil
}
