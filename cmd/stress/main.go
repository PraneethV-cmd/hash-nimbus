package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/PraneethV-cmd/hash-nimbus"
	"github.com/pkg/profile"
)

type kvStateMachine struct {
	mu sync.Mutex
	kv map[string]string
}

func newKvSM() *kvStateMachine {
	return &kvStateMachine{
		kv: make(map[string]string, 0),
	}
}

func encodeKvsmMessage_Get(key string) []byte {
	msg := make([]byte,
		3+ // Message type
			8+ // Key length
			8+ // Empty space
			len(key), // Key
	)
	copy(msg[:3], []byte("get"))
	binary.LittleEndian.PutUint64(msg[3:11], uint64(len(key)))
	copy(msg[19:19+len(key)], key)
	return msg
}

func decodeKvsmMessage_Get(msg []byte) (bool, string) {
	if len(msg) < 3 {
		return false, ""
	}

	msgType := msg[:3]
	if !bytes.Equal(msgType, []byte("get")) {
		return false, ""
	}

	keyLen := binary.LittleEndian.Uint64(msg[3:11])
	key := string(msg[19 : 19+keyLen])

	return true, key
}

func encodeKvsmMessage_Set(key, value string) []byte {
	msg := make([]byte,
		3+ // Message type
			8+ // Key length
			8+ // Value length
			len(key)+ // Key
			len(value), // Value
	)
	copy(msg[:3], []byte("set"))
	binary.LittleEndian.PutUint64(msg[3:11], uint64(len(key)))
	binary.LittleEndian.PutUint64(msg[11:19], uint64(len(value)))
	copy(msg[19:19+len(key)], key)
	copy(msg[19+len(key):19+len(key)+len(value)], value)
	return msg
}

func decodeKvsmMessage_Set(msg []byte) (bool, string, string) {
	if len(msg) < 3 {
		return false, "", ""
	}

	msgType := msg[:3]
	if !bytes.Equal(msgType, []byte("set")) {
		return false, "", ""
	}

	keyLen := binary.LittleEndian.Uint64(msg[3:11])
	key := string(msg[19 : 19+keyLen])

	valLen := binary.LittleEndian.Uint64(msg[11:19])
	val := string(msg[19+keyLen : 19+keyLen+valLen])

	return true, key, val
}

func (kvsm *kvStateMachine) Apply(msg []byte) ([]byte, error) {
	kvsm.mu.Lock()
	defer kvsm.mu.Unlock()

	if ok, key, val := decodeKvsmMessage_Set(msg); ok {
		kvsm.kv[key] = val
		return nil, nil
	} else if ok, key := decodeKvsmMessage_Get(msg); ok {
		return []byte(kvsm.kv[key]), nil
	} else {
		panic("Unknown state machine message.")
		return nil, fmt.Errorf("Unknown state machine message: %x", msg)
	}
}

