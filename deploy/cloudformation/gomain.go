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

// Config is put into a different package to prevent cyclic imports in case
// it is needed in several locations

package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/mikefarah/yq/v4/pkg/yqlib"
)

const (
	DEV  = "DEV_TEMPLATE"
	PROD = "PROD_TEMPLATE"
)

var templatePaths = map[string]map[string]string{
	DeploymentTypeCSPM: {
		DEV:  "elastic-agent-ec2-dev-cspm.yml",
		PROD: "elastic-agent-ec2-cspm.yml",
	},
	DeploymentTypeCNVM: {
		DEV:  "elastic-agent-ec2-dev-cnvm.yml",
		PROD: "elastic-agent-ec2-cnvm.yml",
	},
}

func main() {
	cfg, err := parseConfig()
	if err != nil {
		log.Fatal(err)
	}

	err = createFromConfig(cfg)
	if err != nil {
		log.Fatal(err)
	}
}

func createFromConfig(cfg *config) error {
	params := map[string]string{}

	params["FleetUrl"] = cfg.FleetURL
	params["EnrollmentToken"] = cfg.EnrollmentToken
	params["ElasticAgentVersion"] = cfg.ElasticAgentVersion

	if cfg.ElasticArtifactServer != nil {
		params["ElasticArtifactServer"] = *cfg.ElasticArtifactServer
	}

	templatePath := getTemplatePath(cfg.DeploymentType, PROD)

	if cfg.Dev != nil && cfg.Dev.AllowSSH {
		params["KeyName"] = cfg.Dev.KeyName

		devTemplatePath := getTemplatePath(cfg.DeploymentType, DEV)

		err := generateDevTemplate(templatePath, devTemplatePath)
		if err != nil {
			return fmt.Errorf("could not generate dev template: %v", err)
		}

		templatePath = devTemplatePath
	}

	err := createStack(cfg.StackName, templatePath, params)
	if err != nil {
		return fmt.Errorf("failed to create CloudFormation stack: %v", err)
	}

	return nil
}

func generateDevTemplate(prodTemplatePath string, devTemplatePath string) (err error) {
	const yqExpression = `
.Parameters.KeyName = {
	"Description": "SSH Keypair to login to the instance",
	"Type": "AWS::EC2::KeyPair::KeyName"
} |
.Resources.ElasticAgentEc2Instance.Properties.KeyName = { "Ref": "KeyName" } |
.Resources.ElasticAgentSecurityGroup.Properties.GroupDescription = "Allow SSH from anywhere" |
.Resources.ElasticAgentSecurityGroup.Properties.SecurityGroupIngress += {
	"CidrIp": "0.0.0.0/0",
	"FromPort": 22,
	"IpProtocol": "tcp",
	"ToPort": 22
}
`
	inputBytes, err := os.ReadFile(prodTemplatePath)
	if err != nil {
		return err
	}

	generatedTemplateString, err := yqlib.NewStringEvaluator().Evaluate(
		yqExpression,
		string(inputBytes),
		yqlib.NewYamlEncoder(2, false, yqlib.NewDefaultYamlPreferences()),
		yqlib.NewYamlDecoder(yqlib.NewDefaultYamlPreferences()),
	)
	if err != nil {
		return err
	}

	f, err := os.Create(devTemplatePath)
	if err != nil {
		return err
	}
	defer func(f *os.File) {
		closeErr := f.Close()
		if closeErr != nil && err == nil {
			err = fmt.Errorf("failed to close file: %w", closeErr)
		}
	}(f)

	_, err = f.WriteString(generatedTemplateString)
	if err != nil {
		return fmt.Errorf("failed to write to dev template: %w", err)
	}

	return
}

func createStack(stackName string, templatePath string, params map[string]string) error {
	ctx := context.Background()

	cfg, err := awsConfig.LoadDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to load AWS SDK config: %v", err)
	}

	svc := cloudformation.NewFromConfig(cfg)
	cfParams := make([]types.Parameter, 0, len(params))
	for key, value := range params {
		p := types.Parameter{
			ParameterKey:   aws.String(key),
			ParameterValue: aws.String(value),
		}
		cfParams = append(cfParams, p)
	}

	bodyBytes, err := os.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("failed to open template file: %v", err)
	}

	createStackInput := &cloudformation.CreateStackInput{
		StackName:    &stackName,
		TemplateBody: aws.String(string(bodyBytes)),
		Parameters:   cfParams,
		Capabilities: []types.Capability{types.CapabilityCapabilityNamedIam},
	}

	stackOutput, err := svc.CreateStack(ctx, createStackInput)
	if err != nil {
		return fmt.Errorf("failed to call AWS CloudFormation CreateStack: %v", err)
	}

	log.Printf("Created stack %s", *stackOutput.StackId)
	return nil
}

func getTemplatePath(deploymentType string, env string) string {
	if deploymentType == "" {
		// Default is CNVM
		deploymentType = DeploymentTypeCNVM
	}
	return templatePaths[deploymentType][env]
}
