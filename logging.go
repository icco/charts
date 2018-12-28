package charts

import (
	"net/http"
	"net/url"
	"time"

	stackdriver "github.com/TV4/logrus-stackdriver-formatter"
	"github.com/felixge/httpsnoop"
	"github.com/hellofresh/logging-go/context"
	"github.com/sirupsen/logrus"
)

var log = logrus.New()

// InitLogging initializes a logger to send things to stackdriver.
func InitLogging() *logrus.Logger {
	log.Formatter = stackdriver.NewFormatter(
		stackdriver.WithService("charts"),
	)
	log.Level = logrus.DebugLevel

	log.Info("Logger successfully initialised!")

	return log
}

// LoggingMiddleware is a middleware for writing request logs in a stuctured
// format to stackdriver.
func LoggingMiddleware() func(http.Handler) http.Handler {
	return func(handler http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r = r.WithContext(context.New(r.Context()))
			log.WithFields(logrus.Fields{"method": r.Method, "path": r.URL.Path}).Debug("Started request")

			// reverse proxy replaces original request with target request, so keep original one
			originalURL := &url.URL{}
			*originalURL = *r.URL

			fields := logrus.Fields{
				"method":      r.Method,
				"host":        r.Host,
				"request":     r.RequestURI,
				"remote-addr": r.RemoteAddr,
				"referer":     r.Referer(),
				"user-agent":  r.UserAgent(),
			}

			m := httpsnoop.CaptureMetrics(handler, w, r)

			fields["code"] = m.Code
			fields["duration"] = int(m.Duration / time.Millisecond)
			fields["duration-fmt"] = m.Duration.String()

			if originalURL.String() != r.URL.String() {
				fields["upstream-host"] = r.URL.Host
				fields["upstream-request"] = r.URL.RequestURI()
			}

			log.WithFields(fields).Info("Completed handling request")
		})
	}
}
