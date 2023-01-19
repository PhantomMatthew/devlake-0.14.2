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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"reflect"
	"strings"

	"github.com/apache/incubator-devlake/errors"
	"github.com/apache/incubator-devlake/models/migrationscripts/archived"
	"github.com/apache/incubator-devlake/plugins/gitlab/api"
	"github.com/apache/incubator-devlake/plugins/helper"
	"gorm.io/gorm"
)

type CommitFileAddLength struct {
	archived.DomainEntity
	CommitSha string `gorm:"type:varchar(40)"`
	FilePath  string `gorm:"type:text"`
	Additions int
	Deletions int
}

func (CommitFileAddLength) TableName() string {
	return "commit_files"
}

type CommitFileAddLengthBak struct {
	archived.DomainEntity
	CommitSha string `gorm:"type:varchar(40)"`
	FilePath  string `gorm:"type:varchar(255)"`
	Additions int
	Deletions int
}

func (CommitFileAddLengthBak) TableName() string {
	return "commit_files_bak"
}

type CommitFileComponentBak struct {
	archived.NoPKModel
	CommitFileId  string `gorm:"primaryKey;type:varchar(255)"`
	ComponentName string `gorm:"type:varchar(255)"`
}

func (CommitFileComponentBak) TableName() string {
	return "commit_file_components_bak"
}

type addCommitFilePathLength struct{}

func (*addCommitFilePathLength) Up(ctx context.Context, db *gorm.DB) (errs errors.Error) {
	var err error

	// rename the commit_file_bak to cache old table
	err = db.Migrator().RenameTable(&CommitFile{}, &CommitFileAddLengthBak{})
	if err != nil {
		return errors.Default.Wrap(err, "error no rename commit_file to commit_files_bak")
	}

	// rollback for rename back
	defer func() {
		if errs != nil {
			err = db.Migrator().RenameTable(&CommitFileAddLengthBak{}, &CommitFile{})
			if err != nil {
				errs = errors.Default.Wrap(err, fmt.Sprintf("fail to rollback table commit_file_bak , you must to rollback by yourself. %s", err.Error()))
			}
		}
	}()

	// create new commit_files table
	err = db.Migrator().AutoMigrate(&CommitFileAddLength{})
	if err != nil {
		return errors.Default.Wrap(err, "error on auto migrate commit_file")
	}

	// rollback for create new table
	defer func() {
		if errs != nil {
			err = db.Migrator().DropTable(&CommitFile{})
			if err != nil {
				errs = errors.Default.Wrap(err, fmt.Sprintf("fail to rollback table CommitFile , you must to rollback by yourself. %s", err.Error()))
			}
		}
	}()

	// update old id to new id and write to the new table
	cursor, err := db.Model(&CommitFileAddLengthBak{}).Rows()
	if err != nil {
		return errors.Default.Wrap(err, "error on select CommitFileAddLength")
	}
	defer cursor.Close()

	// caculate and save the data to new table
	batch, err := helper.NewBatchSave(api.BasicRes, reflect.TypeOf(&CommitFileAddLength{}), 200)
	if err != nil {
		return errors.Default.Wrap(err, "error getting batch from table commit_file")
	}

	defer batch.Close()
	for cursor.Next() {
		cfb := CommitFileAddLengthBak{}
		err = db.ScanRows(cursor, &cfb)
		if err != nil {
			return errors.Default.Wrap(err, "error scan rows from table commit_files_bak")
		}

		cf := CommitFileAddLength(cfb)

		// With some long path,the varchar(255) was not enough both ID and file_path
		// So we use the hash to compress the path in ID and add length of file_path.
		shaFilePath := sha256.New()
		shaFilePath.Write([]byte(cf.FilePath))
		cf.Id = cf.CommitSha + ":" + hex.EncodeToString(shaFilePath.Sum(nil))

		err = batch.Add(&cf)
		if err != nil {
			return errors.Default.Wrap(err, "error on commit_files batch add")
		}
	}

	// rename the commit_file_components_bak
	err = db.Migrator().RenameTable(&CommitFileComponent{}, &CommitFileComponentBak{})
	if err != nil {
		return errors.Default.Wrap(err, "error no rename commit_file_components to commit_file_components_bak")
	}

	// rollback for rename back
	defer func() {
		if errs != nil {
			err = db.Migrator().RenameTable(&CommitFileComponentBak{}, &CommitFileComponent{})
			if err != nil {
				errs = errors.Default.Wrap(err, fmt.Sprintf("fail to rollback table commit_file_components_bak , you must to rollback by yourself. %s", err.Error()))
			}
		}
	}()

	// create new commit_file_components table
	err = db.Migrator().AutoMigrate(&CommitFileComponent{})
	if err != nil {
		return errors.Default.Wrap(err, "error on auto migrate commit_file")
	}

	// rollback for create new table
	defer func() {
		if errs != nil {
			err = db.Migrator().DropTable(&CommitFileComponent{})
			if err != nil {
				errs = errors.Default.Wrap(err, fmt.Sprintf("fail to rollback table commit_file_components , you must to rollback by yourself. %s", err.Error()))
			}
		}
	}()

	// update old id to new id and write to the new table
	cursor2, err := db.Model(&CommitFileComponentBak{}).Rows()
	if err != nil {
		return errors.Default.Wrap(err, "error on select commit_file_components_bak")
	}
	defer cursor2.Close()

	// caculate and save the data to new table
	batch2, err := helper.NewBatchSave(api.BasicRes, reflect.TypeOf(&CommitFileComponent{}), 500)
	if err != nil {
		return errors.Default.Wrap(err, "error getting batch from table commit_file_components")
	}
	defer batch2.Close()

	for cursor2.Next() {
		cfcb := CommitFileComponentBak{}
		err = db.ScanRows(cursor2, &cfcb)
		if err != nil {
			return errors.Default.Wrap(err, "error scan rows from table commit_file_components_bak")
		}

		cfc := CommitFileComponent(cfcb)

		ids := strings.Split(cfc.CommitFileId, ":")

		commitSha := ""
		filePath := ""

		if len(ids) > 0 {
			commitSha = ids[0]
			if len(ids) > 1 {
				for i := 1; i < len(ids); i++ {
					if i > 1 {
						filePath += ":"
					}
					filePath += ids[i]
				}
			}
		}

		// With some long path,the varchar(255) was not enough both ID and file_path
		// So we use the hash to compress the path in ID and add length of file_path.
		shaFilePath := sha256.New()
		shaFilePath.Write([]byte(filePath))
		cfc.CommitFileId = commitSha + ":" + hex.EncodeToString(shaFilePath.Sum(nil))

		err = batch2.Add(&cfc)
		if err != nil {
			return errors.Default.Wrap(err, "error on commit_file_components batch add")
		}
	}

	// drop the old table
	err = db.Migrator().DropTable(&CommitFileAddLengthBak{})
	if err != nil {
		return errors.Default.Wrap(err, "error no drop commit_files_bak")
	}
	err = db.Migrator().DropTable(&CommitFileComponentBak{})
	if err != nil {
		return errors.Default.Wrap(err, "error no drop commit_file_components_bak")
	}

	return nil
}

func (*addCommitFilePathLength) Version() uint64 {
	return 20220913165805
}

func (*addCommitFilePathLength) Name() string {
	return "add length of commit_file file_path"
}
