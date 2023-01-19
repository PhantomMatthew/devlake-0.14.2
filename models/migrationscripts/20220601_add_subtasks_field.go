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

package migrationscripts

import (
	"context"
	"github.com/apache/incubator-devlake/errors"
	"time"

	"github.com/apache/incubator-devlake/models/migrationscripts/archived"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type Task20220601 struct {
	archived.Model
	Plugin        string         `json:"plugin" gorm:"index"`
	Subtasks      datatypes.JSON `json:"subtasks"`
	Options       datatypes.JSON `json:"options"`
	Status        string         `json:"status"`
	Message       string         `json:"message"`
	Progress      float32        `json:"progress"`
	FailedSubTask string         `json:"failedSubTask"`
	PipelineId    uint64         `json:"pipelineId" gorm:"index"`
	PipelineRow   int            `json:"pipelineRow"`
	PipelineCol   int            `json:"pipelineCol"`
	BeganAt       *time.Time     `json:"beganAt"`
	FinishedAt    *time.Time     `json:"finishedAt" gorm:"index"`
	SpentSeconds  int            `json:"spentSeconds"`
}

func (Task20220601) TableName() string {
	return "_devlake_tasks"
}

type addSubtasksField struct{}

func (*addSubtasksField) Up(ctx context.Context, db *gorm.DB) errors.Error {
	return errors.Convert(db.Migrator().AddColumn(Task20220601{}, "subtasks"))
}

func (*addSubtasksField) Version() uint64 {
	return 20220601000005
}

func (*addSubtasksField) Name() string {
	return "add column `subtasks` at _devlake_tasks"
}
