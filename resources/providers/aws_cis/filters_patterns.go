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

package aws_cis

// UnauthorizedAPICallsMetric it is a filter to monitor the API calls that are not authorized and/or denied.
const UnauthorizedAPICallsPattern = "{ ($.errorCode = \"UnauthorizedOperation\") || ($.errorCode = \"AccessDenied\") || ($.sourceIPAddress!=\"delivery.logs.amazonaws.com\") || ($.eventName!=\"HeadBucket\") }"

const (
	// NonMFAConsoleLoginPattern it is a filter to monitor the console login attempts that are done without using Multi-factor authentication.
	NonMFAConsoleLoginPattern = "{ ($.eventName = \"ConsoleLogin\") && ($.additionalEventData.MFAUsed != \"Yes\") }"
	// SSONonMFAConsoleLoginPattern it is a filter to monitor the successful console login attempts that are done by an IAM user without using Multi-factor authentication.
	SSONonMFAConsoleLoginPattern = "{ ($.eventName = \"ConsoleLogin\") && ($.additionalEventData.MFAUsed != \"Yes\") && ($.userIdentity.type = \"IAMUser\") && ($.responseElements.ConsoleLogin = \"Success\") }"
)

// TODO: check opa
