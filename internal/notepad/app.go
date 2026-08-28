package notepad

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

func Run() error {
	time.Local = time.FixedZone("CST", 8*3600)

	baseDir := detectBaseDir()

	if err := InitLogger(baseDir); err != nil {
		return err
	}

	logInfo("SimpleNote starting, base dir: %s", baseDir)

	cfg, err := LoadConfig(baseDir)
	if err != nil {
		logError("load config: %v", err)
		return err
	}

	SetLogLevel(cfg.Log.Level)
	logInfo("config loaded, port: %s, log level: %s", cfg.Server.Port, currentLogLevel())
	logDebug("config: port=%s, password=%s, jwt_secret_len=%d, session=%s, remember=%s",
		cfg.Server.Port, cfg.Auth.Password, len(cfg.Auth.JWTSecret),
		cfg.Auth.SessionExpiresIn, cfg.Auth.RememberExpiresIn)

	app, err := newApp(baseDir, cfg)
	if err != nil {
		logError("init app: %v", err)
		return err
	}

	router := newRouter(app, baseDir)
	logInfo("server listening on :%s", cfg.Server.Port)
	return router.Run(":" + cfg.Server.Port)
}

func newApp(baseDir string, cfg *Config) (*App, error) {
	jwtSecret := strings.TrimSpace(cfg.Auth.JWTSecret)
	if jwtSecret == "" {
		return nil, errors.New("auth.jwt_secret is empty")
	}
	logDebug("jwt_secret length: %d", len(jwtSecret))

	sessionTTL, err := parseJWTExpiresIn(cfg.Auth.SessionExpiresIn)
	if err != nil {
		return nil, fmt.Errorf("parse auth.session_expires_in: %w", err)
	}
	logDebug("session ttl: %v", sessionTTL)

	rememberTTL, err := parseJWTExpiresIn(cfg.Auth.RememberExpiresIn)
	if err != nil {
		return nil, fmt.Errorf("parse auth.remember_expires_in: %w", err)
	}
	logDebug("remember ttl: %v", rememberTTL)

	store := &Store{
		notesDir:   filepath.Join(baseDir, "notes"),
		uploadsDir: filepath.Join(baseDir, "uploads"),
		metaFile:   filepath.Join(baseDir, "notes", "meta.json"),
	}
	logDebug("store dirs: notes=%s, uploads=%s, meta=%s", store.notesDir, store.uploadsDir, store.metaFile)

	if err := store.bootstrap(); err != nil {
		return nil, err
	}
	logDebug("store bootstrap done")

	return &App{
		store:       store,
		password:    cfg.Auth.Password,
		jwtSecret:   []byte(jwtSecret),
		sessionTTL:  sessionTTL,
		rememberTTL: rememberTTL,
	}, nil
}