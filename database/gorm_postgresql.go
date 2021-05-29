// +build postgresql

package database

import (
	"github.com/Etpmls/Etpmls-Micro/v2/define"
	"github.com/Etpmls/Etpmls-Micro/v3/define"
	em_library "github.com/Etpmls/Etpmls-Micro/v3/library"
	em "github.com/Etpmls/Etpmls-Micro/v3"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
	"strings"
)

var DB *gorm.DB

const (
	FUZZY_SEARCH = "ILIKE"
	
	// Service Database
	KvServiceDatabase         = "/database/"		// /service/rpcName/database/
	KvServiceDatabaseEnable   = "/database/enable"
	KvServiceDatabaseHost     = "/database/host"
	KvServiceDatabaseUser     = "/database/user"
	KvServiceDatabasePassword = "/database/password"
	KvServiceDatabaseDbName   = "/database/dbname"
	KvServiceDatabasePort     = "/database/port"
	KvServiceDatabaseTimezone = "/database/timezone"
	KvServiceDatabasePrefix   = "/database/prefix"
)

func init()  {

}

func (this *database) runDatabase() {

	dsn := "host=" + host + " user=" + user + " password=" + password + " dbname=" + dbname + " port=" + port + " sslmode=disable TimeZone=" + timezone

	//Connect Database
	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			TablePrefix: prefix,
		},
	})
	if err != nil {
		em.LogPanic.Path("Unable to connect to the database!", err)
	}

	err = DB.AutoMigrate(migrate...)
	if err != nil {
		em.LogInfo.Path("Failed to create database!", err)
	}

}