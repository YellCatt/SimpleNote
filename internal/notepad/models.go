package notepad

import (
	"regexp"
	"sync"
	"time"
)

const (
	appName                    = "简单笔记"
	defaultPort                = "3001"
	defaultPassword            = "asd123456"
	defaultSessionExpiresIn    = "1d"
	defaultRememberExpiresIn   = "30d"
	defaultLogLevel            = "debug"
	rememberCookieMaxAgeSecond = 30 * 24 * 60 * 60
	maxUploadSize              = 10 * 1024 * 1024
	authCookieName             = "auth_token"

	logLevelDebug = "debug"
	logLevelInfo  = "info"
	logLevelError = "error"
)

var (
	attachmentPattern = regexp.MustCompile(`(?i)/uploads/([a-f0-9-]+\.[a-z0-9]+)`)
	imagePattern      = regexp.MustCompile(`(?i)\.(jpg|jpeg|png|gif|webp|svg|bmp)$`)
	expiresInPattern  = regexp.MustCompile(`(?i)^\s*([+-]?\d+(?:\.\d+)?)\s*([a-z]+)?\s*$`)
)

type Note struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	CreatedAt   int64    `json:"createdAt"`
	UpdatedAt   int64    `json:"updatedAt,omitempty"`
	Attachments []string `json:"attachments,omitempty"`
}

type Meta struct {
	Notes    []Note `json:"notes"`
	Migrated bool   `json:"migrated,omitempty"`
}

type Action struct {
	Href string
	Text string
}

type Store struct {
	mu         sync.Mutex
	notesDir   string
	uploadsDir string
	metaFile   string
}

type App struct {
	store       *Store
	password    string
	jwtSecret   []byte
	sessionTTL  time.Duration
	rememberTTL time.Duration
}