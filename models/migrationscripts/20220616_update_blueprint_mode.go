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

type Blueprint20220616 struct {
	Mode     string `json:"mode" gorm:"varchar(20)" validate:"required,oneof=NORMAL ADVANCED"`
	IsManual bool   `json:"isManual"`
}

func (Blueprint20220616) TableName() string {
	return "_devlake_blueprints"
}

type updateBlueprintMode struct{}

func (*updateBlueprintMode) Up(ctx context.Context, db *gorm.DB) errors.Error {
	err := db.Migrator().AutoMigrate(&Blueprint20220616{})
	if err != nil {
		return errors.Convert(err)
	}
	db.Model(&Blueprint20220616{}).Where("mode is null").Update("mode", "ADVANCED")
	db.Model(&Blueprint20220616{}).Where("is_manual is null").Update("is_manual", false)
	return nil
}

func (*updateBlueprintMode) Version() uint64 {
	return 20220616110537
}

func (*updateBlueprintMode) Name() string {
	return "add mode field to blueprint"
}
