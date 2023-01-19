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

package services

import (
	"encoding/json"
	goerror "errors"
	"fmt"
	"github.com/apache/incubator-devlake/errors"
	"strings"

	"github.com/apache/incubator-devlake/logger"
	"github.com/apache/incubator-devlake/models"
	"github.com/apache/incubator-devlake/plugins/core"
	"github.com/apache/incubator-devlake/plugins/helper"
	"github.com/go-playground/validator/v10"
	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

// BlueprintQuery FIXME ...
type BlueprintQuery struct {
	Enable   *bool `form:"enable,omitempty"`
	Page     int   `form:"page"`
	PageSize int   `form:"pageSize"`
}

var (
	blueprintLog = logger.Global.Nested("blueprint")
	vld          = validator.New()
)

// CreateBlueprint accepts a Blueprint instance and insert it to database
func CreateBlueprint(blueprint *models.Blueprint) errors.Error {
	err := validateBlueprint(blueprint)
	if err != nil {
		return err
	}
	dbBlueprint, err := encryptDbBlueprint(parseDbBlueprint(blueprint))
	if err != nil {
		return err
	}
	err = CreateDbBlueprint(dbBlueprint)
	if err != nil {
		return err
	}
	blueprint.Model = dbBlueprint.Model
	err = ReloadBlueprints(cronManager)
	if err != nil {
		return errors.Internal.Wrap(err, "error reloading blueprints")
	}
	return nil
}

// GetBlueprints returns a paginated list of Blueprints based on `query`
func GetBlueprints(query *BlueprintQuery) ([]*models.Blueprint, int64, errors.Error) {
	dbBlueprints, count, err := GetDbBlueprints(query)
	if err != nil {
		return nil, 0, errors.Convert(err)
	}
	blueprints := make([]*models.Blueprint, 0)
	for _, dbBlueprint := range dbBlueprints {
		dbBlueprint, err = decryptDbBlueprint(dbBlueprint)
		if err != nil {
			return nil, 0, err
		}
		blueprint := parseBlueprint(dbBlueprint)
		blueprints = append(blueprints, blueprint)
	}
	return blueprints, count, nil
}

// GetBlueprint returns the detail of a given Blueprint ID
func GetBlueprint(blueprintId uint64) (*models.Blueprint, errors.Error) {
	dbBlueprint, err := GetDbBlueprint(blueprintId)
	if err != nil {
		if goerror.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.NotFound.New("blueprint not found")
		}
		return nil, errors.Internal.Wrap(err, "error getting the task from database")
	}
	dbBlueprint, err = decryptDbBlueprint(dbBlueprint)
	if err != nil {
		return nil, err
	}
	blueprint := parseBlueprint(dbBlueprint)
	return blueprint, nil
}

func validateBlueprint(blueprint *models.Blueprint) errors.Error {
	// validation
	err := vld.Struct(blueprint)
	if err != nil {
		return errors.BadInput.WrapRaw(err)
	}
	if strings.ToLower(blueprint.CronConfig) == "manual" {
		blueprint.IsManual = true
	}

	if !blueprint.IsManual {
		_, err = cron.ParseStandard(blueprint.CronConfig)
		if err != nil {
			return errors.Default.Wrap(err, "invalid cronConfig")
		}
	}
	if blueprint.Mode == models.BLUEPRINT_MODE_ADVANCED {
		plan := make(core.PipelinePlan, 0)
		err = errors.Convert(json.Unmarshal(blueprint.Plan, &plan))

		if err != nil {
			return errors.Default.Wrap(err, "invalid plan")
		}
		// tasks should not be empty
		if len(plan) == 0 || len(plan[0]) == 0 {
			return errors.Default.New("empty plan")
		}
	} else if blueprint.Mode == models.BLUEPRINT_MODE_NORMAL {
		blueprint.Plan, err = GeneratePlanJson(blueprint.Settings)
		if err != nil {
			return errors.Default.Wrap(err, "invalid plan")
		}
	}

	return nil
}

