package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/icco/charts"
	"github.com/stretchr/testify/assert"
)

func doRequest(handler http.Handler, method string, target string, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, target, strings.NewReader(body))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)
	return w
}

func TestGraphQLPOST(t *testing.T) {
	_, err := charts.InitDB(dbURL)
	if err != nil {
		t.Errorf("Init DB: %+v", err)
	}
	h := buildGraphQLHandler()

	t.Run("get a graph", func(t *testing.T) {
		query := `
    {"operationName":null,"variables":{},"query":"mutation {\n  createLineGraph(input: {description: \"\", data: [{x: 1.0, y: 1.0}, {x: 2.0, y: 4.0}, {x: 3.0, y: 6.0}, {x: 4.0, y: 8.0}, {x: 5.0, y: 10.0}]}) {\n    id\n  }\n}\n"}
    `

		resp := doRequest(h, "POST", "/graphql", query)
		assert.Equal(t, http.StatusOK, resp.Code)
		assert.Equal(t, `{"data":{"name":"test"}}`, resp.Body.String())
	})

	t.Run("create a line graph", func(t *testing.T) {
		query := `
    {"operationName":null,"variables":{},"query":"{\n  getGraph(id: \"bc5f70dd-d408-49cc-8949-5476b9af1e3f\") {\n    id\n    data {\n      ... on PairPoint {\n        x\n        y\n        meta {\n          key\n          value\n        }\n      }\n    }\n  }\n}\n"}
    `

		resp := doRequest(h, "POST", "/graphql", query)
		assert.Equal(t, http.StatusOK, resp.Code)
		assert.Equal(t, `{"data":{"name":"test"}}`, resp.Body.String())
	})
}
