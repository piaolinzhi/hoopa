package main

import (
	"database/sql"
	"fmt"
	"os"
	"path"
	"strings"

	_ "github.com/go-sql-driver/mysql"
)

type DbTransformer interface {
	GetTableNames(conn *sql.DB) []string
	GetConstraints(conn *sql.DB, table *Table, blackList map[string]bool)
	GetColumns(conn *sql.DB, table *Table, blackList map[string]bool)
	GetJavaDataType(sqlType string) string
}

type MysqlDB struct {
}

var dbDriver = map[string]DbTransformer{
	"mysql": &MysqlDB{},
}

type MvcPath struct {
	ControllerPath string
	ServicePath    string
	DaoPath        string
	ModelPath      string
	ExceptionPath  string
}

// sql数据类型与java数据类型映射表
var typeMappingMysql = map[string]string{
	"int":                "Integer", //**
	"integer":            "Integer", //
	"tinyint":            "Short",
	"smallint":           "Short",
	"mediumint":          "Integer",
	"bigint":             "Long",
	"int unsigned":       "Integer",
	"integer unsigned":   "Long",       //java.lang.Long
	"tinyint unsigned":   "Integer",    //java.lang.Integer
	"smallint unsigned":  "Integer",    //java.lang.Integer
	"mediumint unsigned": "Integer",    //java.lang.Integer
	"bigint unsigned":    "BigInteger", //java.math.BigInteger
	"bit":                "Long",
	"bool":               "Boolean",
	"enum":               "String",
	"set":                "String",
	"varchar":            "String", //java.lang.String
	"char":               "String", //java.lang.String
	"tinytext":           "String",
	"mediumtext":         "String",
	"text":               "String", //java.lang.String
	"longtext":           "String",
	"blob":               "String",
	"tinyblob":           "String",
	"mediumblob":         "String",
	"longblob":           "byte[]",

	"date":      "Date", //java.sql.Date
	"datetime":  "Date", //java.sql.Timestamp
	"timestamp": "Date", //java.sql.Timestamp
	"time":      "Date", //java.sql.Time

	"float":     "Float",      //java.lang.Float
	"double":    "Double",     //java.lang.Double
	"decimal":   "BigDecimal", //java.math.BigDecimal
	"binary":    "String",
	"varbinary": "String",
}

// 表结构
type Table struct {
	Name          string                 //表名
	Pk            string                 //主键column字段,如果为空,则没有主键
	PkType        string                 //主键字段的数据类型
	Uk            []string               //唯一约束条件
	Fk            map[string]*ForeignKey //外键
	Columns       []*Column              //包含的字段
	ImportTimePkg bool                   //是否包含时间数据类型的字段
}

// 表字段结构
type Column struct {
	Name string  //字段名
	Type string  //字段数据类型
	Tag  *OrmTag //orm标签
}

// 表外键结构
type ForeignKey struct {
	Name      string
	RefSchema string
	RefTable  string
	RefColumn string
}

// orm标签结构
type OrmTag struct {
	Auto        bool   //是否自增长
	Pk          bool   //是否是主键
	Null        bool   //是否允许为空
	Index       bool   //是否是索引
	Unique      bool   //是否是唯一约束
	Column      string //字段名
	Size        string //数据类型长度
	Decimals    string
	Digits      string
	AutoNow     bool
	AutoNowAdd  bool
	Type        string
	Default     string
	RelOne      bool
	ReverseOne  bool
	RelFk       bool
	ReverseMany bool
	RelM2M      bool
	Comment     string //字段注解
}

//组装model
func (tb *Table) GetModelStruct() string {
	rv := fmt.Sprintf("public class %s {\n", BigCamelCase(tb.Name))
	for _, v := range tb.Columns {
		rv += "    " + v.GetModelField() + "\n"
	}
	rv += "}\n"
	return rv
}

//组装dao
func (tb *Table) GetDaoStruct() string {
	rv := fmt.Sprintf("public interface %sDao extends JpaRepository<%s, %s>,JpaSpecificationExecutor<%s> {\n", BigCamelCase(tb.Name), BigCamelCase(tb.Name), tb.PkType, BigCamelCase(tb.Name))
	rv += "}\n"
	return rv
}

//model属性
func (col *Column) GetModelField() string {
	return fmt.Sprintf("%s\n    private %s %s;", col.Tag.String(), col.Type, col.Name)
}

