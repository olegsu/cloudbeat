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

package flavors

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/elastic/cloudbeat/resources/fetchers"
	awscis_logging "github.com/elastic/cloudbeat/resources/providers/aws_cis/logging"
	awscis_monitoring "github.com/elastic/cloudbeat/resources/providers/aws_cis/monitoring"
	"github.com/elastic/cloudbeat/resources/providers/awslib"
	"github.com/elastic/cloudbeat/resources/providers/awslib/cloudtrail"
	"github.com/elastic/cloudbeat/resources/providers/awslib/cloudwatch"
	"github.com/elastic/cloudbeat/resources/providers/awslib/cloudwatch/logs"
	awslib_ec2 "github.com/elastic/cloudbeat/resources/providers/awslib/ec2"
	awslib_iam "github.com/elastic/cloudbeat/resources/providers/awslib/iam"
	awslib_rds "github.com/elastic/cloudbeat/resources/providers/awslib/rds"
	awslib_s3 "github.com/elastic/cloudbeat/resources/providers/awslib/s3"
	"github.com/elastic/cloudbeat/resources/providers/awslib/securityhub"
	"github.com/elastic/cloudbeat/resources/providers/awslib/sns"
	"github.com/elastic/cloudbeat/resources/utils/user"
	"github.com/elastic/elastic-agent-autodiscover/kubernetes"

	"github.com/elastic/cloudbeat/config"
	"github.com/elastic/cloudbeat/dataprovider"
	"github.com/elastic/cloudbeat/evaluator"
	"github.com/elastic/cloudbeat/pipeline"
	_ "github.com/elastic/cloudbeat/processor" // Add cloudbeat default processors.
	"github.com/elastic/cloudbeat/resources/fetchers/ecr"
	"github.com/elastic/cloudbeat/resources/fetchers/eks"
	"github.com/elastic/cloudbeat/resources/fetchers/elb"
	"github.com/elastic/cloudbeat/resources/fetchers/filesystem"
	"github.com/elastic/cloudbeat/resources/fetchers/iam"
	"github.com/elastic/cloudbeat/resources/fetchers/kube"
	"github.com/elastic/cloudbeat/resources/fetchers/logging"
	"github.com/elastic/cloudbeat/resources/fetchers/monitoring"
	"github.com/elastic/cloudbeat/resources/fetchers/network"
	"github.com/elastic/cloudbeat/resources/fetchers/process"
	"github.com/elastic/cloudbeat/resources/fetchers/rds"
	"github.com/elastic/cloudbeat/resources/fetchers/s3"
	"github.com/elastic/cloudbeat/resources/fetchersManager"
	"github.com/elastic/cloudbeat/resources/fetching"
	"github.com/elastic/cloudbeat/resources/providers/awslib/configservice"
	"github.com/elastic/cloudbeat/transformer"
	"github.com/elastic/cloudbeat/uniqueness"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/processors"
	"github.com/elastic/cloudbeat/resources/providers"
	agentconfig "github.com/elastic/elastic-agent-libs/config"
	"github.com/elastic/elastic-agent-libs/logp"

	awssdk_trail "github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	awssdk_cloudwatch "github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	awssdk_cloudwatchlogs "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	awssdk_configservice "github.com/aws/aws-sdk-go-v2/service/configservice"
	awssdk_ec2 "github.com/aws/aws-sdk-go-v2/service/ec2"
	awssdk_iam "github.com/aws/aws-sdk-go-v2/service/iam"
	awssdk_rds "github.com/aws/aws-sdk-go-v2/service/rds"
	awssdk_s3 "github.com/aws/aws-sdk-go-v2/service/s3"
	awssdk_securityhub "github.com/aws/aws-sdk-go-v2/service/securityhub"
	awssdk_sns "github.com/aws/aws-sdk-go-v2/service/sns"
)

// posture configuration.
type posture struct {
	flavorBase
	data       *fetchersManager.Data
	evaluator  evaluator.Evaluator
	resourceCh chan fetching.ResourceInfo
	leader     uniqueness.Manager
	dataStop   fetchersManager.Stop
}

