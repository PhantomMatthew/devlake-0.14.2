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
	"gorm.io/gorm"
)

type modifyLeadTimeMinutes struct{}

type newIssue struct {
	LeadTimeMinutes int64
}

func (newIssue) TableName() string {
	return "issues"
}

func (*modifyLeadTimeMinutes) Up(ctx context.Context, db *gorm.DB) errors.Error {
	err := db.Migrator().RenameColumn(&newIssue{}, "lead_time_minutes", "lead_time_minutes_bak")
	if err != nil {
		return errors.Convert(err)
	}
	err = db.Migrator().AddColumn(newIssue{}, "lead_time_minutes")
	if err != nil {
		return errors.Convert(err)
	}
	err = db.Model(&newIssue{}).Where("lead_time_minutes_bak > 0").UpdateColumn("lead_time_minutes", gorm.Expr("lead_time_minutes_bak")).Error
	if err != nil {
		return errors.Convert(err)
	}
	err = db.Migrator().DropColumn(&newIssue{}, "lead_time_minutes_bak")
	if err != nil {
		return errors.Convert(err)
	}
	db.First(&newIssue{})
	return nil
}

func (*modifyLeadTimeMinutes) Version() uint64 {
	return 20220929145125
}

func (*modifyLeadTimeMinutes) Name() string {
	return "modify lead_time_minutes"
}