//dao接口
func (col *Column) GetDaoInterface() string {
	return fmt.Sprintf("%s\n    private %s %s;", col.Tag.String(), col.Type, col.Name)
}

// String returns the ORM tag string for a column
func (tag *OrmTag) String() string {
	var ormOptions []string
	var columnOptions []string
	if tag.Pk {
		ormOptions = append(ormOptions, "@Id")
	}
	if tag.Auto {
		ormOptions = append(ormOptions, "@GeneratedValue(strategy=GenerationType.AUTO)")
	}
	if tag.Comment != "" {
		ormOptions = append(ormOptions, fmt.Sprintf("@ApiModelProperty(value=\"%s\")", tag.Comment))
	}

	if tag.Column != "" {
		columnOptions = append(columnOptions, fmt.Sprintf("name=\"%s\"", tag.Column))
	}
	if tag.Size != "" {
		columnOptions = append(columnOptions, fmt.Sprintf("length=%s", tag.Size))
	}
	if !tag.Null {
		columnOptions = append(columnOptions, "nullable = false")
	}
	if tag.Unique {
		columnOptions = append(columnOptions, "unique = true")
	}
	return fmt.Sprintf("%s\n    @Column(%s)", strings.Join(ormOptions, "\n    "), strings.Join(columnOptions, ","))
}

/**
生成代码
driver: "mysql"数据库驱动
connStr: 连接数据库参数
tables: 表名
currpath: 当前路径
group: groupid
*/
func generateAppcode(driver, connStr, tables, currpath, group string) {
	var selectedTables map[string]bool
	if tables != "" {
		selectedTables = make(map[string]bool)
		for _, v := range strings.Split(tables, ",") {
			selectedTables[v] = true
		}
	}
	if driver == "" {
		driver = "mysql"
	}
	switch driver {
	case "mysql":
	default:
		fmt.Printf("[ERRO] Unknown database driver: %s\n", driver)
		os.Exit(2)
	}
	gen(driver, connStr, selectedTables, currpath, group, tables)
}

func gen(dbms, connStr string, selectedTableNames map[string]bool, apppath, group, tablesStr string) {
	db, err := sql.Open(dbms, connStr)
	if err != nil {
		fmt.Printf("[ERRO] Could not connect to %s database: %s, %s\n", dbms, connStr, err)
		os.Exit(2)
	}
	defer db.Close()
	if trans, ok := dbDriver[dbms]; ok {
		fmt.Printf("[INFO] Analyzing database tables...\n")
		var tableNames []string
		if tablesStr != "" {
			tableNames = strings.Split(tablesStr, ",")
		} else {
			tableNames = trans.GetTableNames(db)
		}

		tables := getTableObjects(tableNames, db, trans)
		mvcPath := new(MvcPath)
		mvcPath.ControllerPath = path.Join(apppath, "controller")
		mvcPath.ServicePath = path.Join(apppath, "service")
		mvcPath.DaoPath = path.Join(apppath, "dao")
		mvcPath.ModelPath = path.Join(apppath, "model")
		mvcPath.ExceptionPath = path.Join(apppath, "exception")
		createPaths(mvcPath)
		writeSourceFiles(group, tables, mvcPath, selectedTableNames)
	} else {
		fmt.Printf("[ERRO] %s database is not supported yet.\n", dbms)
		os.Exit(2)
	}
}

func (*MysqlDB) GetTableNames(db *sql.DB) (tables []string) {
	rows, err := db.Query("SHOW TABLES")
	if err != nil {
		fmt.Printf("[ERRO] Could not show tables\n")
		fmt.Printf("[HINT] Check your connection string\n")
		os.Exit(2)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			fmt.Printf("[ERRO] Could not show tables\n")
			os.Exit(2)
		}
		tables = append(tables, name)
	}
	return
}

func getTableObjects(tableNames []string, db *sql.DB, dbTransformer DbTransformer) (tables []*Table) {
	//保存不支持的表(如果是联合主键或者没有主键,就是不支持的表)
	blackList := make(map[string]bool)
	// process constraints information for each table, also gather blacklisted table names
	for _, tableName := range tableNames {
		// create a table struct
		tb := new(Table)
		tb.Name = tableName
		tb.Fk = make(map[string]*ForeignKey)
		dbTransformer.GetConstraints(db, tb, blackList)
		tables = append(tables, tb)
	}
	// process columns, ignoring blacklisted tables
	for _, tb := range tables {
		dbTransformer.GetColumns(db, tb, blackList)
	}
	return
}

