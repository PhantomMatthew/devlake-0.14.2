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

package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/apache/incubator-devlake/errors"
	"github.com/apache/incubator-devlake/impl/dalgorm"
	"github.com/apache/incubator-devlake/plugins/core"
	"github.com/apache/incubator-devlake/plugins/core/dal"
	"github.com/lib/pq"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

type Table struct {
	name string
}

func (t *Table) TableName() string {
	return t.name
}

func LoadData(c core.SubTaskContext) errors.Error {
	var db dal.Dal
	config := c.GetData().(*StarRocksConfig)
	if config.SourceDsn != "" && config.SourceType != "" {
		var o *gorm.DB
		var err error
		if config.SourceType == "mysql" {
			o, err = gorm.Open(mysql.Open(config.SourceDsn))
			if err != nil {
				return errors.Convert(err)
			}
		} else if config.SourceType == "postgres" {
			o, err = gorm.Open(postgres.Open(config.SourceDsn))
			if err != nil {
				return errors.Convert(err)
			}
		} else {
			return errors.NotFound.New(fmt.Sprintf("unsupported source type %s", config.SourceType))
		}
		db = dalgorm.NewDalgorm(o)
	} else {
		db = c.GetDal()
	}
	var starrocksTables []string
	if config.DomainLayer != "" {
		starrocksTables = getTablesByDomainLayer(config.DomainLayer)
		if starrocksTables == nil {
			return errors.NotFound.New(fmt.Sprintf("no table found by domain layer: %s", config.DomainLayer))
		}
	} else {
		tables := config.Tables
		allTables, err := db.AllTables()
		if err != nil {
			return err
		}
		if len(tables) == 0 {
			starrocksTables = allTables
		} else {
			for _, table := range allTables {
				for _, r := range tables {
					var ok bool
					ok, err = errors.Convert01(regexp.Match(r, []byte(table)))
					if err != nil {
						return err
					}
					if ok {
						starrocksTables = append(starrocksTables, table)
					}
				}
			}
		}
	}

	starrocks, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local", config.User, config.Password, config.Host, config.Port, config.Database))
	if err != nil {
		return errors.Convert(err)
	}

	for _, table := range starrocksTables {
		starrocksTable := strings.TrimLeft(table, "_")
		var columnMap map[string]string
		columnMap, err = createTable(starrocks, db, starrocksTable, table, c, config.Extra)
		if err != nil {
			c.GetLogger().Error(err, "create table %s in starrocks error", table)
			return errors.Convert(err)
		}
		err = loadData(starrocks, c, starrocksTable, table, columnMap, db, config)
		if err != nil {
			c.GetLogger().Error(err, "load data %s error", table)
			return errors.Convert(err)
		}
	}
	return nil
}
func createTable(starrocks *sql.DB, db dal.Dal, starrocksTable string, table string, c core.SubTaskContext, extra string) (map[string]string, errors.Error) {
	columeMetas, err := db.GetColumns(&Table{name: table}, nil)
	columnMap := make(map[string]string)
	if err != nil {
		return columnMap, err
	}
	var pks []string
	var columns []string
	firstcm := ""
	for _, cm := range columeMetas {
		name := cm.Name()
		starrocksDatatype, ok := cm.ColumnType()
		if !ok {
			return columnMap, errors.Default.New(fmt.Sprintf("Get [%s] ColumeType Failed", name))
		}
		dataType := getDataType(starrocksDatatype)
		columnMap[name] = dataType
		column := fmt.Sprintf("`%s` %s", name, dataType)
		columns = append(columns, column)
		isPrimaryKey, ok := cm.PrimaryKey()
		if isPrimaryKey && ok {
			pks = append(pks, fmt.Sprintf("`%s`", name))
		}
		if firstcm == "" {
			firstcm = fmt.Sprintf("`%s`", name)
		}
	}

	if len(pks) == 0 {
		pks = append(pks, firstcm)
	}

	if extra == "" {
		extra = fmt.Sprintf(`engine=olap distributed by hash(%s) properties("replication_num" = "1")`, strings.Join(pks, ", "))
	}
	tableSql := fmt.Sprintf("create table if not exists `%s` ( %s ) %s", starrocksTable, strings.Join(columns, ","), extra)
	c.GetLogger().Info(tableSql)
	_, err = errors.Convert01(starrocks.Exec(tableSql))
	return columnMap, err
}