func main() {
	//defer profile.Start(profile.MemProfile).Stop()
	defer profile.Start().Stop()
	rand.Seed(0)

	// Delete any existing .dat files
	entries, err := os.ReadDir("./")
	if err != nil {
		panic(err)
	}

	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".dat") {
			os.Remove(e.Name())
		}
	}

	cluster := []goraft.ClusterMember{
		{
			Id:      1,
			Address: "localhost:2020",
		},
		{
			Id:      2,
			Address: "localhost:2021",
		},
		{
			Id:      3,
			Address: "localhost:2022",
		},
	}

	sm1 := newKvSM()
	sm2 := newKvSM()
	sm3 := newKvSM()

	s1 := goraft.NewServer(cluster, sm1, ".", 0)
	s2 := goraft.NewServer(cluster, sm2, ".", 1)
	s3 := goraft.NewServer(cluster, sm3, ".", 2)

	DEBUG := false

	servers := []*goraft.Server{s1, s2, s3}
	for _, s := range servers {
		s.Debug = DEBUG
		s.Start()
	}
	sms := []*kvStateMachine{sm1, sm2, sm3}[:len(servers)]

	leader := waitForLeader(servers)

	N_CLIENTS := 1
	N_ENTRIES := 1 // 50_000 // / N_CLIENTS
	BATCH_SIZE := goraft.MAX_APPEND_ENTRIES_BATCH / N_CLIENTS
	TIME_BETWEEN_INSERTS := time.Duration(0) //15 * time.Second
	fmt.Printf("Clients: %d. Entries: %d. Batch: %d.\n", N_CLIENTS, N_ENTRIES, BATCH_SIZE)

	var wg sync.WaitGroup
	wg.Add(N_CLIENTS)
	var total time.Duration
	var mu sync.Mutex

	var allEntries [][]byte
	for i := 0; i < N_ENTRIES; i++ {
		key := randomString()
		value := randomString()

		allEntries = append(allEntries, encodeKvsmMessage_Set(key, value))
	}

	// allEntries := [][]byte{
	// 	encodeKvsmMessage_Set("a", "1"),
	// 	encodeKvsmMessage_Set("b", "2"),
	// 	encodeKvsmMessage_Set("c", "3"),
	// }

	debugEntry := func(entry []byte) string {
		if ok, key, val := decodeKvsmMessage_Set(entry); ok {
			return fmt.Sprintf("Key: %s. Value: %s.", key, val)
		} else if ok, key := decodeKvsmMessage_Get(entry); ok {
			return fmt.Sprintf("Key: %s.", key)
		}

		return fmt.Sprintf("Unknown: %x. (len: %d)", entry, len(entry))
	}

	for j := 0; j < N_CLIENTS; j++ {
		go func(j int) {
			defer wg.Done()

			for i := 0; i < N_ENTRIES; i += BATCH_SIZE {
				if TIME_BETWEEN_INSERTS != 0 {
					fmt.Println("Injecting latency between client requests.", TIME_BETWEEN_INSERTS)
					time.Sleep(TIME_BETWEEN_INSERTS)
				}
				end := i + BATCH_SIZE
				if end > len(allEntries) {
					end = len(allEntries)
				}
				batch := allEntries[i:end]
			foundALeader:
				for {
					for _, s := range servers {
						t := time.Now()
						_, err := s.Apply(batch)
						if err == goraft.ErrApplyToLeader {
							continue
						} else if err != nil {
							panic(err)
						} else {
							goraft.Assert("Leader stayed the same", s.Id(), leader)
							diff := time.Now().Sub(t)
							mu.Lock()
							total += diff
							mu.Unlock()
							fmt.Printf("Client: %d. %d entries (%d of %d: %d%%) inserted. Latency: %s. Throughput: %f entries/s.\n",
								j,
								len(batch),
								i+1,
								N_ENTRIES,
								((i+1)*100)/N_ENTRIES,
								diff,
								float64(len(batch))/(float64(diff)/float64(time.Second)),
							)

							break foundALeader
						}
					}
					time.Sleep(time.Second)
				}
			}
		}(j)
	}

	wg.Wait()
	fmt.Printf("Total time: %s. Average insert/second: %s. Throughput: %f entries/s.\n", total, total/time.Duration(N_ENTRIES), float64(N_ENTRIES)/(float64(total)/float64(time.Second)))

	validateAllCommitted(servers)
	validateUserEntries(servers, allEntries, debugEntry)

	// Validate state machines.
	for j, entry := range allEntries {
		_, key, value := decodeKvsmMessage_Set(entry)
		for i := range servers {
			sm := sms[i]
			goraft.Assert(fmt.Sprintf("Server %d state machine is up-to-date on entry %d (%s).", cluster[i].Id, j, key), value, sm.kv[key])
		}
	}

	fmt.Println("Validating get.")

	var v []byte
	_, testKey, testValue := decodeKvsmMessage_Set(allEntries[0])
	for _, s := range servers {
		res, err := s.Apply([][]byte{encodeKvsmMessage_Get(testKey)})
		if err == goraft.ErrApplyToLeader {
			continue
		} else if err != nil {
			panic(err)
		} else {
			goraft.Assert("Leader stayed the same", s.Id(), leader)
			v = res[0].Result
			break
		}
	}

	fmt.Printf("%s = %s, expected: %s\n", testKey, string(v), testValue)

	fmt.Println("Testing shutdown and restart still holds all values.")
	for _, s := range servers {
		s.Shutdown()
	}

	servers[0] = goraft.NewServer(cluster, sm1, ".", 0)
	servers[1] = goraft.NewServer(cluster, sm2, ".", 1)
	servers[2] = goraft.NewServer(cluster, sm3, ".", 2)
	goraft.Assert("Servers are still the same size.", 3, len(servers))
	for _, s := range servers {
		s.Start()
	}

	waitForLeader(servers)
	fmt.Println("Found a leader, validating entries.")

	validateAllCommitted(servers)
	validateUserEntries(
		servers,
		// Need to also compare against the addition of the
		// Get message we issued above since Get messages are
		// also committed to the log.
		append(allEntries, encodeKvsmMessage_Get(testKey)),
		debugEntry,
	)

	fmt.Println("Testing deleted log file on one server still recovers entries.")
	for _, s := range servers {
		s.Shutdown()
	}

	servers[0] = goraft.NewServer(cluster, sm1, ".", 0)
	servers[1] = goraft.NewServer(cluster, sm2, ".", 1)
	servers[2] = goraft.NewServer(cluster, sm3, ".", 2)
	os.Remove(servers[2].Metadata())
	goraft.Assert("Servers are still the same size.", 3, len(servers))
	for _, s := range servers {
		s.Start()
	}

	waitForLeader(servers)

	// TODO: figure out why this is racy.
	time.Sleep(5 * time.Second)

	validateAllCommitted(servers)
	validateUserEntries(
		servers,
		// Need to also compare against the addition of the
		// Get message we issued above since Get messages are
		// also committed to the log.
		append(allEntries, encodeKvsmMessage_Get(testKey)),
		debugEntry,
	)

	fmt.Println("ok")
}