// PatchBlueprint FIXME ...
func PatchBlueprint(id uint64, body map[string]interface{}) (*models.Blueprint, errors.Error) {
	// load record from db
	blueprint, err := GetBlueprint(id)
	if err != nil {
		return nil, err
	}

	originMode := blueprint.Mode
	err = helper.DecodeMapStruct(body, blueprint)
	if err != nil {
		return nil, err
	}
	// make sure mode is not being update
	if originMode != blueprint.Mode {
		return nil, errors.Default.New("mode is not updatable")
	}
	// validation
	err = validateBlueprint(blueprint)
	if err != nil {
		return nil, errors.BadInput.WrapRaw(err)
	}

	// save
	err = save(blueprint)
	if err != nil {
		return nil, errors.Internal.Wrap(err, "error saving blueprint")
	}

	// reload schedule
	err = ReloadBlueprints(cronManager)
	if err != nil {
		return nil, errors.Internal.Wrap(err, "error reloading blueprints")
	}
	// done
	return blueprint, nil
}

// DeleteBlueprint FIXME ...
func DeleteBlueprint(id uint64) errors.Error {
	err := DeleteDbBlueprint(id)
	if err != nil {
		return errors.Internal.Wrap(err, fmt.Sprintf("error deleting blueprint %d", id))
	}
	err = ReloadBlueprints(cronManager)
	if err != nil {
		return errors.Internal.Wrap(err, "error reloading blueprints")
	}
	return nil
}

// ReloadBlueprints FIXME ...
func ReloadBlueprints(c *cron.Cron) errors.Error {
	dbBlueprints := make([]*models.DbBlueprint, 0)
	if err := db.Model(&models.DbBlueprint{}).
		Where("enable = ? AND is_manual = ?", true, false).
		Find(&dbBlueprints).Error; err != nil {
		return errors.Internal.Wrap(err, "error finding blueprints while reloading")
	}
	for _, e := range c.Entries() {
		c.Remove(e.ID)
	}
	c.Stop()
	for _, pp := range dbBlueprints {
		pp, err := decryptDbBlueprint(pp)
		if err != nil {
			return err
		}
		blueprint := parseBlueprint(pp)
		plan, err := blueprint.UnmarshalPlan()
		if err != nil {
			blueprintLog.Error(err, failToCreateCronJob)
			return err
		}
		if _, err := c.AddFunc(blueprint.CronConfig, func() {
			pipeline, err := createPipelineByBlueprint(blueprint.ID, blueprint.Name, plan)
			if err != nil {
				blueprintLog.Error(err, "run cron job failed")
			} else {
				blueprintLog.Info("Run new cron job successfully, pipeline id: %d", pipeline.ID)
			}
		}); err != nil {
			blueprintLog.Error(err, failToCreateCronJob)
			return errors.Default.Wrap(err, "created cron job failed")
		}
	}
	if len(dbBlueprints) > 0 {
		c.Start()
	}
	log.Info("total %d blueprints were scheduled", len(dbBlueprints))
	return nil
}

func createPipelineByBlueprint(blueprintId uint64, name string, plan core.PipelinePlan) (*models.Pipeline, errors.Error) {
	newPipeline := models.NewPipeline{}
	newPipeline.Plan = plan
	newPipeline.Name = name
	newPipeline.BlueprintId = blueprintId
	pipeline, err := CreatePipeline(&newPipeline)
	// Return all created tasks to the User
	if err != nil {
		blueprintLog.Error(err, failToCreateCronJob)
		return nil, errors.Convert(err)
	}
	return pipeline, nil
}

// GeneratePlanJson generates pipeline plan by version
func GeneratePlanJson(settings json.RawMessage) (json.RawMessage, errors.Error) {
	bpSettings := new(models.BlueprintSettings)
	err := errors.Convert(json.Unmarshal(settings, bpSettings))

	if err != nil {
		return nil, errors.Default.Wrap(err, fmt.Sprintf("settings:%s", string(settings)))
	}
	var plan interface{}
	switch bpSettings.Version {
	case "1.0.0":
		plan, err = GeneratePlanJsonV100(bpSettings)
	default:
		return nil, errors.Default.New(fmt.Sprintf("unknown version of blueprint settings: %s", bpSettings.Version))
	}
	if err != nil {
		return nil, err
	}
	return errors.Convert01(json.Marshal(plan))
}

