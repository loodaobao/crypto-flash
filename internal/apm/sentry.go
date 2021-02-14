package apm

import (
	"log"

	"github.com/getsentry/sentry-go"
)

func InitSentryService(sentryDsn string) {
	// SENTRY_DSN
	err := sentry.Init(sentry.ClientOptions{
		Dsn: sentryDsn,
	})

	if err != nil {
		log.Fatalf("sentry.Init: %s", err)
	}
}
