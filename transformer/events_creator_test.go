// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package transformer

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"testing"

	"github.com/elastic/beats/v7/libbeat/logp"
	"github.com/elastic/cloudbeat/evaluator"

	"github.com/elastic/cloudbeat/resources/fetchers"
	"github.com/elastic/cloudbeat/resources/fetching"
	"github.com/elastic/cloudbeat/resources/manager"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type args struct {
	resource manager.ResourceMap
	metadata CycleMetadata
}

type testAttr struct {
	name    string
	args    args
	wantErr bool
	mocks   []MethodMock
}

type MethodMock struct {
	methodName string
	args       []interface{}
	returnArgs []interface{}
}

const (
	opaResultsFileName = "opa_results.json"
	testIndex          = "test_index"
)

var fetcherResult = fetchers.FileSystemResource{
	Name:    "scheduler.conf",
	Mode:    "700",
	Gid:     20,
	Uid:     501,
	Owner:   "root",
	Group:   "root",
	Path:    "/hostfs/etc/kubernetes/scheduler.conf",
	Inode:   "8901",
	SubType: "file",
}

var (
	opaResults   evaluator.RuleResult
	resourcesMap = map[string][]fetching.Resource{fetchers.FileSystemType: {fetcherResult}}
	ctx          = context.Background()
	cd           = CommonData{
		clusterId: "test-cluster-id",
		nodeId:    "test-node-id",
	}
)

type EventsCreatorTestSuite struct {
	suite.Suite

	log             *logp.Logger
	cycleId         uuid.UUID
	mockedEvaluator evaluator.MockedEvaluator
}

func TestSuite(t *testing.T) {
	s := new(EventsCreatorTestSuite)
	s.log = logp.NewLogger("cloudbeat_events_creator_test_suite")

	if err := logp.TestingSetup(); err != nil {
		t.Error(err)
	}

	suite.Run(t, s)
}

func (s *EventsCreatorTestSuite) SetupSuite() {
	err := parseJsonfile(opaResultsFileName, &opaResults)
	if err != nil {
		s.log.Errorf("Could not parse JSON file: %v", err)
		return
	}
	s.cycleId, _ = uuid.NewV4()
}

func (s *EventsCreatorTestSuite) TestTransformer_ProcessAggregatedResources() {
	var tests = []testAttr{
		{
			name: "All events propagated as expected",
			args: args{
				resource: resourcesMap,
				metadata: CycleMetadata{CycleId: s.cycleId},
			},
			mocks: []MethodMock{{
				methodName: "Decision",
				args:       []interface{}{ctx, mock.AnythingOfType("Result")},
				returnArgs: []interface{}{mock.Anything, nil},
			}, {
				methodName: "Decode",
				args:       []interface{}{mock.Anything},
				returnArgs: []interface{}{opaResults.Findings, nil},
			},
			},
			wantErr: false,
		},
		{
			name: "Events should not be created due to a policy error",
			args: args{
				resource: resourcesMap,
				metadata: CycleMetadata{CycleId: s.cycleId},
			},
			mocks: []MethodMock{{
				methodName: "Decision",
				args:       []interface{}{ctx, mock.AnythingOfType("Result")},
				returnArgs: []interface{}{mock.Anything, errors.New("policy err")},
			}, {
				methodName: "Decode",
				args:       []interface{}{mock.Anything},
				returnArgs: []interface{}{opaResults.Findings, nil},
			},
			},
			wantErr: true,
		},
		{
			name: "Events should not be created due to a parse error",
			args: args{
				resource: resourcesMap,
				metadata: CycleMetadata{CycleId: s.cycleId},
			},
			mocks: []MethodMock{{
				methodName: "Decision",
				args:       []interface{}{ctx, mock.AnythingOfType("Result")},
				returnArgs: []interface{}{mock.Anything, nil},
			}, {
				methodName: "Decode",
				args:       []interface{}{mock.Anything},
				returnArgs: []interface{}{nil, errors.New("parse err")},
			},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			resetMocks(s)
			for _, methodMock := range tt.mocks {
				s.mockedEvaluator.On(methodMock.methodName, methodMock.args...).Return(methodMock.returnArgs...)
			}

			transformer := NewTransformer(ctx, s.log, &s.mockedEvaluator, cd, testIndex)
			generatedEvents := transformer.ProcessAggregatedResources(tt.args.resource, tt.args.metadata)

			if tt.wantErr {
				s.Equal(0, len(generatedEvents))
			} else {
				s.NotEqual(0, len(generatedEvents))
			}

			for _, event := range generatedEvents {
				resource := event.Fields["resource"].(fetching.ResourceFields)
				s.Equal(s.cycleId, event.Fields["cycle_id"], "event cycle_id is not correct")
				s.NotEmpty(event.Timestamp, `event timestamp is missing`)
				s.NotEmpty(event.Fields["result"], "event result is missing")
				s.NotEmpty(event.Fields["rule"], "event rule is missing")
				s.NotEmpty(resource.Raw, "raw resource is missing")
				s.NotEmpty(resource.SubType, "resource sub type is missing")
				s.NotEmpty(resource.ID, "resource ID is missing")
				s.NotEmpty(resource.Type, "resource  type is missing")
				s.NotEmpty(event.Fields["type"], "resource type is missing") // for BC sake
			}
		})
	}
}

func parseJsonfile(filename string, data interface{}) error {
	fetcherDataFile, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer fetcherDataFile.Close()

	byteValue, err := ioutil.ReadAll(fetcherDataFile)
	if err != nil {
		return err
	}

	err = json.Unmarshal(byteValue, data)
	if err != nil {
		return err
	}
	return nil
}

func resetMocks(s *EventsCreatorTestSuite) {
	s.mockedEvaluator = evaluator.MockedEvaluator{}
}
