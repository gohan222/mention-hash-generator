package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"time"

	jc "github.com/codisms/json-config"
	"github.com/coopernurse/gorp"
	"github.com/inspirent/go-spooky"
	loggerLib "github.com/inspirent/logger"
)

var configFileName = flag.String("conf", "../test.conf", "Pass in a config file")
var configSubsection = flag.String("conf-section", "", "The config file subsection to use")
var config *jc.Config = getConfig()

var logger loggerLib.Logger = getLogger(config)
var dbmap *gorp.DbMap = getDb()
var batchSize int
var sleepTime time.Duration

func main() {

	defer dbmap.Db.Close()

	startTime := time.Now()
	logger.Infof("*****START hash process: %+v\n", startTime.UTC())
	run()
	logger.Infof("*****END hash process: %+v\n", time.Now().UTC())
	logger.Infof("*****Delta Time: %+v\n", time.Now().Sub(startTime))
}

func getConfig() *jc.Config {

	var err error

	flag.Parse()

	if configFileName == nil || *configFileName == "" {
		log.Fatalf("Config file not specified")
	}

	c, err := jc.LoadConfig(*configFileName)
	if err != nil {
		log.Fatalf("Unable to load config file '%v': %v\n", *configFileName, err)
	}
	if configSubsection != nil && *configSubsection != "" {
		var found bool
		c, found = c.GetObject(*configSubsection)
		if !found {
			log.Fatalf("Unable to load config section '%v' of file '%v': %v\n", *configSubsection, *configFileName, err)
		}
	}
	return c
}

func getLogger(config *jc.Config) loggerLib.Logger {

	l, err := loggerLib.New(config)
	if err != nil {
		log.Fatalf("Unable to set up logging: %v", err)
	}

	return l
}

func getDb() *gorp.DbMap {
	dbm, err := initDb()
	if err != nil {
		logger.Errorf("Unable to connect to postgres: %v\n", err)
		log.Fatalf("Unable to connect to postgres: %v\n", err)
	}
	return dbm
}

func initSettings() {
	var err error
	var found bool

	batchSize, found = config.GetInt("batchSize")
	if !found {
		config.SettingNotFound("sleepTime not found in config ")
	}

	sleepTimeString, found := config.GetString("sleepTime")
	if !found {
		config.SettingNotFound("sleepTime not found in config ")
	}

	sleepTime, err = time.ParseDuration(sleepTimeString)
	if err != nil {
		config.SettingNotFound("sleepTime is in wrong format. ex. 3s for 3 second sleep")
	}
}

func run() {
	initSettings()

	//flag for while loop
	recordCount, err := getRecordCount()
	if err != nil {
		logger.Errorf("Unable to record count: %v\n", err)
		return
	}

	if recordCount <= 0 {
		logger.Error("Record count is zero.")
		return
	}

	//calc number of segments
	numberOfSegments := int(math.Ceil(float64(recordCount) / float64(batchSize)))
	logger.Infof("************Record count: %d", numberOfSegments)

	for i := 0; i < numberOfSegments; i++ {

		logger.Info("************Getting next batch.")
		//get the next batch of
		mentions, err := getSegment(batchSize)
		if err != nil {
			logger.Errorf("Unable to create transcation: %v\n", err)
			return
		}

		if mentions == nil {
			logger.Error("Mentions array was nil.")
			break
		}

		if len(mentions) == 0 {
			logger.Info("Mentions array is empty.")
			break
		}

		for _, dbMention := range mentions {
			if dbMention == nil || dbMention.Snippets == nil {
				logger.Error("Nil mention encountered.")
				continue
			}

			if dbMention.Snippets == nil {
				logger.Errorf("Nil snippet found: %+v")
				continue
			}

			hash := createHash(*dbMention.Snippets)
			// logger.Infof("Hash value: %d", *hash)
			err := updateMentionHash(*dbMention.MentionId, hash)
			if err != nil {
				logger.Errorf("Unable update mention hash %v: %v\n", *dbMention.MentionId, err)
				continue
			}
			time.Sleep(sleepTime)
		}

		logger.Infof("Records processed length %+v\n", len(mentions))
		break
	}
}

func getRecordCount() (int64, error) {
	sqlCommand := `SELECT 
						count(m.mention_id)
					FROM 
						mention m
					WHERE
						m.mention_hash IS NULL
					`
	startTime := time.Now().UTC()
	recordCount, err := dbmap.SelectInt(sqlCommand)
	if err != nil {
		logger.Errorf("Unable to get record count: %v\n", err)
		return -1, err
	}
	logger.Infof("*****Time to retrieve count: %+v\n", time.Now().Sub(startTime))

	return recordCount, nil
}

func getSegment(size int) ([]*DbMention, error) {
	var dbMentions []*DbMention

	sqlCommand := `SELECT 
						m.mention_id,
						m.mention_snippets 
					FROM 
						mention m
					WHERE
						m.mention_hash IS NULL
					ORDER BY 
						m.mention_id DESC
					LIMIT 
						$1
					`
	startTime := time.Now().UTC()
	_, err := dbmap.Select(&dbMentions, sqlCommand, size)
	if err != nil {
		logger.Errorf("Unable get mentions list: %v\n", err)
		return nil, err
	}
	logger.Infof("*****Time to retrieve segment: %+v\n", time.Now().Sub(startTime))
	return dbMentions, nil
}

func updateMentionHash(mentionId int64, hash int64) error {
	if mentionId < 0 {
		logger.Errorf("Mention Id is invalid: %v\n", mentionId)
		return fmt.Errorf("Mention Id is invalid: %v\n", mentionId)
	}

	sqlCommand := `	UPDATE
						mention
					SET 
						mention_hash = $1
					WHERE
						mention_id = $2
					`
	startTime := time.Now().UTC()
	_, err := dbmap.Exec(sqlCommand, hash, mentionId)
	if err != nil {
		logger.Errorf("Unable update mention: %v\n", err)
		return err
	}

	logger.Infof("*****Time to update mention: %+v\n", time.Now().Sub(startTime))

	return nil
}

func createHash(key string) int64 {
	var hash int64 = int64(spooky.Hash32([]byte(key)))
	return hash
}

func openTransaction() (*gorp.Transaction, error) {
	trans, err := dbmap.Begin()
	if err != nil {
		logger.Errorf("Unable to create transcation: %v\n", err)
		return nil, err
	}

	return trans, nil
}

func commitTransaction(trans *gorp.Transaction) {
	if trans == nil {
		logger.Errorf("Transaction object is nil.")
	}

	if err := trans.Commit(); err != nil {
		logger.Errorf("Unable to commit transaction: %v\n", err)

		if rErr := trans.Rollback(); rErr != nil {
			logger.Errorf("Unable to roll back transaction: %v\n", rErr)
		}
	}
}