// GeneratePlanJsonV100 generates pipeline plan according v1.0.0 definition
func GeneratePlanJsonV100(settings *models.BlueprintSettings) (core.PipelinePlan, errors.Error) {
	connections := make([]*core.BlueprintConnectionV100, 0)
	err := errors.Convert(json.Unmarshal(settings.Connections, &connections))
	if err != nil {
		return nil, err
	}
	hasDoraEnrich := false
	doraRules := make(map[string]interface{})
	plans := make([]core.PipelinePlan, len(connections))
	for i, connection := range connections {
		if len(connection.Scope) == 0 {
			return nil, errors.Default.New(fmt.Sprintf("connections[%d].scope is empty", i))
		}
		plugin, err := core.GetPlugin(connection.Plugin)
		if err != nil {
			return nil, err
		}
		if pluginBp, ok := plugin.(core.PluginBlueprintV100); ok {
			plans[i], err = pluginBp.MakePipelinePlan(connection.ConnectionId, connection.Scope)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, errors.Default.New(fmt.Sprintf("plugin %s does not support blueprint protocol version 1.0.0", connection.Plugin))
		}
		for _, stage := range plans[i] {
			for _, task := range stage {
				if task.Plugin == "dora" {
					hasDoraEnrich = true
					for k, v := range task.Options {
						doraRules[k] = v
					}
				}
			}
		}
	}
	mergedPipelinePlan := MergePipelinePlans(plans...)
	if hasDoraEnrich {
		plan := core.PipelineStage{
			&core.PipelineTask{
				Plugin:   "dora",
				Subtasks: []string{"calculateChangeLeadTime", "ConnectIssueDeploy"},
				Options:  doraRules,
			},
		}
		mergedPipelinePlan = append(mergedPipelinePlan, plan)
	}
	return FormatPipelinePlans(settings.BeforePlan, mergedPipelinePlan, settings.AfterPlan)
}

// FormatPipelinePlans merges multiple pipelines and append before and after pipeline
func FormatPipelinePlans(beforePlanJson json.RawMessage, mainPlan core.PipelinePlan, afterPlanJson json.RawMessage) (core.PipelinePlan, errors.Error) {
	newPipelinePlan := core.PipelinePlan{}
	if beforePlanJson != nil {
		beforePipelinePlan := core.PipelinePlan{}
		err := errors.Convert(json.Unmarshal(beforePlanJson, &beforePipelinePlan))
		if err != nil {
			return nil, err
		}
		newPipelinePlan = append(newPipelinePlan, beforePipelinePlan...)
	}

	newPipelinePlan = append(newPipelinePlan, mainPlan...)

	if afterPlanJson != nil {
		afterPipelinePlan := core.PipelinePlan{}
		err := errors.Convert(json.Unmarshal(afterPlanJson, &afterPipelinePlan))
		if err != nil {
			return nil, err
		}
		newPipelinePlan = append(newPipelinePlan, afterPipelinePlan...)
	}
	return newPipelinePlan, nil
}

// MergePipelinePlans merges multiple pipelines into one unified pipeline
func MergePipelinePlans(plans ...core.PipelinePlan) core.PipelinePlan {
	merged := make(core.PipelinePlan, 0)
	// iterate all pipelineTasks and try to merge them into `merged`
	for _, plan := range plans {
		// add all stages from plan to merged
		for index, stage := range plan {
			if index >= len(merged) {
				merged = append(merged, nil)
			}
			// add all tasks from plan to target respectively
			merged[index] = append(merged[index], stage...)
		}
	}
	return merged
}

// TriggerBlueprint triggers blueprint immediately
func TriggerBlueprint(id uint64) (*models.Pipeline, errors.Error) {
	// load record from db
	blueprint, err := GetBlueprint(id)
	if err != nil {
		return nil, err
	}
	plan, err := blueprint.UnmarshalPlan()
	if err != nil {
		return nil, err
	}
	pipeline, err := createPipelineByBlueprint(blueprint.ID, blueprint.Name, plan)
	// done
	return pipeline, err
}
func save(blueprint *models.Blueprint) errors.Error {
	dbBlueprint := parseDbBlueprint(blueprint)
	dbBlueprint, err := encryptDbBlueprint(dbBlueprint)
	if err != nil {
		return err
	}
	return errors.Convert(db.Save(dbBlueprint).Error)
}
