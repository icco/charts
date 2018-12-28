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

			// https://cloud.google.com/logging/docs/reference/v2/rest/v2/LogEntry#HttpRequest
			request := map[string]interface{}{
				"requestMethod": r.Method,
				"host":          r.Host,
				"requestUrl":    r.RequestURI,
				"remoteIp":      r.RemoteAddr,
				"referer":       r.Referer(),
				"userAgent":     r.UserAgent(),
			}

			m := httpsnoop.CaptureMetrics(handler, w, r)

			fields["status"] = m.Code
			fields["latency"] = int(m.Duration / time.Millisecond)

			log.WithFields(logrus.Fields{"httpRequest": request}).Info("Completed request")
		})
	}
}