// NewPosture creates an instance of posture.
func NewPosture(_ *beat.Beat, cfg *agentconfig.C) (*posture, error) {
	log := logp.NewLogger("posture")

	ctx, cancel := context.WithCancel(context.Background())

	c, err := config.New(cfg)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	log.Info("Config initiated with cycle period of ", c.Period)

	resourceCh := make(chan fetching.ResourceInfo, resourceChBuffer)

	le := uniqueness.NewLeaderElector(log, c, &providers.KubernetesProvider{})

	fetchers := initFetchers(log, c, resourceCh)
	fetchersRegistry, err := initRegistry(log, c, resourceCh, le, fetchers)
	if err != nil {
		cancel()
		return nil, err
	}

	// TODO: timeout should be configurable and not hard-coded. Setting to 10 minutes for now to account for CSPM fetchers
	// 	https://github.com/elastic/cloudbeat/issues/653
	data, err := fetchersManager.NewData(log, c.Period, time.Minute*10, fetchersRegistry)
	if err != nil {
		cancel()
		return nil, err
	}

	eval, err := evaluator.NewOpaEvaluator(ctx, log, c)
	if err != nil {
		cancel()
		return nil, err
	}

	// namespace will be passed as param from fleet on https://github.com/elastic/security-team/issues/2383 and it's user configurable
	resultsIndex := config.Datastream("", config.ResultsDatastreamIndexPrefix)

	commonDataProvider := dataprovider.NewCommonDataProvider(log, c)
	commonData, err := commonDataProvider.FetchCommonData(ctx)
	if err != nil {
		log.Errorf("could not get common data from common data providers. Error: %v", err)
		cancel()
		return nil, err
	}

	t := transformer.NewTransformer(log, commonData, resultsIndex)

	base := flavorBase{
		ctx:         ctx,
		cancel:      cancel,
		config:      c,
		transformer: t,
		log:         log,
	}

	bt := &posture{
		flavorBase: base,
		evaluator:  eval,
		data:       data,
		resourceCh: resourceCh,
		leader:     le,
	}
	return bt, nil
}

// Run starts posture.
func (bt *posture) Run(b *beat.Beat) error {
	bt.log.Info("posture is running! Hit CTRL-C to stop it")

	if err := bt.leader.Run(bt.ctx); err != nil {
		return err
	}

	bt.dataStop = bt.data.Run(bt.ctx)

	procs, err := bt.configureProcessors(bt.config.Processors)
	if err != nil {
		return err
	}
	bt.log.Debugf("posture configured %d processors", len(bt.config.Processors))

	// Connect publisher (with beat's processors)
	if bt.client, err = b.Publisher.ConnectWith(beat.ClientConfig{
		Processing: beat.ProcessingConfig{
			Processor: procs,
		},
	}); err != nil {
		return err
	}

	// Creating the data pipeline
	findingsCh := pipeline.Step(bt.log, bt.resourceCh, bt.evaluator.Eval)
	eventsCh := pipeline.Step(bt.log, findingsCh, bt.transformer.CreateBeatEvents)

	var eventsToSend []beat.Event
	ticker := time.NewTicker(flushInterval)
	for {
		select {
		case <-bt.ctx.Done():
			bt.log.Warn("Posture context is done")
			return nil

		// Flush events to ES after a pre-defined interval, meant to clean residuals after a cycle is finished.
		case <-ticker.C:
			if len(eventsToSend) == 0 {
				continue
			}

			bt.log.Infof("Publishing %d posture events to elasticsearch, time interval reached", len(eventsToSend))
			bt.client.PublishAll(eventsToSend)
			eventsToSend = nil

		// Flush events to ES when reaching a certain threshold
		case events := <-eventsCh:
			eventsToSend = append(eventsToSend, events...)
			if len(eventsToSend) < eventsThreshold {
				continue
			}

			bt.log.Infof("Publishing %d posture events to elasticsearch, buffer threshold reached", len(eventsToSend))
			bt.client.PublishAll(eventsToSend)
			eventsToSend = nil
		}
	}
}

