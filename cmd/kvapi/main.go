package main 

import (
	"bytes"
	crypto "crypto/rand"
	"encoding/binary"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/PraneethV-cmd/protoraft"
)


// <------------------>
//	All structs
// <------------------>

type StateMachine struct {
	db	*sync.Map //the map used here is for conncurrent synchronized key value storage
	server	int 
}

type Command struct {
	kind	CommandKind
	key	string
	value	string
}

type HTTPServer struct {
	raft	*protoraft.Server
	db	*sync.Map
}

type CommandKind uint8 

const (
	setCommand CommandKind = iota 
	getCommand
)

// a statemachine that will interact with the comands that users send 
// here we are using a decode command function that weill decode the msg that was 
// send as byte stream and see what type of command it is 

func(s *StateMachine) Apply(cmd []byte) ([]byte, error) {
	c := decodeCommand(cmd)

	switch c.kind {
		case setCommand:
			s.db.Store(c.key, c.value)
		case getCommand:
			value, ok := s.db.Load(c.key)
			if !ok {
				return nil, fmt.Errorf("Key not found")
			}
			return []byte(value.(string)), nil
		default:
			return nil, fmt.Errorf("Unknown Command: %x, cmd")
	}

	return nil, nil
}

// since the raft or the KV is in a distributed env, we need to send the command to either one of the KV 
// and we do this by encoding the command we are sending as a byte stream 

func encodeCommand(c Command) []byte {
	msg := bytes.NewBuffer(nil)
	err := msg.WriteByte(uint8(c.kind))
	if err != nil {
		panic(err)
	}

	err = binary.Write(msg, binary.LittleEndian, uint64(len(c.key))) 
	if err != nil {
		panic(err)
	}

	msg.WriteString(c.key)
	
	err = binary.Write(msg, binary.LittleEndian, uint64(len(c.value))) 
	if err != nil {
		panic(err)
	}

	msg.WriteString(c.value)

	return msg.Bytes()
}

// we are sending the command as a byte stream 
// now when the byte stream reaches the statemachine we have to be able to 
// decode it and find the type of the command it is 

func decodeCommand(msg []byte) Command {
	var c Command
	c.kind = CommandKind(msg[0])

	keyLen := binary.LittleEndian.Uint64(msg[1:9])
	c.key = string(msg[9 : 9+keyLen])

	if c.kind == setCommand {
		valLen := binary.LittleEndian.Uint64(msg[9+keyLen : 9+keyLen+8])
		c.value = string(msg[9+keyLen+8 : 9+keyLen+8+valLen])
	}

	return c
}

//exmplae : curl http://localhost:8080/set?key=x&value=1
func(hs HTTPServer) setHandler(w http.ResponseWriter, r *http.Request) {
	var c Command
	c.kind = setCommand
	c.key = r.URL.Query().Get("key")
	c.value = r.URL.Query().Get("value")

	_, err := hs.raft.Apply([][]byte{encodeCommand(c)})
	if err != nil {
		log.Printf("Could not write key-value: %s", err)
		return
	}
}
