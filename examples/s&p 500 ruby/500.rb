#! /usr/bin/env ruby

require "graphql/client"
require "graphql/client/http"
require "json"
require "chronic"

module Chartopia
  HTTP = GraphQL::Client::HTTP.new("https://chartopia.app/graphql")
  Schema = GraphQL::Client.load_schema(HTTP)
  Client = GraphQL::Client.new(schema: Schema, execute: HTTP)
end

data = JSON.parse(File.read("sp-500-data.json")).map do |r|
  # Chartopia requires a iso8601 time format.
  r["timestamp"] = Chronic.parse(r["timestamp"]).iso8601
  r
end


rawQuery = %{
  mutation($data: [TimePointInput]!) {
    createTimeseriesGraph(input:{
      description:"S&P 500 Life"
      data: $data
    }) {
      id
      url
    }
  }
}

CreateQuery = Chartopia::Client.parse rawQuery
result = Chartopia::Client.query(CreateQuery, variables: {data: data})
p result