//获取表的约束条件
func (*MysqlDB) GetConstraints(db *sql.DB, table *Table, blackList map[string]bool) {
	rows, err := db.Query(
		`SELECT
			c.constraint_type, u.column_name, u.referenced_table_schema, u.referenced_table_name, referenced_column_name, u.ordinal_position
		FROM
			information_schema.table_constraints c
		INNER JOIN
			information_schema.key_column_usage u ON c.constraint_name = u.constraint_name
		WHERE
			c.table_schema = database() AND c.table_name = ? AND u.table_schema = database() AND u.table_name = ?`,
		table.Name, table.Name) //  u.position_in_unique_constraint,
	if err != nil {
		fmt.Printf("[ERRO] Could not query INFORMATION_SCHEMA for PK/UK/FK information\n")
		os.Exit(2)
	}
	for rows.Next() {
		var constraintTypeBytes, columnNameBytes, refTableSchemaBytes, refTableNameBytes, refColumnNameBytes, refOrdinalPosBytes []byte
		if err := rows.Scan(&constraintTypeBytes, &columnNameBytes, &refTableSchemaBytes, &refTableNameBytes, &refColumnNameBytes, &refOrdinalPosBytes); err != nil {
			fmt.Printf("[ERRO] Could not read INFORMATION_SCHEMA for PK/UK/FK information\n")
			os.Exit(2)
		}
		constraintType, columnName, refTableSchema, refTableName, refColumnName, refOrdinalPos :=
			string(constraintTypeBytes), string(columnNameBytes), string(refTableSchemaBytes),
			string(refTableNameBytes), string(refColumnNameBytes), string(refOrdinalPosBytes)
		if constraintType == "PRIMARY KEY" {
			if refOrdinalPos == "1" {
				table.Pk = columnName
			} else {
				table.Pk = ""
				// add table to blacklist so that other struct will not reference it, because we are not
				// registering blacklisted tables
				blackList[table.Name] = true
			}
		} else if constraintType == "UNIQUE" {
			table.Uk = append(table.Uk, columnName)
		} else if constraintType == "FOREIGN KEY" {
			fk := new(ForeignKey)
			fk.Name = columnName
			fk.RefSchema = refTableSchema
			fk.RefTable = refTableName
			fk.RefColumn = refColumnName
			table.Fk[columnName] = fk
		}
	}
}

