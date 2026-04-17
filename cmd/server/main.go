package main

import (
	"bufio"
	"compress/gzip"
	"context"
	"errors"
	"flag"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"cloud.google.com/go/storage"
	"github.com/alan/not-scrabble/internal/dict"
	"github.com/alan/not-scrabble/internal/httpapi"
	"github.com/alan/not-scrabble/internal/push"
	"github.com/alan/not-scrabble/internal/store"
	"github.com/alan/not-scrabble/webdist"
)

// fallbackWords is a tiny word list used when no dictionary file is supplied,
// so the server starts and basic plays validate even before you fetch ENABLE.
// Point `-dict` at a full word list for real play — see README.
var fallbackWords = []string{
	"CAT", "CATS", "DOG", "DOGS", "HI", "HA", "IT", "AT", "AS", "AA", "AE",
	"QI", "ZA", "OE", "XI", "XU", "JO", "KA", "OW", "OX", "UT", "EF", "EH",
	"TO", "OF", "ON", "NO", "AN", "IN", "IS", "IT", "HE", "SHE", "BE", "ME",
	"GO", "DO", "UP", "US", "WE", "YE", "YES", "NOR", "OR", "SO", "THE",
	"AND", "BUT", "FOR", "NOT", "YOU", "ALL", "CAN", "HER", "WAS", "ONE",
	"OUR", "OUT", "HAD", "HAS", "HIS", "HIM", "HOW", "ITS", "MAY", "NEW",
	"NOW", "OLD", "SEE", "TWO", "WAY", "WHO", "BOY", "DID", "LET", "MAN",
	"PUT", "SAY", "SHE", "USE", "WAY",
}

func main() {
	var (
		addr     = flag.String("addr", "127.0.0.1:8080", "listen address")
		dictPath = flag.String("dict", "data/enable.txt", "path to dictionary file (.txt or .txt.gz). Falls back to a small built-in list if missing.")
		allowDev = flag.Bool("dev-login", true, "enable POST /api/auth/dev/login for local development")
		noStatic = flag.Bool("no-static", false, "disable serving the embedded frontend")
	)
	flag.Parse()

	// Env-based config (production knobs).
	bucketName := os.Getenv("BUCKET_NAME")
	googleClientID := os.Getenv("GOOGLE_CLIENT_ID")
	sessionSecret := os.Getenv("SESSION_SECRET")
	allowlistEmails := os.Getenv("ALLOWLIST_EMAILS")
	allowlistGCS := os.Getenv("ALLOWLIST_GCS")

	d, err := loadDict(*dictPath)
	if err != nil {
		log.Fatalf("load dictionary: %v", err)
	}
	log.Printf("dictionary loaded: %d words (from %s)", d.Size(), dictSource(*dictPath))

	// --- Store ---
	var st store.Store
	if bucketName != "" {
		client, err := storage.NewClient(context.Background())
		if err != nil {
			log.Fatalf("create GCS client: %v", err)
		}
		st = store.NewGCS(client, bucketName)
		log.Printf("store: GCS bucket %q", bucketName)

		// --- Allowlist (needs GCS client) ---
		if allowlistGCS != "" {
			al, err := httpapi.NewAllowlistGCS(client, allowlistGCS, 5*time.Minute)
			if err != nil {
				log.Fatalf("load GCS allowlist: %v", err)
			}
			_ = al // passed to deps below via env check
		}
	} else {
		st = store.NewMemory()
		log.Printf("store: in-memory (set BUCKET_NAME for GCS)")
	}

	// --- Auth ---
	var auth httpapi.Authenticator
	var googleAuth *httpapi.GoogleAuth
	secureCookie := !*allowDev // Secure cookies in prod

	if googleClientID != "" {
		googleAuth = httpapi.NewGoogleAuth(googleClientID, sessionSecret, secureCookie)
		if *allowDev {
			auth = httpapi.ChainAuth{googleAuth, httpapi.DevAuth{}}
		} else {
			auth = googleAuth
		}
		log.Printf("auth: Google Sign-In (client ID %s…)", googleClientID[:min(len(googleClientID), 16)])
	} else {
		auth = httpapi.DevAuth{}
		log.Printf("auth: dev cookies only (set GOOGLE_CLIENT_ID for Google Sign-In)")
	}

	// --- Push ---
	var notifier *push.Notifier
	vapidPublic := os.Getenv("VAPID_PUBLIC_KEY")
	vapidPrivate := os.Getenv("VAPID_PRIVATE_KEY")
	vapidContact := os.Getenv("VAPID_CONTACT")
	if vapidPublic != "" && vapidPrivate != "" {
		if vapidContact == "" {
			vapidContact = "mailto:admin@example.com"
		}
		notifier = push.NewNotifier(vapidPublic, vapidPrivate, vapidContact)
		log.Printf("push: Web Push enabled")
	} else {
		log.Printf("push: disabled (set VAPID_PUBLIC_KEY + VAPID_PRIVATE_KEY to enable)")
	}

	// --- Allowlist ---
	var allowlist *httpapi.Allowlist
	if allowlistEmails != "" {
		allowlist = httpapi.NewAllowlistInline(allowlistEmails)
		log.Printf("allowlist: %d emails from ALLOWLIST_EMAILS", len(strings.Split(allowlistEmails, ",")))
	} else if allowlistGCS != "" && bucketName != "" {
		client, _ := storage.NewClient(context.Background())
		al, err := httpapi.NewAllowlistGCS(client, allowlistGCS, 5*time.Minute)
		if err != nil {
			log.Fatalf("load GCS allowlist: %v", err)
		}
		allowlist = al
	}

	// --- Static frontend ---
	var staticFS fs.FS
	if !*noStatic {
		staticFS = webdist.FS()
		if staticFS != nil {
			log.Printf("serving embedded frontend")
		} else {
			log.Printf("no embedded frontend assets (run `npm run build` in web/ to populate)")
		}
	}

	srv := httpapi.New(httpapi.Deps{
		Store:         st,
		Dict:          d,
		Auth:          auth,
		GoogleAuth:    googleAuth,
		Allowlist:     allowlist,
		Push:          notifier,
		Now:           time.Now,
		StaticFS:      staticFS,
		AllowDevLogin: *allowDev,
	})

	httpServer := &http.Server{
		Addr:              *addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Graceful shutdown on SIGINT/SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("listening on http://%s", *addr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server: %v", err)
		}
	}()

	<-ctx.Done()
	log.Printf("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}

func loadDict(path string) (*dict.Dictionary, error) {
	if path == "" {
		return dict.FromWords(fallbackWords), nil
	}
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Printf("dictionary %s not found; using small built-in fallback list (pass -dict /path/to/wordlist.txt[.gz] for a full dictionary — see README)", path)
			return dict.FromWords(fallbackWords), nil
		}
		return nil, err
	}
	defer f.Close()

	var r io.Reader = f
	if strings.HasSuffix(path, ".gz") {
		gz, err := gzip.NewReader(f)
		if err != nil {
			return nil, err
		}
		defer gz.Close()
		r = gz
	}
	return dict.LoadReader(bufio.NewReader(r))
}

func dictSource(path string) string {
	if _, err := os.Stat(path); err != nil {
		return "built-in fallback"
	}
	return path
}
