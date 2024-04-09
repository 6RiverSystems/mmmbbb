// Copyright (c) 2021 6 River Systems
//
// Permission is hereby granted, free of charge, to any person obtaining a copy of
// this software and associated documentation files (the "Software"), to deal in
// the Software without restriction, including without limitation the rights to
// use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
// the Software, and to permit persons to whom the Software is furnished to do so,
// subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
// FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
// COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
// IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
// CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

package logging

import (
	"os"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var defaultLoggingOnce = &sync.Once{}

func ConfigureDefaultLogging() {
	defaultLoggingOnce.Do(func() {
		// always log in UTC, with accurate timestamps
		zerolog.TimestampFunc = func() time.Time {
			return time.Now().UTC()
		}
		zerolog.TimeFieldFormat = time.RFC3339Nano
		// NodeJS/bunyan uses "msg" for MessageFieldName, but that's bad for LogDNA,
		// so don't do that here; do make error logging consistent with NodeJS however
		zerolog.ErrorFieldName = "err"

		levelStr := os.Getenv("LOG_LEVEL")
		if levelStr != "" {
			levelStr = strings.ToLower(levelStr)
			level, err := zerolog.ParseLevel(levelStr)
			if err != nil {
				panic(err)
			}
			zerolog.SetGlobalLevel(level)
		} else {
			// default to info logging in production, else debug logging
			if os.Getenv("NODE_ENV") == "production" {
				zerolog.SetGlobalLevel(zerolog.InfoLevel)
			} else {
				zerolog.SetGlobalLevel(zerolog.DebugLevel)
			}
		}

		log.Logger = zerolog.New(os.Stdout).
			With().
			Timestamp().
			Logger()

		log.Info().Msg("Logging initialized")
	})
}
