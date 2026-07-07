package main

import (
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"

	"github.com/YankongLi/talent-profile/backend/internal/config"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	var databaseURL string
	var migrationsDir string
	flags := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flags.StringVar(&databaseURL, "database", "", "PostgreSQL connection string")
	flags.StringVar(&migrationsDir, "dir", "", "migration files directory")
	if err := flags.Parse(os.Args[1:]); err != nil {
		logger.Error("parse flags failed", "error", err)
		os.Exit(2)
	}

	args := flags.Args()
	if len(args) == 0 {
		usage(flags)
		os.Exit(2)
	}

	if databaseURL == "" {
		cfg, err := config.Load()
		if err != nil {
			logger.Error("load config failed", "error", err)
			os.Exit(1)
		}
		databaseURL = cfg.Database.DSN
	}
	if databaseURL == "" {
		logger.Error("database dsn is required; set DATABASE_DSN or pass -database")
		os.Exit(1)
	}

	resolvedDir, err := resolveMigrationsDir(migrationsDir)
	if err != nil {
		logger.Error("resolve migrations directory failed", "error", err)
		os.Exit(1)
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		logger.Error("open database failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		logger.Error("create migration driver failed", "error", err)
		os.Exit(1)
	}

	sourceURL := "file://" + filepath.ToSlash(resolvedDir)
	migrator, err := migrate.NewWithDatabaseInstance(sourceURL, "postgres", driver)
	if err != nil {
		logger.Error("create migrator failed", "error", err)
		os.Exit(1)
	}
	defer migrator.Close()

	if err := run(args, migrator); err != nil {
		logger.Error("migration failed", "error", err)
		os.Exit(1)
	}

	logger.Info("migration completed", "command", args[0])
}

type migrationRunner interface {
	Up() error
	Steps(n int) error
	Version() (uint, bool, error)
}

func run(args []string, migrator migrationRunner) error {
	switch args[0] {
	case "up":
		err := migrator.Up()
		if errors.Is(err, migrate.ErrNoChange) {
			return nil
		}
		return err
	case "down":
		steps := 1
		if len(args) > 1 {
			parsed, err := strconv.Atoi(args[1])
			if err != nil || parsed <= 0 {
				return fmt.Errorf("down steps must be a positive integer")
			}
			steps = parsed
		}
		err := migrator.Steps(-steps)
		if errors.Is(err, migrate.ErrNoChange) {
			return nil
		}
		return err
	case "version":
		version, dirty, err := migrator.Version()
		if errors.Is(err, migrate.ErrNilVersion) {
			fmt.Println("version=0 dirty=false")
			return nil
		}
		if err != nil {
			return err
		}
		fmt.Printf("version=%d dirty=%t\n", version, dirty)
		return nil
	default:
		return fmt.Errorf("unsupported command %q", args[0])
	}
}

func resolveMigrationsDir(dir string) (string, error) {
	if dir == "" {
		dir = os.Getenv("DATABASE_MIGRATIONS_DIR")
	}
	candidates := []string{dir}
	if dir == "" {
		candidates = []string{"migrations", "backend/migrations"}
	}

	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		info, err := os.Stat(candidate)
		if err == nil && info.IsDir() {
			abs, err := filepath.Abs(candidate)
			if err != nil {
				return "", err
			}
			return abs, nil
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
	}

	return "", fmt.Errorf("migrations directory not found")
}

func usage(flags *flag.FlagSet) {
	fmt.Fprintf(flags.Output(), "Usage: %s [flags] up|down [steps]|version\n", flags.Name())
	flags.PrintDefaults()
}
