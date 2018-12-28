package charts

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInitLogging(t *testing.T) {
	logger := InitLogging()

	assert.NotNil(t, log)
	assert.NotNil(t, logger)
	log.Warn("Test the logger!")
	logger.Warn("Test the logger!")
}
