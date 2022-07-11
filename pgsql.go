package main

import (
	"fmt"
	"log"
	"reflect"
	"strconv"
	"strings"
	"time"

	"backpocket/utils"
)

func sqlTableID() uint64 {
	sqlID, _ := strconv.Atoi(fmt.Sprintf("%v", time.Now().UnixNano())[:15])
	return uint64(sqlID)
}

func sqlDBInit() {
	utils.Init(nil)

	var allTables = []string{"assets", "orders", "markets"}

	lExists := false
	sqlTable := "SELECT EXISTS (SELECT FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace WHERE n.nspname = 'public' AND c.relname = $1 AND c.relkind = 'r')"
	for _, tableName := range allTables {

		err := utils.SqlDB.Get(&lExists, sqlTable, tableName)
		if err != nil {
			log.Println(err.Error())
		}

		if !lExists {
			switch tableName {
			case "assets":
				if reflectType := reflect.TypeOf(assets{}); !sqlTableCreate(reflectType) {
					log.Panicf("Table creation failed for table [%s] \n", reflectType.Name())
				}

			case "orders":
				if reflectType := reflect.TypeOf(orders{}); !sqlTableCreate(reflectType) {
					log.Panicf("Table creation failed for table [%s] \n", reflectType.Name())
				}

			case "markets":
				if reflectType := reflect.TypeOf(markets{}); !sqlTableCreate(reflectType) {
					log.Panicf("Table creation failed for table [%s] \n", reflectType.Name())
				}

				// case "Users":
				// 	if reflectType := reflect.TypeOf(tables.Users{}); !sqlTableCreate(reflectType) {
				// 		log.Panicf("Table creation failed for table [%s] \n", reflectType.Name())
				// 	}
			}
		}
	}
	time.Sleep(time.Second)
}

func sqlTableCreate(reflectType reflect.Type) (success bool) {
	tablename := strings.ToLower(reflectType.Name())

	if tablename == "" {
		return
	}

	var sqlTypes = map[string]string{
		"bool":     "bool",
		"time":     "timestamp",
		"jsondate": "date",
		"jsontime": "time",
		"string":   "text",
		"int":      "int",
		"uint":     "int",
		"int64":    "int8",
		"uint32":   "int8",
		"uint64":   "int8",
		"float32":  "float8",
		"float64":  "float8",
	}

	var sqlCreate, sqlIndex string
	for i := 0; i < reflectType.NumField(); i++ {
		field := reflectType.Field(i)
		tag := field.Tag.Get("sql")
		fieldName := strings.ToLower(field.Name)
		fieldType := sqlTypes[strings.ToLower(field.Type.Name())]

		defaultValue := ""
		switch fieldType {
		case "bool":
			defaultValue = "DEFAULT false"
		case "date":
			defaultValue = "DEFAULT current_date"
		case "time":
			defaultValue = "DEFAULT current_time"
		case "timestamp":
			defaultValue = "DEFAULT current_timestamp"
		case "text":
			defaultValue = "DEFAULT ''"
		case "float", "float8":
			defaultValue = "DEFAULT 0.0"
		case "int", "int8":
			defaultValue = "DEFAULT 0"
		default:
			log.Printf(fieldType)
		}

		if defaultValue != "" {
			sqlCreate += fmt.Sprintf("%s %s %s", fieldName, fieldType, defaultValue)
		}

		switch tag {
		case "pk":
			if fieldName == "id" {
				sqlCreate += "id SERIAL PRIMARY KEY"
			}
		case "index", "unique index":
			sqlIndex += fmt.Sprintf("\ncreate %s idx_"+tablename+"_%s on "+tablename+" (%s);", tag, fieldName, fieldName)
		}
		if sqlCreate != "" {
			sqlCreate += ", "
		}
	}

	if sqlCreate == "" {
		return
	}
	utils.SqlDB.Exec("drop table " + tablename)
	sqlCreate = "create table " + tablename + " (" + strings.TrimSuffix(sqlCreate, ", ") + "); "

	if _, err := utils.SqlDB.Exec(sqlCreate); err != nil {
		log.Panicf("\n error creating database table %v \n %v", err, sqlCreate)
	}

	if success = true; sqlIndex == "" {
		return
	}

	if _, err := utils.SqlDB.Exec(sqlIndex); err != nil {
		log.Panicf("error creating table indices %v \n", err)
	}

	return
}