// getColumns retrieve columns details from information_schema
// and fill in the Column struct
func (mysqlDB *MysqlDB) GetColumns(db *sql.DB, table *Table, blackList map[string]bool) {
	// retrieve columns
	colDefRows, _ := db.Query(
		`SELECT
			column_name, data_type, column_type, is_nullable, column_default, extra, column_comment 
		FROM
			information_schema.columns
		WHERE
			table_schema = database() AND table_name = ?`,
		table.Name)
	defer colDefRows.Close()
	for colDefRows.Next() {
		// datatype as bytes so that SQL <null> values can be retrieved
		var colNameBytes, dataTypeBytes, columnTypeBytes, isNullableBytes, columnDefaultBytes, extraBytes, columnCommentBytes []byte
		if err := colDefRows.Scan(&colNameBytes, &dataTypeBytes, &columnTypeBytes, &isNullableBytes, &columnDefaultBytes, &extraBytes, &columnCommentBytes); err != nil {
			fmt.Printf("[ERRO] Could not query INFORMATION_SCHEMA for column information\n")
			os.Exit(2)
		}
		colName, dataType, columnType, isNullable, columnDefault, extra, columnComment :=
			string(colNameBytes), string(dataTypeBytes), string(columnTypeBytes), string(isNullableBytes), string(columnDefaultBytes), string(extraBytes), string(columnCommentBytes)
		// create a column
		col := new(Column)
		col.Name = LittleCamelCase(colName)
		col.Type = mysqlDB.GetJavaDataType(dataType)
		// Tag info
		tag := new(OrmTag)
		tag.Column = colName
		tag.Comment = columnComment
		if table.Pk == colName {
			table.PkType = col.Type
			//col.Name = "Id"
			//col.Type = "int"
			tag.Pk = true
			if extra == "auto_increment" {
				tag.Auto = true
			}
		} else {
			fkCol, isFk := table.Fk[colName]
			isBl := false
			if isFk {
				_, isBl = blackList[fkCol.RefTable]
			}
			// check if the current column is a foreign key
			if isFk && !isBl {
				tag.RelFk = true
				refStructName := fkCol.RefTable
				col.Name = BigCamelCase(colName)
				col.Type = "*" + BigCamelCase(refStructName)
			} else {
				// if the name of column is Id, and it's not primary key
				if colName == "id" {
					col.Name = "Id_RENAME"
				}
				if isNullable == "YES" {
					tag.Null = true
				}
				if isSQLSignedIntType(dataType) {
					sign := extractIntSignness(columnType)
					if sign == "unsigned" && extra != "auto_increment" {
						col.Type = mysqlDB.GetJavaDataType(dataType + " " + sign)
					}
				}
				if isSQLStringType(dataType) {
					tag.Size = extractColSize(columnType)
				}
				if isSQLTemporalType(dataType) {
					tag.Type = dataType
					//check auto_now, auto_now_add
					if columnDefault == "CURRENT_TIMESTAMP" && extra == "on update CURRENT_TIMESTAMP" {
						tag.AutoNow = true
					} else if columnDefault == "CURRENT_TIMESTAMP" {
						tag.AutoNowAdd = true
					}
					// need to import time package
					table.ImportTimePkg = true
				}
				if isSQLDecimal(dataType) {
					tag.Digits, tag.Decimals = extractDecimal(columnType)
				}
				if isSQLBinaryType(dataType) {
					tag.Size = extractColSize(columnType)
				}
				if isSQLBitType(dataType) {
					tag.Size = extractColSize(columnType)
				}
			}
		}
		col.Tag = tag
		table.Columns = append(table.Columns, col)
	}
}

// 把sql类型转化成java类型
func (*MysqlDB) GetJavaDataType(sqlType string) (javaType string) {
	var typeMapping = map[string]string{}
	typeMapping = typeMappingMysql
	if v, ok := typeMapping[sqlType]; ok {
		return v
	}
	fmt.Printf("[ERRO] data type (%s) not found!\n", sqlType)
	os.Exit(2)
	return javaType
}

// 创建mvc层次目录
func createPaths(paths *MvcPath) {
	os.Mkdir(paths.ControllerPath, 0777)
	os.Mkdir(paths.ServicePath, 0777)
	os.Mkdir(paths.DaoPath, 0777)
	os.Mkdir(paths.ModelPath, 0777)
	os.Mkdir(paths.ExceptionPath, 0777)
}

func writeSourceFiles(group string, tables []*Table, paths *MvcPath, selectedTables map[string]bool) {
	writeControllerFiles(tables, paths.ControllerPath, selectedTables, group)
	writeServiceFiles(tables, paths.ServicePath, selectedTables, group)
	writeDaoFiles(tables, paths.DaoPath, selectedTables, group)
	writeModelFiles(tables, paths.ModelPath, selectedTables, group)
	writeExceptionFiles(tables, paths.ExceptionPath, selectedTables, group)
}

// 根据ControllerTPL生成controller文件
func writeControllerFiles(tables []*Table, mPath string, selectedTables map[string]bool, group string) {
	for _, tb := range tables {
		// if selectedTables map is not nil and this table is not selected, ignore it
		if selectedTables != nil {
			if _, selected := selectedTables[tb.Name]; !selected {
				continue
			}
		}
		filename := BigCamelCase(tb.Name)
		fpath := path.Join(mPath, filename+"Controller.java")
		var f *os.File
		var err error
		if isExist(fpath) {
			fmt.Printf("[WARN] '%v' already exists. Do you want to overwrite it? [Yes|No] ", fpath)
			if askForConfirmation() {
				f, err = os.OpenFile(fpath, os.O_RDWR|os.O_TRUNC, 0666)
				if err != nil {
					fmt.Printf("[WARN] %v\n", err)
					continue
				}
			} else {
				fmt.Printf("[WARN] Skipped create file '%s'\n", fpath)
				continue
			}
		} else {
			f, err = os.OpenFile(fpath, os.O_CREATE|os.O_RDWR, 0666)
			if err != nil {
				fmt.Printf("[WARN] %v\n", err)
				continue
			}
		}

		template := ""
		template = ControllerTPL
		fileStr := strings.Replace(template, "{{groupPath}}", group, -1)
		fileStr = strings.Replace(fileStr, "{{BigModelName}}", BigCamelCase(tb.Name), -1)
		fileStr = strings.Replace(fileStr, "{{LittleModelName}}", LittleCamelCase(tb.Name), -1)
		fileStr = strings.Replace(fileStr, "{{AllLittleModelName}}", strings.ToLower(LittleCamelCase(tb.Name)), -1)
		fileStr = strings.Replace(fileStr, "{{BigPkName}}", BigCamelCase(tb.Pk), -1)
		fileStr = strings.Replace(fileStr, "{{PkTypeName}}", tb.PkType, -1)

		if _, err := f.WriteString(fileStr); err != nil {
			fmt.Printf("[ERRO] Could not write controller file to %s\n", fpath)
			os.Exit(2)
		}
		CloseFile(f)
	}
}

