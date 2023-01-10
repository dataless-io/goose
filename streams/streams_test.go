package streams

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/fulldump/biff"
	"github.com/google/uuid"

	"goose/inceptiondb"
)

func TestHello(t *testing.T) {

	inception := inceptiondb.NewClient(inceptiondb.Config{
		Base: "https://saas.inceptiondb.io/v1",

		// Development credentials by default:
		DatabaseID: "1b5b8fef-db19-4308-852d-d0c61eda7143",
		ApiKey:     "973417a6-ea20-4ac9-8b9f-6f3db9213a01",
		ApiSecret:  "ebac1daf-48fc-4e88-a6ba-04ebf1b48125",
	})

	s := NewStreams(inception)

	ensureErr := s.Ensure("honk_create")
	biff.AssertNil(ensureErr)

	sendErr := s.Send("honk_create", JSON{
		"id":        uuid.New().String(),
		"message":   "Hello world!! " + time.Now().String(),
		"user_id":   "my-user",
		"nick":      "my-nick",
		"timestamp": time.Now().UnixNano(),
	})
	biff.AssertNil(sendErr)

	go func() {
		receiveErr := s.Receive("honk_create", "mentions", func(data []byte) error {

			tweet := JSON{}
			json.Unmarshal(data, &tweet)

			fmt.Println("mentions:", tweet)

			return nil
		})
		biff.AssertNil(receiveErr)
	}()

	cols := []string{
		"streams.counters",
		// "streams.stream.honk_create",
	}
	for _, col := range cols {
		fmt.Println("==== ", col, " =================")
		data, findErr := inception.Find(col, inceptiondb.FindQuery{
			Limit: 100,
		})
		_ = findErr
		// biff.AssertNil(findErr)
		io.Copy(os.Stdout, data)
	}

	s.Close()
	s.Wait()
}
