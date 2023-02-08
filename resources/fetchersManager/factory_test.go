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

package fetchersManager

import (
	"context"
	"testing"

	"github.com/elastic/elastic-agent-libs/logp"

	"github.com/elastic/cloudbeat/resources/fetching"
	agentconfig "github.com/elastic/elastic-agent-libs/config"
	"github.com/stretchr/testify/suite"
)

type syncNumberFetcher struct {
	num        int
	stopCalled bool
	resourceCh chan fetching.ResourceInfo
}

func newSyncNumberFetcher(num int, ch chan fetching.ResourceInfo) fetching.Fetcher {
	return &syncNumberFetcher{num, false, ch}
}

func (f *syncNumberFetcher) Fetch(ctx context.Context, cMetadata fetching.CycleMetadata) error {
	f.resourceCh <- fetching.ResourceInfo{
		Resource:      fetchValue(f.num),
		CycleMetadata: cMetadata,
	}

	return nil
}

func (f *syncNumberFetcher) Stop() {
	f.stopCalled = true
}

type FactoriesTestSuite struct {
	suite.Suite

	log        *logp.Logger
	F          factories
	resourceCh chan fetching.ResourceInfo
}

type numberFetcherFactory struct{}

func (n *numberFetcherFactory) Create(log *logp.Logger, c *agentconfig.C, ch chan fetching.ResourceInfo) (fetching.Fetcher, error) {
	x, _ := c.Int("num", -1)
	return &syncNumberFetcher{int(x), false, ch}, nil
}

func numberConfig(number int) *agentconfig.C {
	c := agentconfig.NewConfig()
	err := c.SetInt("num", -1, int64(number))
	if err != nil {
		logp.L().Errorf("Could not set number config: %v", err)
		return nil
	}
	return c
}

func TestFactoriesTestSuite(t *testing.T) {
	s := new(FactoriesTestSuite)
	s.log = logp.NewLogger("cloudbeat_factories_test_suite")

	if err := logp.TestingSetup(); err != nil {
		t.Error(err)
	}

	suite.Run(t, s)
}

func (s *FactoriesTestSuite) SetupTest() {
	s.F = New()
	s.resourceCh = make(chan fetching.ResourceInfo, 50)
}

func (s *FactoriesTestSuite) TearDownTest() {
	close(s.resourceCh)
}

func (s *FactoriesTestSuite) TestListFetcher() {
	tests := []struct {
		key string
	}{
		{"process"},
		{"file-system"},
	}

	for _, test := range tests {
		s.F.RegisterFactory(test.key, &numberFetcherFactory{})
	}

	s.Contains(s.F.m, "process")
	s.Contains(s.F.m, "file-system")
}

func (s *FactoriesTestSuite) TestCreateFetcherCollision() {
	tests := []struct {
		key string
	}{
		{"process"},
		{"process"},
	}

	s.Panics(func() {
		for _, test := range tests {
			s.F.RegisterFactory(test.key, &numberFetcherFactory{})
		}
	})
}
