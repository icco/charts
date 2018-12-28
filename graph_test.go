package charts

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestValidJSONParsing(t *testing.T) {
	type testStruct struct {
		Type GraphType
		JSON string
	}

	InitLogging()

	for i, test := range []testStruct{
		testStruct{GraphTypeTimeseries, "[]"},
		testStruct{GraphTypePie, "[]"},
		testStruct{GraphTypeLine, "[]"},
		testStruct{GraphTypeTimeseries, `[{"timestamp": "2012-04-23T18:25:43.511Z", "value": 56}]`},
		testStruct{GraphTypePie, `[{"percent": 50}]`},
		testStruct{GraphTypeLine, `[{"x": 1, "y": 1}]`},
		testStruct{GraphTypeTimeseries, `[{"timestamp": "2012-04-23T18:25:43.511Z", "value": 56.2}]`},
		testStruct{GraphTypePie, `[{"percent": 50.29}]`},
		testStruct{GraphTypeLine, `[{"x": 1.6, "y": 1.5}]`},
		testStruct{GraphTypeTimeseries, `[{}]`},
		testStruct{GraphTypePie, `[{}]`},
		testStruct{GraphTypeLine, `[{}]`},
		testStruct{GraphTypeLine, `[{"meta": []}]`},
		testStruct{GraphTypeLine, `[{"meta": [{"key": "test", "value": "blargh"}]}]`},
	} {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			var g Graph
			g.Type = test.Type
			err := g.parseJSONToData(json.RawMessage(test.JSON))
			if err != nil {
				t.Error(err)
			}

			if g.Data == nil {
				t.Error("Data is empty!")
			}
		})
	}
}
