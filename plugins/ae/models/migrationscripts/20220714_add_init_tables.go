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

	"github.com/apache/incubator-devlake/plugins/ae/models/migrationscripts/archived"
	"github.com/apache/incubator-devlake/plugins/core"
	"github.com/apache/incubator-devlake/plugins/helper"
	"gorm.io/gorm"
)

type addInitTables struct {
	config core.ConfigGetter
}

func (u *addInitTables) SetConfigGetter(config core.ConfigGetter) {
	u.config = config
}

func (u *addInitTables) Up(ctx context.Context, db *gorm.DB) errors.Error {
	err := db.Migrator().DropTable(
		&archived.AECommit{},
		&archived.AEProject{},
		"_raw_ae_project",
		"_raw_ae_commits",
	)
	if err != nil {
		return errors.Convert(err)
	}

	err = db.Migrator().AutoMigrate(
		&archived.AECommit{},
		&archived.AEProject{},
		&archived.AeConnection{},
	)
	if err != nil {
		return errors.Convert(err)
	}

	encodeKey := u.config.GetString(core.EncodeKeyEnvStr)
	connection := &archived.AeConnection{}
	connection.Endpoint = u.config.GetString("AE_ENDPOINT")
	connection.Proxy = u.config.GetString("AE_PROXY")
	connection.SecretKey = u.config.GetString("AE_SECRET_KEY")
	connection.AppId = u.config.GetString("AE_APP_ID")
	connection.Name = "AE"
	if connection.Endpoint != "" && connection.AppId != "" && connection.SecretKey != "" && encodeKey != "" {
		err = helper.UpdateEncryptFields(connection, func(plaintext string) (string, errors.Error) {
			return core.Encrypt(encodeKey, plaintext)
		})
		if err != nil {
			return errors.Default.Wrap(err, "error encrypting fields")
		}
		// update from .env and save to db
		db.Create(connection)
	}

	return nil
}

func (*addInitTables) Version() uint64 {
	return 20220714201133
}

func (*addInitTables) Name() string {
	return "AE init schemas"
}