func initRegistry(log *logp.Logger, cfg *config.Config, ch chan fetching.ResourceInfo, le uniqueness.Manager, fetchers map[string]fetchers.Fetcher) (fetchersManager.FetchersRegistry, error) {
	registry := fetchersManager.NewFetcherRegistry(log)

	list, err := fetchersManager.ParseConfigFetchers(log, cfg, ch, fetchers)
	if err != nil {
		return nil, err
	}

	if err := registry.RegisterFetchers(list, le); err != nil {
		return nil, err
	}

	return registry, nil
}

func initFetchers(log *logp.Logger, cfg *config.Config, ch chan fetching.ResourceInfo) map[string]fetchers.Fetcher {
	k8sProvider := providers.KubernetesProvider{}
	// TODO: load kubeconfig
	k8sClient, err := k8sProvider.GetClient(log, "", kubernetes.KubeClientOptions{})
	if err != nil {
		panic(err)
	}

	awsConfigProvider := awslib.ConfigProvider{MetadataProvider: awslib.Ec2MetadataProvider{}}
	awsConfig, err := awsConfigProvider.InitializeAWSConfig(context.Background(), cfg.CloudConfig.AwsCred)
	if awsConfig.Region == "" {
		// TODO: do we really need this?
		awsConfig.Region = awslib.DefaultRegion
	}
	if err != nil {
		panic(err)
	}

	identityProvider := awslib.GetIdentityClient(*awsConfig)
	identity, err := identityProvider.GetIdentity(context.Background())
	if err != nil {
		panic(err)
	}

	crossRegionCloudtrailProvider := awslib.MultiRegionClientFactory[cloudtrail.Client]{}
	crossRegionS3Provider := awslib.MultiRegionClientFactory[awslib_s3.Client]{}
	crossRegionSecurityhubProvider := awslib.MultiRegionClientFactory[securityhub.Client]{}
	crossRegionCloudwatchProvider := awslib.MultiRegionClientFactory[cloudwatch.Client]{}
	crossRegionCloudwatchlogsProvider := awslib.MultiRegionClientFactory[logs.Client]{}
	crossRegionSNSProvider := awslib.MultiRegionClientFactory[sns.Client]{}
	crossRegionEC2Provider := awslib.MultiRegionClientFactory[awslib_ec2.Client]{}
	crossRegionConfigserviceProvider := awslib.MultiRegionClientFactory[configservice.Client]{}
	crossRegionRDSProvider := awslib.MultiRegionClientFactory[awslib_rds.Client]{}

	list, err := config.GetFetcherNames(cfg) // get all the fetchers from the config file to register only them
	if err != nil {
		panic(err)
	}

	awsEC2Service := awssdk_ec2.NewFromConfig(*awsConfig)
	awsIAMService := awssdk_iam.NewFromConfig(*awsConfig)
	awsS3MultiRegionService := crossRegionS3Provider.NewMultiRegionClients(awsEC2Service, *awsConfig, func(cfg awssdk.Config) awslib_s3.Client {
		return awssdk_s3.NewFromConfig(cfg)
	}, log).GetMultiRegionsClientMap()
	awsTrailMultiRegionService := crossRegionCloudtrailProvider.NewMultiRegionClients(awsEC2Service, *awsConfig, func(cfg awssdk.Config) cloudtrail.Client {
		return awssdk_trail.NewFromConfig(cfg)
	}, log).GetMultiRegionsClientMap()
	awsSecurityhubMultiRegionService := crossRegionSecurityhubProvider.NewMultiRegionClients(awsEC2Service, *awsConfig, func(cfg awssdk.Config) securityhub.Client {
		return awssdk_securityhub.NewFromConfig(cfg)
	}, log).GetMultiRegionsClientMap()
	awsCloudwatchMultiRegionService := crossRegionCloudwatchProvider.NewMultiRegionClients(awsEC2Service, *awsConfig, func(cfg awssdk.Config) cloudwatch.Client {
		return awssdk_cloudwatch.NewFromConfig(cfg)
	}, log).GetMultiRegionsClientMap()
	awsCloudwatchlogsMultiRegionService := crossRegionCloudwatchlogsProvider.NewMultiRegionClients(awsEC2Service, *awsConfig, func(cfg awssdk.Config) logs.Client {
		return awssdk_cloudwatchlogs.NewFromConfig(cfg)
	}, log).GetMultiRegionsClientMap()
	awsSNSMultiRegionService := crossRegionSNSProvider.NewMultiRegionClients(awsEC2Service, *awsConfig, func(cfg awssdk.Config) sns.Client {
		return awssdk_sns.NewFromConfig(cfg)
	}, log).GetMultiRegionsClientMap()
	awsEC2MultiRegionService := crossRegionEC2Provider.
		NewMultiRegionClients(ec2.NewFromConfig(*awsConfig), *awsConfig, func(cfg awssdk.Config) awslib_ec2.Client {
			return awssdk_ec2.NewFromConfig(cfg)
		}, log).
		GetMultiRegionsClientMap()
	awsConfigserviceMultiRegionService := crossRegionConfigserviceProvider.NewMultiRegionClients(awsEC2Service, *awsConfig, func(cfg awssdk.Config) configservice.Client {
		return awssdk_configservice.NewFromConfig(cfg)
	}, log).GetMultiRegionsClientMap()
	awsRDSMultiRegionService := crossRegionRDSProvider.NewMultiRegionClients(awsEC2Service, *awsConfig, func(cfg awssdk.Config) awslib_rds.Client {
		return awssdk_rds.NewFromConfig(cfg)
	}, log).GetMultiRegionsClientMap()

	// 12/12 fetchers
	// also in case there is a fetcher in yml that is not recognized should we exit?
	reg := map[string]fetchers.Fetcher{}
	if _, ok := list[fetching.EcrType]; ok {
		reg[fetching.EcrType] = ecr.NewFetcher(
			ecr.WithLogger(log),
			ecr.WithFetcherConfig(cfg),
			ecr.WithKubeClient(k8sClient),
			ecr.WithAWSConfig(*awssdk.NewConfig()),
			ecr.WithPodDescriber(*identity.Account, awslib.NewEcrProvider()),
			ecr.WithResourceChannel(ch),
		)
	}

	if _, ok := list[fetching.EksType]; ok {
		reg[fetching.EksType] = eks.NewFetcher(
			eks.WithLogger(log),
			eks.WithResourceChannel(ch),
			eks.WithFetcherConfig(cfg),
			eks.WithEKSProvider(awslib.NewEksProvider(*awsConfig)),
		)
	}

	if _, ok := list[fetching.ElbType]; ok {
		reg[fetching.ElbType] = elb.NewFetcher(
			elb.WithLogger(log),
			elb.WithResourceChannel(ch),
			elb.WithCloudIdentity(identity),
			elb.WithFetcherConfig(cfg),
			elb.WithRegexMatcher(awsConfig.Region),
			elb.WithKubeClient(k8sClient),
		)
	}

	if _, ok := list[fetching.FileSystemType]; ok {
		reg[fetching.FileSystemType] = filesystem.NewFetcher(
			filesystem.WithLogger(log),
			filesystem.WithResourceChannel(ch),
			filesystem.WithFetcherConfig(cfg),
			filesystem.WithUserProvider(user.NewOSUserUtil()),
		)
	}

	if _, ok := list[fetching.IAMType]; ok {
		reg[fetching.IAMType] = iam.NewFetcher(
			iam.WithLogger(log),
			iam.WithResourceChannel(ch),
			iam.WithFetcherConfig(cfg),
			iam.WithCloudIdentity(identity),
			iam.WithIAMProvider(awslib_iam.NewIAMProvider(log, awsIAMService)),
		)
	}

	if _, ok := list[fetching.KubeAPIType]; ok {
		reg[fetching.KubeAPIType] = kube.NewFetcher(
			kube.WithLogger(log),
			kube.WithResourceChannel(ch),
			kube.WithWatchers(kube.InitWatchers(log, k8sClient)),
			kube.WtihClientProvider(k8sClient),
		)
	}

	if _, ok := list[fetching.TrailType]; ok {
		reg[fetching.TrailType] = logging.NewFetcher(
			logging.WithLogger(log),
			logging.WithResourceChannel(ch),
			logging.WithLoggingProvider(
				awscis_logging.NewProvider(log, *awsConfig, awsTrailMultiRegionService, awsS3MultiRegionService),
			),
			logging.WithConfigserviceProvider(
				configservice.NewProvider(log, awsConfigserviceMultiRegionService, *identity.Account),
			),
		)
	}

	if _, ok := list[fetching.MonitoringType]; ok {
		reg[fetching.MonitoringType] = monitoring.NewFetcher(
			monitoring.WithLogger(log),
			monitoring.WithResourceChannel(ch),
			monitoring.WithFetcherConfig(cfg),
			monitoring.WithCloudIdentity(identity),
			monitoring.WithSecurityhubProvider(securityhub.NewProvider(*awsConfig, log, awsSecurityhubMultiRegionService)),
			monitoring.WithMonitoringProvider(&awscis_monitoring.Provider{
				Cloudtrail:     cloudtrail.NewProvider(log, awsTrailMultiRegionService), // TODO: make the same order of params as in cloudwatch
				Cloudwatch:     cloudwatch.NewProvider(log, *awsConfig, awsCloudwatchMultiRegionService),
				Cloudwatchlogs: logs.NewCloudwatchLogsProvider(log, awsCloudwatchlogsMultiRegionService),
				Sns:            sns.NewSNSProvider(log, awsSNSMultiRegionService),
				Log:            log,
			}),
		)
	}

	if _, ok := list[fetching.EC2NetworkingType]; ok {
		reg[fetching.EC2NetworkingType] = network.NewFetcher(
			network.WithLogger(log),
			network.WithResourceChannel(ch),
			network.WithFetcherConfig(cfg),
			network.WithCloudIdentity(identity),
			network.WithEC2Clients(awslib_ec2.NewEC2Provider(log, *identity.Account, awsEC2MultiRegionService)),
		)
	}

	if _, ok := list[fetching.ProcessType]; ok {
		reg[fetching.ProcessType] = process.NewFetcher(
			process.WithLogger(log),
			process.WithResourceChannel(ch),
			process.WithFetcherConfig(cfg),
			process.WithFSProvider(func(dir string) fs.FS { return os.DirFS(dir) }),
		)
	}

	if _, ok := list[fetching.S3Type]; ok {
		reg[fetching.S3Type] = s3.NewFetcher(
			s3.WithLogger(log),
			s3.WithResourceChannel(ch),
			s3.WithFetcherConfig(cfg),
			s3.WithS3Client(awslib_s3.NewProvider(log, awsS3MultiRegionService)),
		)
	}

	if _, ok := list[fetching.RdsType]; ok {
		reg[fetching.RdsType] = rds.NewFetcher(
			rds.WithLogger(log),
			rds.WithResourceChannel(ch),
			rds.WithFetcherConfig(cfg),
			rds.WithRdsProvider(awslib_rds.NewProvider(log, awsRDSMultiRegionService)),
		)
	}

	return reg
}

// Stop stops cloudbeat.
func (bt *posture) Stop() {
	if bt.dataStop != nil {
		bt.dataStop(bt.ctx, shutdownGracePeriod)
	}
	bt.evaluator.Stop(bt.ctx)
	bt.leader.Stop()
	close(bt.resourceCh)
	if err := bt.client.Close(); err != nil {
		bt.log.Fatal("Cannot close client", err)
	}

	bt.cancel()
}

// configureProcessors configure processors to be used by the beat
func (bt *posture) configureProcessors(processorsList processors.PluginConfig) (procs *processors.Processors, err error) {
	return processors.New(processorsList)
}
