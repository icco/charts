package charts

import (
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/felixge/httpsnoop"
	"github.com/hellofresh/logging-go/context"
	stackdriver "github.com/icco/logrus-stackdriver-formatter"
	"github.com/sirupsen/logrus"
)

var log = logrus.New()

// InitLogging initializes a logger to send things to stackdriver.
func InitLogging() *logrus.Logger {
	log.Formatter = stackdriver.NewFormatter()
	log.Level = logrus.DebugLevel
	log.SetOutput(os.Stdout)

	log.Info("Logger successfully initialised!")

	return log
}

// LoggingMiddleware is a middleware for writing request logs in a stuctured
// format to stackdriver.
func LoggingMiddleware() func(http.Handler) http.Handler {
	return func(handler http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r = r.WithContext(context.New(r.Context()))

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

			log.WithFields(logrus.Fields{"httpRequest": fields}).Info("Completed request")
		})
	}
}
