package main

import(
    "fmt"
    "strconv"
    "sync"
    "net"
    "strings"
    "code.google.com/p/go-uuid/uuid"
)

var wg sync.WaitGroup

func main() {

    users := 1000

    // Increment the WaitGroup counter.
    wg.Add(users)

    for i := 1; i <= users; i++ {
        // Create a connection
    	go startAuth(i)
    }

    // Wait for all HTTP fetches to complete.
    wg.Wait()

    fmt.Println("FINISHED!")
}

func startAuth(i int) {
    //fmt.Println("Starting... #"+strconv.Itoa(i))

	cmdId        := uuid.New()
	connectionId := "TEST_CID_"+strconv.Itoa(i)

	auth := `
{
	"command": "ClientAuth",
	"id": "`+cmdId+`",
	"data": {
		"connectionID": "`+connectionId+`",
		"info" : {
			"appVersion" : "2.5",
			"model" : "LG NEXUS 5", 
			"os" : "Android 5.0", 
			"pushToken": "1234567890987654321",
			"mid" : "1234567890"
		}
	}
}`

	conn, err := net.Dial("tcp", "node1.mag.ndmsystems.com:2424")
    if err != nil {
        fmt.Println("FAILED TO DIAL #"+strconv.Itoa(i))
        fmt.Println(err)
        return
    }

    // send CMD
    conn.Write([]byte(strings.Replace(auth, "\n", "", -1)+"\n"))

    // Decrement the counter when the goroutine completes.
    wg.Done()

    //fmt.Println("Finished #"+strconv.Itoa(i))
}

func generateSessionID() string {
	return uuid.New()
}
