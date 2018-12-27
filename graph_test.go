package charts

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestJSONParsing(t *testing.T) {
	type testStruct struct {
		Type GraphType
		JSON string
	}

	for i, test := range []testStruct{
		testStruct{GraphTypeTimeseries, "[]"},
		testStruct{GraphTypePie, "[]"},
		testStruct{GraphTypeLine, "[]"},
	} {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			var g Graph
			g.Type = test.Type
			err := g.parseJSONToData(json.RawMessage(test.JSON))
			if err != nil {
				t.Error(err)
			}
		})
	}
}