// 根据ServiceTPL生成Service文件
func writeServiceFiles(tables []*Table, mPath string, selectedTables map[string]bool, group string) {
	for _, tb := range tables {
		// if selectedTables map is not nil and this table is not selected, ignore it
		if selectedTables != nil {
			if _, selected := selectedTables[tb.Name]; !selected {
				continue
			}
		}
		filename := BigCamelCase(tb.Name)
		fpath := path.Join(mPath, filename+"Service.java")
		var f *os.File
		var err error
		if isExist(fpath) {
			fmt.Printf("[WARN] '%v' already exists. Do you want to overwrite it? [Yes|No] ", fpath)
			if askForConfirmation() {
				f, err = os.OpenFile(fpath, os.O_RDWR|os.O_TRUNC, 0666)
				if err != nil {
					fmt.Printf("[WARN] %v\n", err)
					continue
				}
			} else {
				fmt.Printf("[WARN] Skipped create file '%s'\n", fpath)
				continue
			}
		} else {
			f, err = os.OpenFile(fpath, os.O_CREATE|os.O_RDWR, 0666)
			if err != nil {
				fmt.Printf("[WARN] %v\n", err)
				continue
			}
		}

		template := ""
		template = ServiceTPL
		fileStr := strings.Replace(template, "{{groupPath}}", group, -1)
		fileStr = strings.Replace(fileStr, "{{BigModelName}}", BigCamelCase(tb.Name), -1)
		fileStr = strings.Replace(fileStr, "{{LittleModelName}}", LittleCamelCase(tb.Name), -1)
		fileStr = strings.Replace(fileStr, "{{BigPkName}}", BigCamelCase(tb.Pk), -1)
		fileStr = strings.Replace(fileStr, "{{PkTypeName}}", tb.PkType, -1)

		if _, err := f.WriteString(fileStr); err != nil {
			fmt.Printf("[ERRO] Could not write service file to %s\n", fpath)
			os.Exit(2)
		}
		CloseFile(f)
	}
}

// 根据DaoTPL生成Dao文件
func writeDaoFiles(tables []*Table, mPath string, selectedTables map[string]bool, group string) {
	for _, tb := range tables {
		// if selectedTables map is not nil and this table is not selected, ignore it
		if selectedTables != nil {
			if _, selected := selectedTables[tb.Name]; !selected {
				continue
			}
		}
		filename := BigCamelCase(tb.Name)
		fpath := path.Join(mPath, filename+"Dao.java")
		var f *os.File
		var err error
		if isExist(fpath) {
			fmt.Printf("[WARN] '%v' already exists. Do you want to overwrite it? [Yes|No] ", fpath)
			if askForConfirmation() {
				f, err = os.OpenFile(fpath, os.O_RDWR|os.O_TRUNC, 0666)
				if err != nil {
					fmt.Printf("[WARN] %v\n", err)
					continue
				}
			} else {
				fmt.Printf("[WARN] Skipped create file '%s'\n", fpath)
				continue
			}
		} else {
			f, err = os.OpenFile(fpath, os.O_CREATE|os.O_RDWR, 0666)
			if err != nil {
				fmt.Printf("[WARN] %v\n", err)
				continue
			}
		}
		template := ""
		template = DaoTPL

		fileStr := strings.Replace(template, "{{groupPath}}", group, -1)
		fileStr = strings.Replace(fileStr, "{{daoStruct}}", tb.GetDaoStruct(), 1)
		if _, err := f.WriteString(fileStr); err != nil {
			fmt.Printf("[ERRO] Could not write dao file to %s\n", fpath)
			os.Exit(2)
		}
		CloseFile(f)
	}
}

