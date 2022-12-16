package streams

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	"goose/inceptiondb"
)

type Streams struct {
	Inception *inceptiondb.Client
	Prefix    string
	counters  map[string]int64
	wg        *sync.WaitGroup
	stop      bool
}

type JSON = map[string]interface{}

type Counter struct {
	Name string `json:"name"`
	Last int64  `json:"last"`
}

func NewStreams(inception *inceptiondb.Client) *Streams {

	prefix := "streams." // todo: hardcoded!

	err := inception.EnsureIndex(prefix+"counters", &inceptiondb.IndexOptions{
		Name:   "by_name",
		Type:   "btree",
		Fields: []string{"name"},
	})
	if err != nil {
		panic(err) // todo: dont panic!!!!
	}

	data, err := inception.Find(prefix+"counters", inceptiondb.FindQuery{Limit: 99999})
	if err != nil {
		panic(err) // todo: dont panic!!!!
	}

	counters := map[string]int64{}
	d := json.NewDecoder(data)
	for {
		entry := Counter{}
		err := d.Decode(&entry)
		if err == io.EOF {
			break
		}
		if err != nil {
			panic(err) // todo: dont panic!!!!
		}
		counters[entry.Name] = entry.Last
	}

	fmt.Println(counters)

	return &Streams{
		Prefix:    prefix,
		Inception: inception,

		counters: counters,
		wg:       &sync.WaitGroup{},
	}
}

func (s *Streams) Ensure(name string) error {

	err := s.Inception.EnsureCollection(s.Prefix + "stream." + name)
	if err != nil {
		return err
	}

	s.Inception.EnsureIndex(s.Prefix+"stream."+name, &inceptiondb.IndexOptions{
		Name:   "number",
		Type:   "btree",
		Fields: []string{"timestamp"},
	})

	return nil
}

type Entry struct {
	Timestamp int64           `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
}

func (s *Streams) Send(name string, payload interface{}) error { // todo: json.RawMessage instead?

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	entry := Entry{
		Timestamp: time.Now().UnixNano(),
		Payload:   payloadBytes,
	}

	return s.Inception.Insert(s.Prefix+"stream."+name, entry)
}

func (s *Streams) Receive(name string, flow string, callback func(data []byte) error) error {

	s.wg.Add(1)
	defer s.wg.Done()

	counter_name := name + ":" + flow

	// Create counter if does not exist..., not sure if needed
	s.Inception.Insert(s.Prefix+"counters", Counter{
		Name: counter_name,
		Last: 0,
	})

	for !s.stop {

		data, err := s.Inception.Find(s.Prefix+"stream."+name, inceptiondb.FindQuery{
			Index: "number",
			From: JSON{
				"timestamp": s.counters[counter_name],
			},
			To: JSON{
				"timestamp": time.Now().UnixNano(), // Avoid fetch events from the future
			},
			Limit: 1000,
		})
		if err != nil {
			fmt.Println("ERROR:", err)
			time.Sleep(1 * time.Second)
			continue
		}

		d := json.NewDecoder(data)
		for {
			entry := Entry{}
			err := d.Decode(&entry)
			if err == io.EOF {
				break
			}
			if err != nil {
				panic(err) // todo: dont panic!!!!
			}

			if entry.Timestamp == s.counters[counter_name] {
				continue
			}

			callbackErr := callback(entry.Payload)
			if callbackErr == nil {
				s.counters[counter_name] = entry.Timestamp
			}
		}

		time.Sleep(1 * time.Second)
	}

	return nil
}

func (s *Streams) Wait() {
	s.wg.Wait()
	s.Persist()
}

func (s *Streams) Close() {
	s.stop = true
}

func (s *Streams) Persist() error {

	for name, last := range s.counters {
		s.Inception.Patch(s.Prefix+"counters", inceptiondb.PatchQuery{
			Filter: JSON{
				"name": name,
			},
			Patch: JSON{
				"last": last,
			},
		})
	}

	return nil
}
