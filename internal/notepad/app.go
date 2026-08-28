package notepad

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

func Run() error {
	baseDir := detectBaseDir()

	cfg, err := LoadConfig(baseDir)
	if err != nil {
		return err
	}

	app, err := newApp(baseDir, cfg)
	if err != nil {
		return err
	}

	router := newRouter(app, baseDir)
	return router.Run(":" + cfg.Server.Port)
}

func newApp(baseDir string, cfg *Config) (*App, error) {
	jwtSecret := strings.TrimSpace(cfg.Auth.JWTSecret)
	if jwtSecret == "" {
		return nil, errors.New("auth.jwt_secret is empty")
	}

	sessionTTL, err := parseJWTExpiresIn(cfg.Auth.SessionExpiresIn)
	if err != nil {
		return nil, fmt.Errorf("parse auth.session_expires_in: %w", err)
	}

	rememberTTL, err := parseJWTExpiresIn(cfg.Auth.RememberExpiresIn)
	if err != nil {
		return nil, fmt.Errorf("parse auth.remember_expires_in: %w", err)
	}

	store := &Store{
		notesDir:   filepath.Join(baseDir, "notes"),
		uploadsDir: filepath.Join(baseDir, "uploads"),
		metaFile:   filepath.Join(baseDir, "notes", "meta.json"),
	}

	if err := store.bootstrap(); err != nil {
		return nil, err
	}

	return &App{
		store:       store,
		password:    cfg.Auth.Password,
		jwtSecret:   []byte(jwtSecret),
		sessionTTL:  sessionTTL,
		rememberTTL: rememberTTL,
	}, nil
}