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

// Code generated by mockery v2.15.0. DO NOT EDIT.

package securityhub

import (
	context "context"

	servicesecurityhub "github.com/aws/aws-sdk-go-v2/service/securityhub"
	mock "github.com/stretchr/testify/mock"
)

// MockClient is an autogenerated mock type for the Client type
type MockClient struct {
	mock.Mock
}

type MockClient_Expecter struct {
	mock *mock.Mock
}

func (_m *MockClient) EXPECT() *MockClient_Expecter {
	return &MockClient_Expecter{mock: &_m.Mock}
}

// DescribeHub provides a mock function with given fields: ctx, params, optFns
func (_m *MockClient) DescribeHub(ctx context.Context, params *servicesecurityhub.DescribeHubInput, optFns ...func(*servicesecurityhub.Options)) (*servicesecurityhub.DescribeHubOutput, error) {
	_va := make([]interface{}, len(optFns))
	for _i := range optFns {
		_va[_i] = optFns[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, params)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 *servicesecurityhub.DescribeHubOutput
	if rf, ok := ret.Get(0).(func(context.Context, *servicesecurityhub.DescribeHubInput, ...func(*servicesecurityhub.Options)) *servicesecurityhub.DescribeHubOutput); ok {
		r0 = rf(ctx, params, optFns...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*servicesecurityhub.DescribeHubOutput)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *servicesecurityhub.DescribeHubInput, ...func(*servicesecurityhub.Options)) error); ok {
		r1 = rf(ctx, params, optFns...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockClient_DescribeHub_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'DescribeHub'
type MockClient_DescribeHub_Call struct {
	*mock.Call
}

// DescribeHub is a helper method to define mock.On call
//   - ctx context.Context
//   - params *servicesecurityhub.DescribeHubInput
//   - optFns ...func(*servicesecurityhub.Options)
func (_e *MockClient_Expecter) DescribeHub(ctx interface{}, params interface{}, optFns ...interface{}) *MockClient_DescribeHub_Call {
	return &MockClient_DescribeHub_Call{Call: _e.mock.On("DescribeHub",
		append([]interface{}{ctx, params}, optFns...)...)}
}

func (_c *MockClient_DescribeHub_Call) Run(run func(ctx context.Context, params *servicesecurityhub.DescribeHubInput, optFns ...func(*servicesecurityhub.Options))) *MockClient_DescribeHub_Call {
	_c.Call.Run(func(args mock.Arguments) {
		variadicArgs := make([]func(*servicesecurityhub.Options), len(args)-2)
		for i, a := range args[2:] {
			if a != nil {
				variadicArgs[i] = a.(func(*servicesecurityhub.Options))
			}
		}
		run(args[0].(context.Context), args[1].(*servicesecurityhub.DescribeHubInput), variadicArgs...)
	})
	return _c
}

func (_c *MockClient_DescribeHub_Call) Return(_a0 *servicesecurityhub.DescribeHubOutput, _a1 error) *MockClient_DescribeHub_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

type mockConstructorTestingTNewMockClient interface {
	mock.TestingT
	Cleanup(func())
}

// NewMockClient creates a new instance of MockClient. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewMockClient(t mockConstructorTestingTNewMockClient) *MockClient {
	mock := &MockClient{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
