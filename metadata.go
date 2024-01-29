package main

import (
	"encoding/json"
	"log"

	srvConfig "github.com/CHESSComputing/golib/config"
	mongo "github.com/CHESSComputing/golib/mongo"
	bson "go.mongodb.org/mongo-driver/bson"
)

// metaInsert inserts record into MLHub database
func metaInsert(rec Record) error {
	var records []any
	records = append(records, rec)
	mongo.UpsertAny(
		srvConfig.Config.MLHub.MongoDB.DBName,
		srvConfig.Config.MLHub.MongoDB.DBColl,
		records)
	return nil
}

// metaUpdate updates record in MLHub database
func metaUpdate(rec Record) error {
	spec := bson.M{"model": rec.Model}
	meta := bson.M{"model": rec.Model, "type": rec.Type}
	err := mongo.UpsertRecord(
		srvConfig.Config.MLHub.MongoDB.DBName,
		srvConfig.Config.MLHub.MongoDB.DBColl,
		spec,
		meta)
	return err
}

// metaRemove removes given model from MLHub database
func metaRemove(model string) error {
	spec := bson.M{"name": model}
	mongo.Remove(
		srvConfig.Config.MLHub.MongoDB.DBName,
		srvConfig.Config.MLHub.MongoDB.DBColl,
		spec)
	return nil
}

// metaRecords retrieves records from underlying MLHub database
func metaRecords(model, mlType, version string) ([]Record, error) {
	spec := bson.M{}
	if model != "" {
		spec["model"] = model
	}
	if version != "" {
		spec["version"] = version
	}
	if mlType != "" {
		spec["type"] = mlType
	}
	results := mongo.Get(
		srvConfig.Config.MLHub.MongoDB.DBName,
		srvConfig.Config.MLHub.MongoDB.DBColl,
		spec, 0, -1)
	var records []Record
	for _, rec := range results {
		var r Record
		delete(rec, "_id")
		data, err := json.Marshal(rec)
		if err != nil {
			log.Printf("Umable to marshal record %+v, error %v", rec, err)
			continue
		}
		err = json.Unmarshal(data, &r)
		if err != nil {
			log.Printf("Umable to unmarshal record %+v to Record data-struct, error %v", rec, err)
			continue
		}
		records = append(records, r)
	}
	return records, nil
}
