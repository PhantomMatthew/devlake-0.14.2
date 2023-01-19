/*
Licensed to the Apache Software Foundation (ASF) under one or more
contributor license agreements.  See the NOTICE file distributed with
this work for additional information regarding copyright ownership.
The ASF licenses this file to You under the Apache License, Version 2.0
(the "License"); you may not use this file except in compliance with
the License.  You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package impl

import (
	"github.com/apache/incubator-devlake/errors"
	"github.com/apache/incubator-devlake/migration"
	"github.com/apache/incubator-devlake/plugins/bitbucket/api"
	"github.com/apache/incubator-devlake/plugins/bitbucket/models"
	"github.com/apache/incubator-devlake/plugins/bitbucket/models/migrationscripts"
	"github.com/apache/incubator-devlake/plugins/bitbucket/tasks"
	"github.com/apache/incubator-devlake/plugins/core"
	"github.com/apache/incubator-devlake/plugins/helper"
	"github.com/spf13/viper"
	"gorm.io/gorm"
)

var _ core.PluginMeta = (*Bitbucket)(nil)
var _ core.PluginInit = (*Bitbucket)(nil)
var _ core.PluginTask = (*Bitbucket)(nil)
var _ core.PluginApi = (*Bitbucket)(nil)
var _ core.Migratable = (*Bitbucket)(nil)
var _ core.PluginBlueprintV100 = (*Bitbucket)(nil)

type Bitbucket string

func (plugin Bitbucket) Init(config *viper.Viper, logger core.Logger, db *gorm.DB) errors.Error {
	api.Init(config, logger, db)
	return nil
}

func (plugin Bitbucket) Description() string {
	return "To collect and enrich data from Bitbucket"
}

func (plugin Bitbucket) SubTaskMetas() []core.SubTaskMeta {
	return []core.SubTaskMeta{
		tasks.CollectApiRepoMeta,
		tasks.ExtractApiRepoMeta,
		tasks.CollectApiPullRequestsMeta,
		tasks.ExtractApiPullRequestsMeta,
		tasks.CollectApiIssuesMeta,
		tasks.ExtractApiIssuesMeta,
		tasks.CollectApiPrCommentsMeta,
		tasks.ExtractApiPrCommentsMeta,
		tasks.CollectApiIssueCommentsMeta,
		tasks.ExtractApiIssueCommentsMeta,
		tasks.CollectApiPipelinesMeta,
		tasks.ExtractApiPipelinesMeta,
		tasks.ConvertRepoMeta,
		tasks.ConvertAccountsMeta,
		tasks.ConvertPullRequestsMeta,
		tasks.ConvertPrCommentsMeta,
		tasks.ConvertIssuesMeta,
		tasks.ConvertIssueCommentsMeta,
		tasks.ConvertPipelineMeta,
	}
}

func (plugin Bitbucket) PrepareTaskData(taskCtx core.TaskContext, options map[string]interface{}) (interface{}, errors.Error) {
	op, err := tasks.DecodeAndValidateTaskOptions(options)
	if err != nil {
		return nil, err
	}
	connectionHelper := helper.NewConnectionHelper(
		taskCtx,
		nil,
	)
	connection := &models.BitbucketConnection{}
	err = connectionHelper.FirstById(connection, op.ConnectionId)
	if err != nil {
		return nil, errors.Default.Wrap(err, "unable to get bitbucket connection by the given connection ID")
	}

	apiClient, err := tasks.CreateApiClient(taskCtx, connection)
	if err != nil {
		return nil, errors.Default.Wrap(err, "unable to get bitbucket API client instance")
	}

	return &tasks.BitbucketTaskData{
		Options:   op,
		ApiClient: apiClient,
	}, nil
}

func (plugin Bitbucket) RootPkgPath() string {
	return "github.com/apache/incubator-devlake/plugins/bitbucket"
}

func (plugin Bitbucket) MigrationScripts() []migration.Script {
	return migrationscripts.All()
}

func (plugin Bitbucket) MakePipelinePlan(connectionId uint64, scope []*core.BlueprintScopeV100) (core.PipelinePlan, errors.Error) {
	return api.MakePipelinePlan(plugin.SubTaskMetas(), connectionId, scope)
}

func (plugin Bitbucket) ApiResources() map[string]map[string]core.ApiResourceHandler {
	return map[string]map[string]core.ApiResourceHandler{
		"test": {
			"POST": api.TestConnection,
		},
		"connections": {
			"POST": api.PostConnections,
			"GET":  api.ListConnections,
		},
		"connections/:connectionId": {
			"PATCH":  api.PatchConnection,
			"DELETE": api.DeleteConnection,
			"GET":    api.GetConnection,
		},
	}
}