// 根据ExceptionTPL生成exception文件
func writeExceptionFiles(tables []*Table, mPath string, selectedTables map[string]bool, group string) {
	for _, tb := range tables {
		// if selectedTables map is not nil and this table is not selected, ignore it
		if selectedTables != nil {
			if _, selected := selectedTables[tb.Name]; !selected {
				continue
			}
		}
		filename := BigCamelCase(tb.Name)
		fpath := path.Join(mPath, filename+"NotFound.java")
		var f *os.File
		var err error
		if isExist(fpath) {
			fmt.Printf("[WARN] '%v' already exists. Do you want to overwrite it? [Yes|No] ", fpath)
			if askForConfirmation() {
				f, err = os.OpenFile(fpath, os.O_RDWR|os.O_TRUNC, 0666)
				if err != nil {
					fmt.Printf("[WARN] %v\n", err)
					continue
				}
			} else {
				fmt.Printf("[WARN] Skipped create file '%s'\n", fpath)
				continue
			}
		} else {
			f, err = os.OpenFile(fpath, os.O_CREATE|os.O_RDWR, 0666)
			if err != nil {
				fmt.Printf("[WARN] %v\n", err)
				continue
			}
		}
		template := ""
		template = ExceptionTPL

		fileStr := strings.Replace(template, "{{groupPath}}", group, -1)
		fileStr = strings.Replace(fileStr, "{{modelName}}", BigCamelCase(tb.Name), -1)
		if _, err := f.WriteString(fileStr); err != nil {
			fmt.Printf("[ERRO] Could not write exception file to %s\n", fpath)
			os.Exit(2)
		}
		CloseFile(f)
	}
}

// 根据ModelTPL生成model文件
func writeModelFiles(tables []*Table, mPath string, selectedTables map[string]bool, group string) {
	for _, tb := range tables {
		// if selectedTables map is not nil and this table is not selected, ignore it
		if selectedTables != nil {
			if _, selected := selectedTables[tb.Name]; !selected {
				continue
			}
		}
		filename := BigCamelCase(tb.Name)
		fpath := path.Join(mPath, filename+".java")
		var f *os.File
		var err error
		if isExist(fpath) {
			fmt.Printf("[WARN] '%v' already exists. Do you want to overwrite it? [Yes|No] ", fpath)
			if askForConfirmation() {
				f, err = os.OpenFile(fpath, os.O_RDWR|os.O_TRUNC, 0666)
				if err != nil {
					fmt.Printf("[WARN] %v\n", err)
					continue
				}
			} else {
				fmt.Printf("[WARN] Skipped create file '%s'\n", fpath)
				continue
			}
		} else {
			f, err = os.OpenFile(fpath, os.O_CREATE|os.O_RDWR, 0666)
			if err != nil {
				fmt.Printf("[WARN] %v\n", err)
				continue
			}
		}
		template := ""
		template = ModelTPL
		fileStr := strings.Replace(template, "{{groupPath}}", group, -1)
		fileStr = strings.Replace(fileStr, "{{modelStruct}}", tb.GetModelStruct(), 1)
		//fileStr = strings.Replace(fileStr, "{{modelName}}", BigCamelCase(tb.Name), -1)
		fileStr = strings.Replace(fileStr, "{{tableName}}", tb.Name, -1)
		// if table contains time field, import time.Time package

		importTimePkg := ""
		if tb.ImportTimePkg {
			importTimePkg = "import java.util.Date;"
		}
		fileStr = strings.Replace(fileStr, "{{importTimePkg}}", importTimePkg, -1)
		if _, err := f.WriteString(fileStr); err != nil {
			fmt.Printf("[ERRO] Could not write model file to %s\n", fpath)
			os.Exit(2)
		}
		CloseFile(f)
	}
}

