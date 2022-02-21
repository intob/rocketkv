package store

import (
	"encoding/base64"
	"encoding/gob"
	"log"
	"math/rand"
	"os"
	"path"
	"time"
)

const replFileName = "repl.gob"

type ReplClient struct {
	State           *ReplClientState
	HeadIncremented bool
	Store           *Store
	Dir             string
}

type ReplClientState struct {
	Id   []byte
	Head int
}

type ReplClientConfig struct {
	Network    string
	Address    string
	AuthSecret string
}

// spin up client & connect to master
func (rc *ReplClient) Init(cfg *ReplClientConfig) {
	rc.ensureStateFile()
	go rc.writeStateToFilePeriodically()
}

// ensures that repl file exists
func (rc *ReplClient) ensureStateFile() {
	replFilePath := path.Join(rc.Dir, replFileName)
	replFile, err := os.Open(replFilePath)
	if err != nil {
		log.Println("no part list found, will create...")
		// make new file
		newReplFile, err := os.Create(replFilePath)
		if err != nil {
			log.Fatalf("failed to create repl file, check directory exists: %s", rc.Dir)
		}

		// generate new id
		rc.State = &ReplClientState{}
		rc.State.Id = make([]byte, 32)
		rand.Seed(time.Now().UnixMicro())
		rand.Read(rc.State.Id)

		gob.NewEncoder(newReplFile).Encode(rc.State)
	} else {
		// decode list
		state := ReplClientState{}
		err := gob.NewDecoder(replFile).Decode(&state)
		if err != nil {
			log.Fatalf("failed to decode repl file: %s", err)
		}
		rc.State = &state
	}
	log.Printf("initialised repl client with id %s and head %v\r\n",
		base64.RawStdEncoding.EncodeToString(rc.State.Id), rc.State.Head)
}

func (rc *ReplClient) writeStateToFilePeriodically() {
	for {
		if rc.HeadIncremented {
			rc.writeStateToFile()
		}
		time.Sleep(time.Duration(10) * time.Second)
	}
}

func (rc *ReplClient) writeStateToFile() {
	fullPath := path.Join(rc.Dir, replFileName)
	file, err := os.Create(fullPath)
	if err != nil {
		log.Fatalf("failed to create repl file: %s\r\n", err)
	}
	gob.NewEncoder(file).Encode(&rc.State)
	file.Close()
	rc.HeadIncremented = false
}