func loadData(starrocks *sql.DB, c core.SubTaskContext, starrocksTable string, table string, columnMap map[string]string, db dal.Dal, config *StarRocksConfig) error {
	offset := 0
	starrocksTmpTable := starrocksTable + "_tmp"
	// create tmp table in starrocks
	_, execErr := starrocks.Exec(fmt.Sprintf("drop table if exists %s; create table %s like %s", starrocksTmpTable, starrocksTmpTable, starrocksTable))
	if execErr != nil {
		return execErr
	}
	var err error
	for {
		var rows *sql.Rows
		var data []map[string]interface{}
		// select data from db
		rows, err = db.RawCursor(fmt.Sprintf("select * from %s limit %d offset %d", table, config.BatchSize, offset))
		if err != nil {
			return err
		}
		cols, err := rows.Columns()
		if err != nil {
			return err
		}
		for rows.Next() {
			row := make(map[string]interface{})
			columns := make([]interface{}, len(cols))
			columnPointers := make([]interface{}, len(cols))
			for i := range columns {
				dataType := columnMap[cols[i]]
				if strings.HasPrefix(dataType, "array") {
					var arr []string
					columns[i] = &arr
					columnPointers[i] = pq.Array(&arr)
				} else {
					columnPointers[i] = &columns[i]
				}
			}
			err = rows.Scan(columnPointers...)
			if err != nil {
				return err
			}
			for i, colName := range cols {
				row[colName] = columns[i]
			}
			data = append(data, row)
		}
		if len(data) == 0 {
			c.GetLogger().Warn(nil, "no data found in table %s already, limit: %d, offset: %d, so break", table, config.BatchSize, offset)
			break
		}
		// insert data to tmp table
		loadURL := fmt.Sprintf("http://%s:%d/api/%s/%s/_stream_load", config.BeHost, config.BePort, config.Database, starrocksTmpTable)
		headers := map[string]string{
			"format":            "json",
			"strip_outer_array": "true",
			"Expect":            "100-continue",
			"ignore_json_size":  "true",
			"Connection":        "close",
		}
		jsonData, err := json.Marshal(data)
		if err != nil {
			return err
		}
		client := http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
		req, err := http.NewRequest(http.MethodPut, loadURL, bytes.NewBuffer(jsonData))
		if err != nil {
			return err
		}
		req.SetBasicAuth(config.User, config.Password)
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		if resp.StatusCode == 307 {
			var location *url.URL
			location, err = resp.Location()
			if err != nil {
				return err
			}
			req, err = http.NewRequest(http.MethodPut, location.String(), bytes.NewBuffer(jsonData))
			if err != nil {
				return err
			}
			req.SetBasicAuth(config.User, config.Password)
			for k, v := range headers {
				req.Header.Set(k, v)
			}
			resp, err = client.Do(req)
		}
		if err != nil {
			return err
		}
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		var result map[string]interface{}
		err = json.Unmarshal(b, &result)
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusOK {
			c.GetLogger().Error(nil, "[%s]: %s", resp.StatusCode, string(b))
		}
		if result["Status"] != "Success" {
			c.GetLogger().Error(nil, "load %s failed: %s", table, string(b))
		} else {
			c.GetLogger().Info("load %s success: %s, limit: %d, offset: %d", table, b, config.BatchSize, offset)
		}
		offset += len(data)
	}
	// drop old table
	_, err = starrocks.Exec(fmt.Sprintf("drop table if exists %s", starrocksTable))
	if err != nil {
		return err
	}
	// rename tmp table to old table
	_, err = starrocks.Exec(fmt.Sprintf("alter table %s rename %s", starrocksTmpTable, starrocksTable))
	if err != nil {
		return err
	}
	c.GetLogger().Info("load %s to starrocks success", table)
	return nil
}

var (
	LoadDataTaskMeta = core.SubTaskMeta{
		Name:             "LoadData",
		EntryPoint:       LoadData,
		EnabledByDefault: true,
		Description:      "Load data to StarRocks",
	}
)