const (
	ModelTPL = `package {{groupPath}}.model;

import javax.persistence.*;

{{importTimePkg}}
import io.swagger.annotations.ApiModelProperty;
import lombok.Data;

@Entity
@Table(name="{{tableName}}")
@Data
{{modelStruct}}
`

	DaoTPL = `package {{groupPath}}.dao;

import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.JpaSpecificationExecutor;
import {{groupPath}}.model.*;

{{daoStruct}}
`

	ExceptionTPL = `package {{groupPath}}.exception;

public class {{modelName}}NotFound extends Exception {

}
`

	ServiceTPL = `package {{groupPath}}.service;

import {{groupPath}}.dao.*;
import {{groupPath}}.exception.*;
import {{groupPath}}.model.*;
import org.springframework.beans.factory.annotation.Autowired;

import org.springframework.data.domain.Page;
import org.springframework.data.domain.PageRequest;
import org.springframework.data.domain.Sort;
import org.springframework.data.domain.Sort.Direction;
import org.springframework.data.domain.Sort.Order;
import org.springframework.data.jpa.domain.Specification;

import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import javax.persistence.criteria.*;
import java.util.ArrayList;
import java.util.List;

@Service
public class {{BigModelName}}Service {

    @Autowired
    private {{BigModelName}}Dao {{LittleModelName}}Dao;

    @Transactional
    public {{BigModelName}} add{{BigModelName}}({{BigModelName}} {{LittleModelName}}) {
        return {{LittleModelName}}Dao.save({{LittleModelName}});
    }

    @Transactional
    public {{BigModelName}} update{{BigModelName}}({{BigModelName}} {{LittleModelName}}) throws {{BigModelName}}NotFound {
        {{BigModelName}} {{LittleModelName}}Update = {{LittleModelName}}Dao.findOne({{LittleModelName}}.get{{BigPkName}}());
        if ({{LittleModelName}}Update==null){
            throw new {{BigModelName}}NotFound();
        }
        /*TODO: need add logic eg:
        if (user.getName()!=null) {
            userUpdate.setName(user.getName());
        }
        if (user.getAge()!=0) {
            userUpdate.setAge(user.getAge());
        }
        */
        {{LittleModelName}}Dao.save({{LittleModelName}}Update);
        return {{LittleModelName}}Update;
    }

    @Transactional
    public void delete{{BigModelName}}By{{BigPkName}}({{PkTypeName}} id) throws {{BigModelName}}NotFound {
        {{BigModelName}} {{LittleModelName}}Delete = {{LittleModelName}}Dao.findOne(id);
        if ({{LittleModelName}}Delete==null) {
            throw new {{BigModelName}}NotFound();
        }
        {{LittleModelName}}Dao.delete(id);
    }

    public {{BigModelName}} get{{BigModelName}}By{{BigPkName}}({{PkTypeName}} id) {
        return {{LittleModelName}}Dao.findOne(id);
    }

	public Page<{{BigModelName}}> getAll{{BigModelName}}(String query, /*String fields, */String sortby,Integer page, Integer pageSize) {
        if (page == null) {
            page = 0;
        }
        if (pageSize == null) {
            pageSize = 10;
        }

        PageRequest pageRequest;
        if (sortby != null && sortby.length() > 0) {
            List<Order> orders = new ArrayList<Order>();
            String[] sortFields = sortby.split(",");
            for (String sortField : sortFields) {
                String[] orderbys = sortField.split(":");
                if(orderbys[1].equals("desc")) {
                    orders.add(new Order(Sort.Direction.DESC,orderbys[0]));
                } else if (orderbys[1].equals("asc")) {
                    orders.add(new Order(Sort.Direction.ASC, orderbys[0]));
                }
            }
            pageRequest = new PageRequest(page, pageSize,new Sort(orders));
        }else{
            pageRequest = new PageRequest(page,pageSize);
        }

        return {{LittleModelName}}Dao.findAll(new Specification<{{BigModelName}}>() {
            @Override
            public Predicate toPredicate(Root<{{BigModelName}}> root, CriteriaQuery<?> cq, CriteriaBuilder cb) {
                List<Predicate> preList = new ArrayList<Predicate>();
                if (query != null && query.length() > 0) {
                    String[] queryFields = query.split(",");
                    for (String queryField : queryFields) {
                        String[] queryKv = queryField.split(":");
                        preList.add(cb.equal(root.get(queryKv[0]),queryKv[1]));
                    }
                }
                return cq.where(preList.toArray(new Predicate[preList.size()])).getRestriction();
            }
        },pageRequest);
    }
}
`

	ControllerTPL = `package {{groupPath}}.controller;

import {{groupPath}}.exception.*;
import {{groupPath}}.model.*;
import {{groupPath}}.service.{{BigModelName}}Service;
import io.swagger.annotations.*;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.data.domain.Page;
import org.springframework.http.MediaType;
import org.springframework.web.bind.annotation.*;

import java.util.List;

@RestController
@RequestMapping("/{{AllLittleModelName}}")
@Api(value = "{{BigModelName}}Controller", description = "oprations for {{BigModelName}}")
public class {{BigModelName}}Controller {
    @Autowired
    private {{BigModelName}}Service {{LittleModelName}}Service;

    @ApiOperation(value="add{{BigModelName}}", notes="Create {{BigModelName}}")
    @RequestMapping(value="", method= RequestMethod.POST,produces = MediaType.APPLICATION_JSON_VALUE)
    public {{BigModelName}} add{{BigModelName}}(@RequestBody {{BigModelName}} {{LittleModelName}}) {
        return {{LittleModelName}}Service.add{{BigModelName}}({{LittleModelName}});
    }

    @ApiOperation(value="update{{BigModelName}}", notes="Update {{BigModelName}}")
    @RequestMapping(value="", method=RequestMethod.PUT,produces = MediaType.APPLICATION_JSON_VALUE)
    public {{BigModelName}} update{{BigModelName}}(@RequestBody {{BigModelName}} {{LittleModelName}}) throws {{BigModelName}}NotFound {
        return {{LittleModelName}}Service.update{{BigModelName}}({{LittleModelName}});
    }

    @ApiOperation(value="delete{{BigModelName}}By{{BigPkName}}", notes="Delete {{BigModelName}} By {{BigPkName}}")
    @RequestMapping(value="/{id}", method=RequestMethod.DELETE)
    public String delete{{BigModelName}}By{{BigPkName}}(@PathVariable {{PkTypeName}} id) throws {{BigModelName}}NotFound{
        {{LittleModelName}}Service.delete{{BigModelName}}By{{BigPkName}}(id);
        return "ok";
    }

    @ApiOperation(value="get{{BigModelName}}", notes="Get {{BigModelName}} Info By {{BigPkName}}")
    @RequestMapping(value="/{id}", method=RequestMethod.GET,produces = MediaType.APPLICATION_JSON_VALUE)
    public {{BigModelName}} get{{BigModelName}}(@PathVariable {{PkTypeName}} id) {
        return {{LittleModelName}}Service.get{{BigModelName}}By{{BigPkName}}(id);
    }

	@ApiOperation(value="getAll{{BigModelName}}", notes="Get All {{BigModelName}} info")
    @ApiImplicitParams({ @ApiImplicitParam(paramType = "query",name = "query",required = false, value = "Filter. e.g. col1:v1,col2:v2 ...",dataType = "String"),
            /*@ApiImplicitParam(paramType = "query",name = "fields",required = false,value = "Fields returned. e.g. col1,col2 ...",dataType = "String"),*/
            @ApiImplicitParam(paramType = "query",name = "sortby",required = false,value = "Order corresponding to each sortby field. e.g. col1:desc,col2:asc ...",dataType = "String"),
            @ApiImplicitParam(paramType = "query",name = "page",required = false,value = "Limit the size of result set. Must be an integer",dataType = "Int"),
            @ApiImplicitParam(paramType = "query",name = "pagesize",required = false,value = "Start position of result set. Must be an integer",dataType = "Int")})
    @RequestMapping(value="", method=RequestMethod.GET,produces = MediaType.APPLICATION_JSON_VALUE)
    public Page<{{BigModelName}}> getAll{{BigModelName}}(@RequestParam(value = "query",required = false) String query,
                                 /*@RequestParam(value = "fields",required = false) String fields,*/
                                 @RequestParam(value = "sortby",required = false) String sortby,
                                 @RequestParam(value = "page",required = false) Integer page,
                                 @RequestParam(value = "pagesize",required = false) Integer pagesize) {
        return {{LittleModelName}}Service.getAll{{BigModelName}}(query,/*fields,*/sortby,page,pagesize);
    }
}
`
)