func sqlTableInsert(reflectType reflect.Type, reflectValue reflect.Value) (sqlQuery string, sqlParams []interface{}) {
	tablename := strings.ToLower(reflectType.Name())

	if tablename == "" {
		return
	}

	var sqlColumns, sqlPlacers []string
	for i := 0; i < reflectType.NumField(); i++ {

		field := reflectType.Field(i)
		fieldName := strings.ToLower(field.Name)

		fieldValue := reflectValue.FieldByName(field.Name)
		switch fieldValue.Kind() {
		case reflect.Int, reflect.Int32, reflect.Int64:
			sqlParams = append(sqlParams, fieldValue.Int())

		case reflect.Uint, reflect.Uint32, reflect.Uint64:
			if fieldName != "id" {
				sqlParams = append(sqlParams, fieldValue.Uint())
			} else {
				if fieldValue.Uint() > 0 {
					sqlParams = append(sqlParams, fieldValue.Uint())
				} else {
					id := sqlTableID()
					sqlParams = append(sqlParams, id)
				}
			}

		case reflect.Float32, reflect.Float64:
			sqlParams = append(sqlParams, fieldValue.Float())

		default:
			if fieldName == "created" || fieldName == "updated" {
				sqlParams = append(sqlParams, utils.GetSystemTime())
			} else {
				sqlParams = append(sqlParams, fieldValue.String())
			}
		}
		sqlColumns = append(sqlColumns, fieldName)
		sqlPlacers = append(sqlPlacers, fmt.Sprintf("$%v ", len(sqlParams)))
	}

	sqlQuery = "insert into " + tablename + " (" + strings.Join(sqlColumns, ",") + ") values (" + strings.Join(sqlPlacers, " ,") + ")"
	return
}

func sqlTableUpdate(reflectType reflect.Type, reflectValue reflect.Value, mapUpdatefields map[string]bool) (sqlQuery string, sqlParams []interface{}) {
	tablename := strings.ToLower(reflectType.Name())

	if tablename == "" {
		return
	}

	var sqlColumns []string
	for i := 0; i < reflectType.NumField(); i++ {

		field := reflectType.Field(i)
		fieldName := strings.ToLower(field.Name)
		if fieldName == "id" || fieldName == "created" || fieldName == "createdby" || fieldName == "ownerid" {
			continue
		}

		if len(mapUpdatefields) > 0 && !mapUpdatefields[fieldName] {
			continue
		}

		fieldValue := reflectValue.FieldByName(field.Name)
		switch fieldValue.Kind() {
		case reflect.Int, reflect.Int32, reflect.Int64:
			// if fieldValue.Int() == 0 {
			// 	continue
			// }
			sqlParams = append(sqlParams, fieldValue.Int())

		case reflect.Uint, reflect.Uint32, reflect.Uint64:
			// if fieldValue.Uint() == 0 {
			// 	continue
			// }
			sqlParams = append(sqlParams, fieldValue.Uint())

		case reflect.Float32, reflect.Float64:
			// if fieldValue.Float() == 0.0 {
			// 	continue
			// }
			sqlParams = append(sqlParams, fieldValue.Float())

		default:
			// if fieldValue.String() == "" && fieldName != "updated" {
			// 	continue
			// }

			if fieldName == "updated" {
				sqlParams = append(sqlParams, utils.GetSystemTime())
			} else {
				sqlParams = append(sqlParams, fieldValue.String())
			}
		}
		sqlColumns = append(sqlColumns, fieldName+" = "+fmt.Sprintf(" $%v ", len(sqlParams)))
	}

	fieldValue := reflectValue.FieldByName("ID")
	sqlParams = append(sqlParams, fieldValue.Uint())

	sqlQuery = "update " + tablename + " set " + strings.Join(sqlColumns, ",") + " where id = " + fmt.Sprintf(" $%v ", len(sqlParams))
	return
}
